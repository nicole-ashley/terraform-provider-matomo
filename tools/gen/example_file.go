// tools/gen/example_file.go
package main

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

//go:embed templates/example.tf.tmpl
var exampleTemplateFS embed.FS

var exampleTemplate = template.Must(template.ParseFS(exampleTemplateFS, "templates/example.tf.tmpl"))

// exampleOutputDir is a var (not const) for the same test-isolation
// reason outputDir is (see main.go's comment on outputDir).
var exampleOutputDir = "examples/resources"

// writeExampleIfAbsent writes a minimal, realistic example config for
// spec, but only if that file doesn't already exist - so a
// hand-improved example is never clobbered by a later tools/gen run,
// matching writeTestScaffoldIfAbsent's convention.
func writeExampleIfAbsent(spec TypeSpec) error {
	dir := filepath.Join(exampleOutputDir, spec.ResourceName)
	path := filepath.Join(dir, "resource.tf")
	if _, err := os.Stat(path); err == nil {
		return nil // already exists, leave it alone
	} else if !os.IsNotExist(err) {
		return err
	}

	var buf bytes.Buffer
	if err := exampleTemplate.Execute(&buf, newTemplateData(spec)); err != nil {
		return fmt.Errorf("rendering example for %s %q: %w", spec.Kind, spec.TypeID, err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}
