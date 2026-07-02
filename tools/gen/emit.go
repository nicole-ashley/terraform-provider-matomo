// tools/gen/emit.go
package main

import (
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"text/template"
)

// templateData adds the Go-identifier fields the template needs
// (GoModelName, GoSchemaFuncName, etc.) on top of a TypeSpec, without
// polluting TypeSpec itself with rendering-only concerns.
type templateData struct {
	TypeSpec
	GoModelName      string
	GoSchemaFuncName string
	GoTypeName       string
	GoModelReceiver  string
}

func newTemplateData(spec TypeSpec) templateData {
	typeName := ExportedName(spec.Kind) + ExportedName(spec.Slug)
	return templateData{
		TypeSpec:         spec,
		GoModelName:      spec.Kind + ExportedName(spec.Slug) + "Model",
		GoSchemaFuncName: spec.Kind + ExportedName(spec.Slug) + "Schema",
		GoTypeName:       typeName,
		GoModelReceiver:  "m",
	}
}

//go:embed templates/schema.go.tmpl
var schemaTemplateFS embed.FS

var schemaTemplate = template.Must(template.ParseFS(schemaTemplateFS, "templates/schema.go.tmpl"))

// RenderSchema renders spec into a gofmt'd Go source file implementing
// the type's generated model + schema.Schema + typedModel methods, ready
// to write to internal/provider/generated/<kind>_<slug>.go.
func RenderSchema(spec TypeSpec) ([]byte, error) {
	var buf bytes.Buffer
	if err := schemaTemplate.Execute(&buf, newTemplateData(spec)); err != nil {
		return nil, fmt.Errorf("rendering template for %s %q: %w", spec.Kind, spec.TypeID, err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("gofmt-ing generated source for %s %q: %w\n--- unformatted source ---\n%s", spec.Kind, spec.TypeID, err, buf.String())
	}
	return formatted, nil
}
