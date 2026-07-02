# Docs and examples — design spec

## 1. Goal and scope

Phase 8a of the original provider design (`docs/superpowers/specs/2026-06-30-matomo-tagmanager-provider-design.md`
§10, §11, §12). Phase 8 originally bundled docs/examples with the
release pipeline; per user decision this session, it's split into two
independent specs — this one covers docs and examples only. The release
pipeline (goreleaser, signing, Terraform Registry manifest, conventional
commits/`CHANGELOG.md`) is a separate follow-up spec.

Concretely, this phase:

- Adds `tfplugindocs`-generated documentation under `docs/`, committed
  to git and verified in CI.
- Populates `examples/` with one example config per resource, data
  source, and action — a hard requirement for `tfplugindocs` to render
  an example block on each doc page, and per the original spec, doubles
  as an integration fixture.
- Extends `tools/gen` so the 64 generated typed tag/trigger/variable
  resources get their example files automatically, rather than 64
  hand-written near-duplicates.

Out of scope: the release pipeline, any change to resource/data
source/action schemas or behavior, custom `tfplugindocs` templates
(default templates only, per the approved design decision — revisit
only if a concrete cross-linking need comes up later).

## 2. Current state (confirmed by inspection)

- No `examples/` directory exists yet.
- `docs/` exists but only contains `docs/superpowers/` (specs and
  plans) — no `tfplugindocs` output yet.
- No `tfplugindocs` invocation anywhere in `GNUmakefile` or
  `.github/workflows/*.yml`.
- Every resource/data source/action already has `Description` fields
  set on its schema (built incrementally across Phases 1-7) —
  `tfplugindocs` needs no schema changes to produce useful docs.
- 65 files under `internal/provider/generated_*.go` (excluding
  `_test.go`), comprising 31 tag / 17 trigger / 16 variable typed
  resources (64 total; the 65th is `generated_resources.go`, the
  registration list, not a resource itself).
- The non-generated surface needing examples: 6 resources
  (`matomo_site`, `matomo_custom_dimension`, `matomo_tagmanager_container`,
  `matomo_tagmanager_tag`, `matomo_tagmanager_trigger`,
  `matomo_tagmanager_variable`), 6 data sources (`matomo_site` plus the
  5 Phase 6 discovery data sources), 4 actions (the Phase 7 container-
  version/publish/preview actions).

## 3. `examples/` layout

Following `tfplugindocs`' required convention:

```
examples/
  provider/
    provider.tf
  resources/
    matomo_site/resource.tf
    matomo_custom_dimension/resource.tf
    matomo_tagmanager_container/resource.tf
    matomo_tagmanager_tag/resource.tf
    matomo_tagmanager_trigger/resource.tf
    matomo_tagmanager_variable/resource.tf
    matomo_tagmanager_tag_customhtml/resource.tf       # x64, tools/gen-generated
    ...
  data-sources/
    matomo_site/data-source.tf
    matomo_tagmanager_contexts/data-source.tf
    matomo_tagmanager_environments/data-source.tf
    matomo_tagmanager_tag_types/data-source.tf
    matomo_tagmanager_trigger_types/data-source.tf
    matomo_tagmanager_variable_types/data-source.tf
  actions/
    matomo_tagmanager_create_container_version/action.tf
    matomo_tagmanager_publish_container_version/action.tf
    matomo_tagmanager_enable_preview_mode/action.tf
    matomo_tagmanager_disable_preview_mode/action.tf
```

`examples/provider/provider.tf` is a minimal, real block:

```hcl
provider "matomo" {
  base_url  = "https://analytics.example.com"
  api_token = var.matomo_api_token
}
```

## 4. Hand-written examples (16 files)

The 6 resources, 6 data sources, and 4 actions each get one hand-written
`.tf` file. These follow the same shape already proven in each type's
acceptance test (`resource_*_acc_test.go`, `data_source_*_acc_test.go`,
`action_tagmanager_*_acc_test.go`) — a minimal, realistic, valid
configuration, not a copy of the acceptance test's placeholder values
(e.g. use a real-looking site name/URL, not "Acceptance Test Site").
Each file is a plain, standalone `.tf` — no wrapping Go, no `provider {}`
block (tfplugindocs' example rendering assumes the reader already has a
configured provider, matching every other Terraform provider's docs
convention).

## 5. `tools/gen`-generated examples (64 files)

### 5.1 New template

New file `tools/gen/templates/example.tf.tmpl`, adapted from
`tools/gen/templates/acc_test.go.tmpl`'s `testAcc{{.GoTypeName}}Config()`
HCL body (the exact same schema-driven logic: site + container +,
for tags, a `PageView` trigger + `fire_trigger_ids`, plus one line per
required/conditionally-required param using `.Params`/`.GoType`/
`.AvailableValues`), with two differences:

- No Go wrapper (`package provider`, `func ...() string { return `...`` }`)
  — just the raw HCL, since this file is consumed directly as a `.tf`
  file, not embedded in a Go string.
- Naming reworded from "Acceptance Site"/"Acceptance Container" to
  "Example Site"/"Example Container" (this is a person's example, not a
  test fixture) and resource labels from `"test"` to `"example"`.

```hcl
{{/* tools/gen/templates/example.tf.tmpl */}}
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example {{.TypeID}} Site"
  urls = ["https://example-{{.Slug}}.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example {{.TypeID}} Container"
}
{{- if eq .Kind "tag"}}

