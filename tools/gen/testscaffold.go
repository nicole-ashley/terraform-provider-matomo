// tools/gen/testscaffold.go
package main

import (
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"text/template"
)

//go:embed templates/acc_test.go.tmpl
var testScaffoldTemplateFS embed.FS

var testScaffoldTemplate = template.Must(template.ParseFS(testScaffoldTemplateFS, "templates/acc_test.go.tmpl"))

// writeTestScaffoldIfAbsent writes a minimal create+read-back acceptance
// test for spec, but only if that file doesn't already exist - so a
// hand-improved test is never clobbered by a later tools/gen run.
func writeTestScaffoldIfAbsent(spec TypeSpec) error {
	path := filepath.Join(outputDir, fmt.Sprintf("generated_%s_%s_acc_test.go", spec.Kind, spec.Slug))
	if _, err := os.Stat(path); err == nil {
		return nil // already exists, leave it alone
	} else if !os.IsNotExist(err) {
		return err
	}

	var buf bytes.Buffer
	if err := testScaffoldTemplate.Execute(&buf, newTemplateData(spec)); err != nil {
		return fmt.Errorf("rendering test scaffold for %s %q: %w", spec.Kind, spec.TypeID, err)
	}
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("gofmt-ing test scaffold for %s %q: %w\n--- unformatted source ---\n%s", spec.Kind, spec.TypeID, err, buf.String())
	}
	return os.WriteFile(path, formatted, 0o644)
}
