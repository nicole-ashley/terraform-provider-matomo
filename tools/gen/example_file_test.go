// tools/gen/example_file_test.go
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// mustParseHCL fails the test if src is not syntactically valid HCL.
func mustParseHCL(t *testing.T, name string, src []byte) {
	t.Helper()
	_, diags := hclsyntax.ParseConfig(src, name, hcl.InitialPos)
	if diags.HasErrors() {
		t.Fatalf("generated example is not valid HCL: %v\n--- source ---\n%s", diags, src)
	}
}

func TestWriteExampleIfAbsent(t *testing.T) {
	dir := t.TempDir()
	origExampleOutputDir := exampleOutputDir
	exampleOutputDir = dir
	defer func() { exampleOutputDir = origExampleOutputDir }()

	spec := testSpecs()[0] // CustomHtml tag, has one required String param
	path := filepath.Join(dir, "matomo_tagmanager_tag_customhtml", "resource.tf")

	if err := writeExampleIfAbsent(spec); err != nil {
		t.Fatalf("writeExampleIfAbsent() error = %v", err)
	}
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	mustParseHCL(t, path, src)
	if !strings.Contains(string(src), `resource "matomo_tagmanager_tag_customhtml" "example"`) {
		t.Errorf("generated example missing expected resource block; got:\n%s", src)
	}
	if !strings.Contains(string(src), `custom_html = "example-value"`) {
		t.Errorf("generated example missing expected required param; got:\n%s", src)
	}

	// Second call must not clobber a hand-edited file.
	handEdited := []byte("# hand-edited, do not overwrite\n")
	if err := os.WriteFile(path, handEdited, 0o644); err != nil {
		t.Fatalf("writing hand-edited placeholder: %v", err)
	}
	if err := writeExampleIfAbsent(spec); err != nil {
		t.Fatalf("writeExampleIfAbsent() (second call) error = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s after second call: %v", path, err)
	}
	if string(got) != string(handEdited) {
		t.Errorf("writeExampleIfAbsent() overwrote an existing file; got:\n%s", got)
	}
}
