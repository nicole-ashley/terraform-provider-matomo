// tools/gen/main_test.go
package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testSpecs returns a couple of hand-built TypeSpec values (one tag, one
// variable) covering both branches of the resources.go.tmpl kind switch
// and both the "has required params" and "trivial" shapes. These are not
// discovered from a live Matomo - live Matomo is unavailable in this
// environment, so this is this task's substitute for the brief's Step 4
// end-to-end verification.
func testSpecs() []TypeSpec {
	return []TypeSpec{
		{
			Kind:         "tag",
			TypeID:       "CustomHtml",
			Slug:         "customhtml",
			ResourceName: "matomo_tagmanager_tag_customhtml",
			Description:  "Inject custom HTML",
			Params: []ParamSpec{
				{MatomoName: "customHtml", TFName: "custom_html", GoFieldName: "CustomHtml", Description: "The HTML to inject", GoType: "String", Required: true},
			},
		},
		{
			Kind:         "variable",
			TypeID:       "Constant",
			Slug:         "constant",
			ResourceName: "matomo_tagmanager_variable_constant",
			Description:  "A constant value",
			Params: []ParamSpec{
				{MatomoName: "value", TFName: "value", GoFieldName: "Value", Description: "The constant value", GoType: "String", Required: true},
			},
		},
	}
}

// mustParseGo fails the test if src is not syntactically valid Go.
func mustParseGo(t *testing.T, name string, src []byte) {
	t.Helper()
	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, name, src, parser.AllErrors); err != nil {
		t.Fatalf("%s does not parse as valid Go: %v\n---\n%s", name, err, src)
	}
}

func TestWriteSchemaFile(t *testing.T) {
	dir := t.TempDir()
	origOutputDir := outputDir
	outputDir = dir
	defer func() { outputDir = origOutputDir }()

	spec := testSpecs()[0]
	if err := writeSchemaFile(spec); err != nil {
		t.Fatalf("writeSchemaFile() error = %v", err)
	}

	path := filepath.Join(dir, "tag_customhtml.go")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	mustParseGo(t, path, src)

	if !strings.Contains(string(src), "tagCustomhtmlModel") {
		t.Errorf("generated schema file missing expected type name; got:\n%s", src)
	}
}

func TestWriteTestScaffoldIfAbsent(t *testing.T) {
	dir := t.TempDir()
	origOutputDir := outputDir
	outputDir = dir
	defer func() { outputDir = origOutputDir }()

	spec := testSpecs()[1]
	path := filepath.Join(dir, "variable_constant_acc_test.go")

	if err := writeTestScaffoldIfAbsent(spec); err != nil {
		t.Fatalf("writeTestScaffoldIfAbsent() error = %v", err)
	}
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	mustParseGo(t, path, src)
	if !strings.Contains(string(src), "TestAccVariableConstant_createAndReadBack") {
		t.Errorf("generated test scaffold missing expected test func name; got:\n%s", src)
	}

	// Second call must not clobber a hand-edited file.
	handEdited := []byte("package provider\n\n// hand-edited, do not overwrite\n")
	if err := os.WriteFile(path, handEdited, 0o644); err != nil {
		t.Fatalf("writing hand-edited placeholder: %v", err)
	}
	if err := writeTestScaffoldIfAbsent(spec); err != nil {
		t.Fatalf("writeTestScaffoldIfAbsent() (second call) error = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s after second call: %v", path, err)
	}
	if string(got) != string(handEdited) {
		t.Errorf("writeTestScaffoldIfAbsent() overwrote an existing file; got:\n%s", got)
	}
}

func TestWriteResourcesFile(t *testing.T) {
	dir := t.TempDir()
	origOutputDir := outputDir
	outputDir = dir
	defer func() { outputDir = origOutputDir }()

	specs := testSpecs()
	if err := writeResourcesFile(specs); err != nil {
		t.Fatalf("writeResourcesFile() error = %v", err)
	}

	path := filepath.Join(dir, "generated_resources.go")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	mustParseGo(t, path, src)

	got := string(src)
	for _, want := range []string{
		"func generatedResources() []func() resource.Resource {",
		"newTypedTagResource(newTagCustomhtmlModel)",
		"newTypedVariableResource(newVariableConstantModel)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("generated_resources.go missing %q; full source:\n%s", want, got)
		}
	}
}

// TestDiscoverAllSpecsSupportingLogicEndToEnd exercises the full
// writeSchemaFile + writeTestScaffoldIfAbsent + writeResourcesFile
// pipeline together against hand-built specs (standing in for what
// discoverAllSpecs would have returned from a live Matomo), all writing
// into a t.TempDir() so nothing under internal/provider/generated is
// touched.
func TestDiscoverAllSpecsSupportingLogicEndToEnd(t *testing.T) {
	dir := t.TempDir()
	origOutputDir := outputDir
	outputDir = dir
	defer func() { outputDir = origOutputDir }()

	specs := testSpecs()
	for _, spec := range specs {
		if err := writeSchemaFile(spec); err != nil {
			t.Fatalf("writeSchemaFile(%s %s): %v", spec.Kind, spec.TypeID, err)
		}
		if err := writeTestScaffoldIfAbsent(spec); err != nil {
			t.Fatalf("writeTestScaffoldIfAbsent(%s %s): %v", spec.Kind, spec.TypeID, err)
		}
	}
	if err := writeResourcesFile(specs); err != nil {
		t.Fatalf("writeResourcesFile: %v", err)
	}

	wantFiles := []string{
		"tag_customhtml.go",
		"tag_customhtml_acc_test.go",
		"variable_constant.go",
		"variable_constant_acc_test.go",
		"generated_resources.go",
	}
	for _, name := range wantFiles {
		path := filepath.Join(dir, name)
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("expected %s to be written: %v", path, err)
		}
		mustParseGo(t, path, src)
	}
}
