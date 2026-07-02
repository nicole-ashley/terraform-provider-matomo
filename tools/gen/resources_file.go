// tools/gen/resources_file.go
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

//go:embed templates/resources.go.tmpl
var resourcesTemplateFS embed.FS

var resourcesTemplate = template.Must(template.ParseFS(resourcesTemplateFS, "templates/resources.go.tmpl"))

func writeResourcesFile(specs []TypeSpec) error {
	data := make([]templateData, len(specs))
	for i, spec := range specs {
		data[i] = newTemplateData(spec)
	}

	var buf bytes.Buffer
	if err := resourcesTemplate.Execute(&buf, data); err != nil {
		return fmt.Errorf("rendering generated_resources.go: %w", err)
	}
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("gofmt-ing generated_resources.go: %w\n--- unformatted source ---\n%s", err, buf.String())
	}
	return os.WriteFile(filepath.Join(outputDir, "generated_resources.go"), formatted, 0o644)
}
