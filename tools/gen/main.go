// tools/gen/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

// outputDir is a var (not const) so tests can point it at a t.TempDir()
// without touching the real tree.
//
// Generated files must live directly in internal/provider, not a
// subdirectory: Go package identity is per-directory, not per
// package-name string, so a "package provider" file in
// internal/provider/generated/ would compile as its own, separate
// package that cannot see internal/provider's unexported typedMeta/
// typedModel/paramXString helpers the generated code needs - confirmed
// the hard way via a live CI build failure ("undefined: typedMeta") the
// first time real generated output was compiled together with the rest
// of the package. Generated filenames are prefixed with "generated_" to
// keep them visually distinct from hand-written files in the same
// directory.
var outputDir = "internal/provider"

func main() {
	baseURL := os.Getenv("MATOMO_BASE_URL")
	apiToken := os.Getenv("MATOMO_API_TOKEN")
	if baseURL == "" || apiToken == "" {
		log.Fatal("tools/gen requires MATOMO_BASE_URL and MATOMO_API_TOKEN to be set (point them at the acceptance-test Matomo fixture)")
	}

	client := matomo.NewClient(baseURL, apiToken, &http.Client{})
	ctx := context.Background()

	specs, err := discoverAllSpecs(ctx, client)
	if err != nil {
		log.Fatalf("discovering Tag Manager types: %v", err)
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		log.Fatalf("creating %s: %v", outputDir, err)
	}

	for _, spec := range specs {
		if err := writeSchemaFile(spec); err != nil {
			log.Fatalf("writing schema file for %s %q: %v", spec.Kind, spec.TypeID, err)
		}
		if err := writeTestScaffoldIfAbsent(spec); err != nil {
			log.Fatalf("writing test scaffold for %s %q: %v", spec.Kind, spec.TypeID, err)
		}
	}

	if err := writeResourcesFile(specs); err != nil {
		log.Fatalf("writing generated_resources.go: %v", err)
	}

	log.Printf("generated %d typed Tag Manager resources into %s", len(specs), outputDir)
}

func discoverAllSpecs(ctx context.Context, client *matomo.Client) ([]TypeSpec, error) {
	var specs []TypeSpec

	tagTemplates, err := client.GetAvailableTagTypes(ctx, "web")
	if err != nil {
		return nil, fmt.Errorf("GetAvailableTagTypes: %w", err)
	}
	for _, tmpl := range tagTemplates {
		spec, err := BuildTypeSpec("tag", tmpl)
		if err != nil {
			return nil, err
		}
		specs = append(specs, spec)
	}

	triggerTemplates, err := client.GetAvailableTriggerTypes(ctx, "web")
	if err != nil {
		return nil, fmt.Errorf("GetAvailableTriggerTypes: %w", err)
	}
	for _, tmpl := range triggerTemplates {
		spec, err := BuildTypeSpec("trigger", tmpl)
		if err != nil {
			return nil, err
		}
		specs = append(specs, spec)
	}

	variableTemplates, err := client.GetAvailableVariableTypes(ctx, "web")
	if err != nil {
		return nil, fmt.Errorf("GetAvailableVariableTypes: %w", err)
	}
	for _, tmpl := range variableTemplates {
		spec, err := BuildTypeSpec("variable", tmpl)
		if err != nil {
			return nil, err
		}
		specs = append(specs, spec)
	}

	return specs, nil
}

func writeSchemaFile(spec TypeSpec) error {
	src, err := RenderSchema(spec)
	if err != nil {
		return err
	}
	path := filepath.Join(outputDir, fmt.Sprintf("generated_%s_%s.go", spec.Kind, spec.Slug))
	return os.WriteFile(path, src, 0o644)
}