resource "matomo_tagmanager_trigger" "example" {
  container_id = matomo_tagmanager_container.example.id
  type         = "PageView"
  name         = "Example {{.TypeID}} Trigger"
}
{{- end}}

resource "{{.ResourceName}}" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-{{.Slug}}"
{{- if eq .Kind "tag"}}
  fire_trigger_ids = [matomo_tagmanager_trigger.example.id]
{{- end}}
{{- range .Params}}
{{- if or .Required .ConditionallyRequired}}
{{- if eq .GoType "String"}}
{{- if .AvailableValues}}
  {{.TFName}} = {{printf "%q" (index .AvailableValues 0)}}
{{- else}}
  {{.TFName}} = "example-value"
{{- end}}
{{- else if eq .GoType "Bool"}}
  {{.TFName}} = true
{{- else if eq .GoType "Int64"}}
  {{.TFName}} = 1
{{- else if eq .GoType "Float64"}}
  {{.TFName}} = 1.0
{{- else if eq .GoType "List"}}
  {{.TFName}} = ["example-value"]
{{- end}}
{{- end}}
{{- end}}
}
```

### 5.2 New write step

New file `tools/gen/example_file.go`:

```go
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
// reason outputDir is - see main.go's comment on outputDir.
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
```

(No `format.Source`/gofmt step here, unlike `writeTestScaffoldIfAbsent`
— this output is HCL, not Go; `terraform fmt` could be run on it
separately if formatting matters, but isn't required for `tfplugindocs`
to consume it correctly.)

### 5.3 Wiring into `main.go`

Add one line to `main.go`'s per-spec loop (`tools/gen/main.go:50-57`),
alongside the existing two calls:

```go
for _, spec := range specs {
	if err := writeSchemaFile(spec); err != nil {
		log.Fatalf("writing schema file for %s %q: %v", spec.Kind, spec.TypeID, err)
	}
	if err := writeTestScaffoldIfAbsent(spec); err != nil {
		log.Fatalf("writing test scaffold for %s %q: %v", spec.Kind, spec.TypeID, err)
	}
	if err := writeExampleIfAbsent(spec); err != nil {
		log.Fatalf("writing example for %s %q: %v", spec.Kind, spec.TypeID, err)
	}
}
```

## 6. Docs generation

- Add a `docs` target to `GNUmakefile`:
  ```makefile
  .PHONY: docs
  docs:
  	go tool tfplugindocs generate
  ```
- `go.mod` declares `go 1.25.8` (confirmed), which supports the
  `go get -tool` / `tool` directive mechanism (added in Go 1.24) — run
  `go get -tool github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs`
  once, which adds a `tool` line to `go.mod` and an entry to `go.sum`,
  pinning the version reproducibly without a `tools.go`-style blank
  import file. The Makefile target then invokes it via
  `go tool tfplugindocs generate`. This adds `tfplugindocs` to `go.mod`
  as a build tool only — it is never imported by any package in this
  module, so it doesn't appear in the provider binary's own dependency
  graph (matches how `golangci-lint` is already just a CI-invoked
  binary, not a library this codebase imports).
- Default templates only (per the approved design decision) — no
  `templates/` directory added in this phase.
- Output: `docs/index.md`, `docs/resources/*.md`,
  `docs/data-sources/*.md`, `docs/actions/*.md` — all committed to git.

## 7. CI verification

New step in `.github/workflows/ci.yml` (not `acceptance.yml` —
`tfplugindocs` only needs the compiled provider's schema info via its
own protocol handshake, not a live Matomo instance), added after the
existing build/test/lint steps:

```yaml
- name: Check docs are up to date
  run: |
    make docs
    git diff --exit-code -- docs/ || (echo "::error::docs/ is out of date - run 'make docs' and commit the result" && exit 1)
```

This is the exact same drift-check shape already used for `tools/gen`'s
own output in `acceptance.yml` (`git diff --cached --exit-code`), just
targeted at `docs/` and running in the fast `ci.yml` job instead of the
live-Matomo `acceptance.yml` job, since generating docs needs no
external API calls.

## 8. Testing

No new test framework needed. Correctness is enforced structurally:

- `tools/gen`'s existing unit tests (`tools/gen/*_test.go`) get one new
  test asserting `writeExampleIfAbsent` produces valid HCL (parse with
  `hclsyntax.ParseConfig` or similar — check what HCL-parsing dependency,
  if any, is already available in `go.sum` before adding a new one; if
  none exists, a simpler smoke check — e.g. the file contains the
  expected `resource "..." "example"` block for the spec's
  `ResourceName` — is sufficient and avoids a new dependency).
- CI's docs drift-check (§7) is itself the test that the whole
  `examples/` + `tfplugindocs` pipeline produces stable, correct output
  — if any example is malformed HCL, `tfplugindocs generate` fails
  outright rather than silently producing bad docs.

## 9. Explicitly out of scope

- Custom `tfplugindocs` templates / cross-linking between generic and
  typed resources (per the approved design decision).
- The release pipeline (goreleaser, signing, Terraform Registry
  manifest, `CHANGELOG.md`) — separate spec.
- Regenerating/improving the 64 already-existing
  `generated_*_acc_test.go` files' placeholder values as part of this
  work (unrelated to docs/examples, already handled in earlier phases).
