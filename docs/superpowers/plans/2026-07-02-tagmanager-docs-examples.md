# Docs and Examples Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Populate `examples/` for every resource, data source, and action, and generate `tfplugindocs` documentation under `docs/`, both verified in CI via drift-checks.

**Architecture:** `tools/gen` gains a third per-spec output (alongside the existing schema file and acc-test scaffold): a write-once-if-absent example `.tf` file per generated typed resource, reusing the same schema-driven HCL already proven in each type's acceptance test template. The 16 non-generated items (6 resources, 6 data sources, 4 actions, plus the top-level provider example) get hand-written examples. `tfplugindocs` is added as a Go tool dependency (`go get -tool`, no runtime import) and wired into the Makefile and CI as a regenerate-and-diff drift-check, mirroring the existing pattern already used for `tools/gen`'s own output in `acceptance.yml`.

**Tech Stack:** Go `text/template` + `embed.FS` (matching `tools/gen`'s existing template machinery), `github.com/hashicorp/hcl/v2/hclsyntax` (already an indirect dependency via `go.sum`) for validating generated example HCL in tests, `github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs` as a `go.mod` tool dependency.

## Global Constraints

- No custom `tfplugindocs` templates in this phase — default templates only.
- `tools/gen`'s new example-writing step must be write-once-if-absent (never clobbers a hand-edited example), matching `writeTestScaffoldIfAbsent`'s existing convention exactly.
- Generated example output path: `examples/resources/<ResourceName>/resource.tf` (e.g. `examples/resources/matomo_tagmanager_tag_customhtml/resource.tf`).
- Hand-written examples use realistic naming ("Example ... Site", not "Acceptance Test Site") and contain no `provider "matomo" {}` block (tfplugindocs' convention assumes the reader already has a configured provider) — this differs from the generated examples, which DO include `provider "matomo" {}` since they're adapted directly from the acc-test template's config, which needs it to be a runnable acceptance test config; this inconsistency is accepted as-is per the spec (§4 vs §5.1), not a bug to fix.
- `go.mod` already declares `go 1.25.8`, which supports `go get -tool` (added in Go 1.24) — use it to add `tfplugindocs`, not a `tools.go`-style blank import file.
- CI's docs drift-check step lives in `ci.yml` (fast job, no live Matomo needed), not `acceptance.yml` — `tfplugindocs` only needs the compiled provider's schema (via a real `terraform` binary, already provisioned in `ci.yml`'s `test` job via `hashicorp/setup-terraform@v3`), not a live Matomo instance.
- This sandbox has neither Docker (for live Matomo) nor a `terraform` CLI binary installed — matches this project's established, repeatedly-hit limitation. The 64 generated example files and the real `docs/` content cannot be produced locally in this environment; both require a temporary CI bootstrap-commit step (the same mechanism already used and reverted multiple times earlier in this project for `tools/gen`'s own output) to generate-and-commit for real, then revert back to a pure drift-check once real content exists.
- Full spec: `docs/superpowers/specs/2026-07-02-tagmanager-docs-examples-design.md`.

---

## File Structure

- Create: `tools/gen/templates/example.tf.tmpl` — the new example template.
- Create: `tools/gen/example_file.go` — `writeExampleIfAbsent`.
- Create: `tools/gen/example_file_test.go` — its unit test.
- Modify: `tools/gen/main.go` — wire `writeExampleIfAbsent` into the per-spec loop.
- Create: `examples/provider/provider.tf`.
- Create: `examples/resources/matomo_site/resource.tf`, `examples/resources/matomo_custom_dimension/resource.tf`, `examples/resources/matomo_tagmanager_container/resource.tf`, `examples/resources/matomo_tagmanager_tag/resource.tf`, `examples/resources/matomo_tagmanager_trigger/resource.tf`, `examples/resources/matomo_tagmanager_variable/resource.tf`.
- Create: `examples/data-sources/matomo_site/data-source.tf`, `examples/data-sources/matomo_tagmanager_contexts/data-source.tf`, `examples/data-sources/matomo_tagmanager_environments/data-source.tf`, `examples/data-sources/matomo_tagmanager_tag_types/data-source.tf`, `examples/data-sources/matomo_tagmanager_trigger_types/data-source.tf`, `examples/data-sources/matomo_tagmanager_variable_types/data-source.tf`.
- Create: `examples/actions/matomo_tagmanager_create_container_version/action.tf`, `examples/actions/matomo_tagmanager_publish_container_version/action.tf`, `examples/actions/matomo_tagmanager_enable_preview_mode/action.tf`, `examples/actions/matomo_tagmanager_disable_preview_mode/action.tf`.
- Modify: `go.mod`, `go.sum` — add `tfplugindocs` as a tool dependency.
- Modify: `GNUmakefile` — add a `docs` target.
- Modify: `.github/workflows/ci.yml` — add the docs drift-check step.
- Modify: `.github/workflows/acceptance.yml` — temporarily toggled to also stage/diff `examples/resources/*` in its existing drift-check (Task 4 only, reverted at the end of Task 4).

---

### Task 1: `tools/gen` example template and writer

**Files:**
- Create: `tools/gen/templates/example.tf.tmpl`
- Create: `tools/gen/example_file.go`
- Create: `tools/gen/example_file_test.go`
- Modify: `tools/gen/main.go:50-57`

**Interfaces:**
- Consumes: `TypeSpec` (`tools/gen/spec.go:30-40`, fields `Kind`, `TypeID`, `Slug`, `ResourceName`, `Params []ParamSpec`), `ParamSpec` (`tools/gen/spec.go:12-26`, fields `TFName`, `GoType`, `Required`, `AvailableValues`, `ConditionallyRequired`), `newTemplateData(spec TypeSpec) templateData` (`tools/gen/emit.go:76`, embeds `TypeSpec` plus `GoTypeName` — the template only needs `.TypeID`/`.Slug`/`.Kind`/`.ResourceName`/`.Params`, all on the embedded `TypeSpec`).
- Produces: `func writeExampleIfAbsent(spec TypeSpec) error` — Task 4's live CI run calls this transitively via `tools/gen`'s `main()`, once wired in.

- [ ] **Step 1: Write the template**

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

- [ ] **Step 2: Write the failing test**

```go
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
```

`testSpecs()` already exists in `tools/gen/main_test.go` (same package `main`, no import needed) and returns `[]TypeSpec{CustomHtml tag, Constant variable}` — index `[0]` is the `CustomHtml` tag spec with one required `String` param (`custom_html`). Confirmed via `go doc`: `hcl.InitialPos` is `var InitialPos = Pos{Byte: 0, Line: 1, Column: 1}` and `hclsyntax.ParseConfig(src []byte, filename string, start hcl.Pos) (*hcl.File, hcl.Diagnostics)` — both match the usage above exactly.

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./tools/gen/... -run TestWriteExampleIfAbsent -v`
Expected: FAIL — `undefined: writeExampleIfAbsent` / `undefined: exampleOutputDir` (neither exists yet).

- [ ] **Step 4: Write the implementation**

```go
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
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./tools/gen/... -run TestWriteExampleIfAbsent -v`
Expected: PASS.

- [ ] **Step 6: Wire into `main.go`'s per-spec loop**

Edit `tools/gen/main.go:50-57`:

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

- [ ] **Step 7: Full package check and commit**

`example_file_test.go` newly imports `github.com/hashicorp/hcl/v2` directly, which `go.mod` currently lists as `// indirect` — run `go mod tidy` first so `go.mod`/`go.sum` correctly reflect it as a direct dependency (a plain `go build` alone will not update the `// indirect` comment, and this sandbox's Go toolchain has previously been observed to silently auto-patch `go.mod` on a bare `go build` in a way that can mask a real missing-dependency bug — verify with `GOFLAGS=-mod=readonly` after `go mod tidy`, not before).

Run: `go mod tidy`
Expected: `go.mod`'s `github.com/hashicorp/hcl/v2` line loses its `// indirect` suffix; `go.sum` is unchanged or gains entries (no existing entries removed for packages still used elsewhere).

Run: `GOFLAGS=-mod=readonly go build ./... && GOFLAGS=-mod=readonly go vet ./...`
Expected: both clean. If this fails with a missing-dependency error, `go mod tidy` didn't fully resolve it — do not proceed until this passes under `-mod=readonly`, since that's what CI effectively enforces.

Run: `gofmt -l tools/gen/example_file.go tools/gen/example_file_test.go tools/gen/main.go tools/gen/templates/example.tf.tmpl`
Expected: prints nothing for the three `.go` files (the `.tmpl` file isn't Go source, `gofmt -l` will just ignore or no-op on it — don't worry if it's not a recognized extension).

Run: `go test ./tools/gen/... -v -count=1`
Expected: all tests PASS, including the new one and every pre-existing `tools/gen` test.

```bash
git add tools/gen/templates/example.tf.tmpl tools/gen/example_file.go tools/gen/example_file_test.go tools/gen/main.go go.mod go.sum
git commit -m "Add tools/gen example-file generation for typed resources"
```

---

### Task 2: Hand-written examples for non-generated resources, data sources, and actions

**Files:**
- Create: `examples/provider/provider.tf`
- Create: `examples/resources/matomo_site/resource.tf`
- Create: `examples/resources/matomo_custom_dimension/resource.tf`
- Create: `examples/resources/matomo_tagmanager_container/resource.tf`
- Create: `examples/resources/matomo_tagmanager_tag/resource.tf`
- Create: `examples/resources/matomo_tagmanager_trigger/resource.tf`
- Create: `examples/resources/matomo_tagmanager_variable/resource.tf`
- Create: `examples/data-sources/matomo_site/data-source.tf`
- Create: `examples/data-sources/matomo_tagmanager_contexts/data-source.tf`
- Create: `examples/data-sources/matomo_tagmanager_environments/data-source.tf`
- Create: `examples/data-sources/matomo_tagmanager_tag_types/data-source.tf`
- Create: `examples/data-sources/matomo_tagmanager_trigger_types/data-source.tf`
- Create: `examples/data-sources/matomo_tagmanager_variable_types/data-source.tf`
- Create: `examples/actions/matomo_tagmanager_create_container_version/action.tf`
- Create: `examples/actions/matomo_tagmanager_publish_container_version/action.tf`
- Create: `examples/actions/matomo_tagmanager_enable_preview_mode/action.tf`
- Create: `examples/actions/matomo_tagmanager_disable_preview_mode/action.tf`

**Interfaces:**
- Consumes: nothing from earlier tasks (pure content, no code).
- Produces: nothing later tasks import — Task 4's `tfplugindocs generate` run consumes these files by convention (path-based discovery), not a Go interface.

- [ ] **Step 1: Provider example**

```hcl
# examples/provider/provider.tf
provider "matomo" {
  base_url  = "https://analytics.example.com"
  api_token = var.matomo_api_token
}
```

- [ ] **Step 2: `matomo_site` resource example**

```hcl
# examples/resources/matomo_site/resource.tf
resource "matomo_site" "main" {
  name     = "My Website"
  urls     = ["https://www.example.com"]
  timezone = "America/New_York"
  currency = "USD"
}
```

- [ ] **Step 3: `matomo_custom_dimension` resource example**

```hcl
# examples/resources/matomo_custom_dimension/resource.tf
resource "matomo_custom_dimension" "user_type" {
  site_id = matomo_site.main.id
  name    = "User Type"
  scope   = "visit"
  active  = true
}
```

- [ ] **Step 4: `matomo_tagmanager_container` resource example**

```hcl
# examples/resources/matomo_tagmanager_container/resource.tf
resource "matomo_tagmanager_container" "main" {
  site_id     = matomo_site.main.id
  context     = "web"
  name        = "Main Website Container"
  description = "Primary Tag Manager container for the main website"
}
```

- [ ] **Step 5: `matomo_tagmanager_tag` (generic fallback) resource example**

```hcl
# examples/resources/matomo_tagmanager_tag/resource.tf
resource "matomo_tagmanager_trigger" "all_pages" {
  container_id = matomo_tagmanager_container.main.id
  type         = "PageView"
  name         = "All Page Views"
}

resource "matomo_tagmanager_tag" "custom_html" {
  container_id     = matomo_tagmanager_container.main.id
  type             = "CustomHtml"
  name             = "Custom HTML Tag"
  fire_trigger_ids = [matomo_tagmanager_trigger.all_pages.id]

  parameter {
    name  = "customHtml"
    value = "<script>console.log('loaded');</script>"
  }
}
```

- [ ] **Step 6: `matomo_tagmanager_trigger` (generic fallback) resource example**

```hcl
# examples/resources/matomo_tagmanager_trigger/resource.tf
resource "matomo_tagmanager_trigger" "custom" {
  container_id = matomo_tagmanager_container.main.id
  type         = "PageView"
  name         = "Custom Page View Trigger"
}
```

- [ ] **Step 7: `matomo_tagmanager_variable` (generic fallback) resource example**

```hcl
# examples/resources/matomo_tagmanager_variable/resource.tf
resource "matomo_tagmanager_variable" "environment" {
  container_id = matomo_tagmanager_container.main.id
  type         = "Constant"
  name         = "Environment Name"

  parameter {
    name  = "constantValue"
    value = "production"
  }
}
```

- [ ] **Step 8: `matomo_site` data source example**

```hcl
# examples/data-sources/matomo_site/data-source.tf
data "matomo_site" "existing" {
  name = "My Existing Website"
}
```

- [ ] **Step 9: `matomo_tagmanager_contexts` data source example**

```hcl
# examples/data-sources/matomo_tagmanager_contexts/data-source.tf
data "matomo_tagmanager_contexts" "all" {}

output "available_contexts" {
  value = [for c in data.matomo_tagmanager_contexts.all.contexts : c.id]
}
```

- [ ] **Step 10: `matomo_tagmanager_environments` data source example**

```hcl
# examples/data-sources/matomo_tagmanager_environments/data-source.tf
data "matomo_tagmanager_environments" "all" {}

output "available_environments" {
  value = [for e in data.matomo_tagmanager_environments.all.environments : e.id]
}
```

- [ ] **Step 11: `matomo_tagmanager_tag_types` data source example**

```hcl
# examples/data-sources/matomo_tagmanager_tag_types/data-source.tf
data "matomo_tagmanager_tag_types" "web" {
  context = "web"
}

output "available_tag_types" {
  value = [for t in data.matomo_tagmanager_tag_types.web.tag_types : t.id]
}
```

- [ ] **Step 12: `matomo_tagmanager_trigger_types` data source example**

```hcl
# examples/data-sources/matomo_tagmanager_trigger_types/data-source.tf
data "matomo_tagmanager_trigger_types" "web" {
  context = "web"
}

output "available_trigger_types" {
  value = [for t in data.matomo_tagmanager_trigger_types.web.trigger_types : t.id]
}
```

- [ ] **Step 13: `matomo_tagmanager_variable_types` data source example**

```hcl
# examples/data-sources/matomo_tagmanager_variable_types/data-source.tf
data "matomo_tagmanager_variable_types" "web" {
  context = "web"
}

output "available_variable_types" {
  value = [for t in data.matomo_tagmanager_variable_types.web.variable_types : t.id]
}
```

- [ ] **Step 14: `matomo_tagmanager_create_container_version` action example**

```hcl
# examples/actions/matomo_tagmanager_create_container_version/action.tf
action "matomo_tagmanager_create_container_version" "checkpoint" {
  config {
    container_id = matomo_tagmanager_container.main.id
    name         = "pre-release-checkpoint"
    description  = "Snapshot before publishing new changes"
  }
}
```

- [ ] **Step 15: `matomo_tagmanager_publish_container_version` action example**

```hcl
# examples/actions/matomo_tagmanager_publish_container_version/action.tf
action "matomo_tagmanager_publish_container_version" "go_live" {
  config {
    container_id = matomo_tagmanager_container.main.id
    environment  = "live"
  }
}
```

- [ ] **Step 16: `matomo_tagmanager_enable_preview_mode` action example**

```hcl
# examples/actions/matomo_tagmanager_enable_preview_mode/action.tf
action "matomo_tagmanager_enable_preview_mode" "preview" {
  config {
    container_id = matomo_tagmanager_container.main.id
  }
}
```

- [ ] **Step 17: `matomo_tagmanager_disable_preview_mode` action example**

```hcl
# examples/actions/matomo_tagmanager_disable_preview_mode/action.tf
action "matomo_tagmanager_disable_preview_mode" "preview" {
  config {
    container_id = matomo_tagmanager_container.main.id
  }
}
```

- [ ] **Step 18: Commit**

```bash
git add examples/
git commit -m "Add hand-written examples for non-generated resources, data sources, and actions"
```

---

### Task 3: `tfplugindocs` tool dependency and Makefile target

**Files:**
- Modify: `go.mod`, `go.sum`
- Modify: `GNUmakefile`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `go tool tfplugindocs` (invokable from anywhere in the module), `make docs` — Task 4's CI step calls `make docs`.

- [ ] **Step 1: Add the tool dependency**

Run: `go get -tool github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@latest`
Expected: `go.mod` gains a `tool` directive (e.g. `tool github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs`), `go.sum` gains new entries. No changes to the module's `require` block for the main provider's own runtime dependencies.

- [ ] **Step 2: Verify the tool resolves**

Run: `go tool tfplugindocs --help`
Expected: prints `tfplugindocs`'s usage/help text (confirms the tool is resolvable via `go tool`, not that generation itself works yet — that needs a `terraform` binary, checked in Task 4).

- [ ] **Step 3: Add the Makefile target**

Edit `GNUmakefile`, adding after the existing `lint` target:

```makefile
.PHONY: docs
docs:
	go tool tfplugindocs generate
```

- [ ] **Step 4: Verify build still passes**

Run: `go build ./... && go vet ./...`
Expected: clean (the tool dependency doesn't affect the buildable module's own code).

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum GNUmakefile
git commit -m "Add tfplugindocs as a go tool dependency"
```

---

### Task 4: Generate real `examples/` (64 files) and `docs/` content via CI, then lock in drift-checks

This task is different from the others: it produces real content this sandbox cannot generate locally (no Docker/live Matomo, no `terraform` CLI), by temporarily toggling CI to commit its own generated output — the exact same bootstrap-commit mechanism already used and reverted multiple times earlier in this project (see `docs/superpowers/plans/2026-07-02-tagmanager-typed-resource-codegen.md`'s Task 11/12 and this project's git history for `.github/workflows/acceptance.yml`).

**Files:**
- Modify: `.github/workflows/acceptance.yml` (temporarily, reverted at the end of this task)
- Modify: `.github/workflows/ci.yml` (permanently — adds the docs drift-check)
- Create: `examples/resources/<64 generated resource dirs>/resource.tf` (produced by CI, not hand-written)
- Create: `docs/index.md`, `docs/resources/*.md`, `docs/data-sources/*.md`, `docs/actions/*.md` (produced by CI, not hand-written)

**Interfaces:**
- Consumes: `writeExampleIfAbsent` (Task 1, runs automatically as part of `go run ./tools/gen`, which `acceptance.yml` already invokes), `make docs` (Task 3).
- Produces: committed `examples/resources/*` (64 dirs) and `docs/` content — nothing later tasks call as code, this is committed data.

- [ ] **Step 1: Temporarily add a bootstrap-commit step to `acceptance.yml` for the new example files**

The existing drift-check in `.github/workflows/acceptance.yml` only stages `internal/provider/generated_*.go`. Temporarily broaden it to also generate-and-commit the new `examples/resources/*` files it doesn't yet have committed, by replacing the existing "Fail if generated resources have drifted" step with:

```yaml
      - name: Commit generated resources and examples (temporary bootstrap)
        run: |
          git config user.email "noreply@anthropic.com"
          git config user.name "Claude"
          git add -A -- 'internal/provider/generated_*.go' 'examples/resources/*'
          git diff --cached --quiet || git commit -m "Regenerate generated resources and examples against live Matomo"
      - uses: ad-m/github-push-action@master
        with:
          github_token: ${{ secrets.GITHUB_TOKEN }}
          branch: ${{ github.head_ref || github.ref_name }}
```

This grants the workflow write access temporarily — also add `permissions: contents: write` at the top of the `acceptance` job in this same edit.

- [ ] **Step 2: Push this branch and let CI run**

Run: `git add .github/workflows/acceptance.yml && git commit -m "Temporarily enable bootstrap-commit for example generation" && git push`

Wait for the `acceptance` workflow to complete. It will run `go run ./tools/gen` against live Matomo (via the docker-compose fixture already set up in that job), which now also calls `writeExampleIfAbsent` for all 64 generated types (Task 1's wiring), producing and committing the 64 `examples/resources/<type>/resource.tf` files.

If you cannot trigger/observe CI directly from this environment, hand off this step and step 3-4 to whoever can (per this project's established pattern for live-Matomo-dependent work — see prior plans' "Live-Matomo Step X impossible in this environment" notes) and resume at step 5 once the bootstrap-commit has landed and you've pulled the result.

- [ ] **Step 3: Pull the bootstrap-committed examples**

Run: `git pull`
Expected: 64 new `examples/resources/*/resource.tf` files appear, committed by the CI bot.

- [ ] **Step 4: Revert the temporary bootstrap-commit step back to a pure drift-check**

Edit `.github/workflows/acceptance.yml`, replacing the bootstrap-commit step (and the `permissions: contents: write` line) back to:

```yaml
      - name: Fail if generated resources or examples have drifted
        run: |
          git add -A -- 'internal/provider/generated_*.go' 'examples/resources/*'
          git diff --cached --exit-code -- 'internal/provider/generated_*.go' 'examples/resources/*'
```

Remove the `ad-m/github-push-action` step and the `permissions: contents: write` line entirely — back to the original read-only drift-check shape, just with a broadened glob.

```bash
git add .github/workflows/acceptance.yml
git commit -m "Revert to pure drift-check now examples are committed for real"
git push
```

- [ ] **Step 5: Add the docs drift-check to `ci.yml`**

Edit `.github/workflows/ci.yml`'s `test` job, adding a step after the existing `go test ./... -v -count=1` line:

```yaml
      - run: go build -o /dev/null .
      - run: go test ./... -v -count=1
      - name: Check docs are up to date
        run: |
          make docs
          git diff --exit-code -- docs/ || (echo "::error::docs/ is out of date - run 'make docs' and commit the result" && exit 1)
```

`ci.yml`'s `test` job already provisions both Go and a real `terraform` binary (via `hashicorp/setup-terraform@v3`), so `tfplugindocs generate` can run here without any live Matomo dependency.

- [ ] **Step 6: Commit, push, and let CI generate the real `docs/` content**

```bash
git add .github/workflows/ci.yml
git commit -m "Add docs drift-check to CI"
git push
```

The first run of this new step will fail (`docs/` doesn't exist yet, so `git diff --exit-code -- docs/` reports a diff) — this is expected. Run `make docs` in an environment with a `terraform` binary (CI has one; this sandbox does not, per this task's own note above), commit the result, and push:

```bash
make docs
git add docs/
git commit -m "Generate initial tfplugindocs output"
git push
```

If you cannot run `make docs` directly (no `terraform` binary in this sandbox), hand this step off the same way as step 2 — once `docs/` is committed, CI's drift-check step will pass on the next run since there's nothing left to regenerate a diff against.

- [ ] **Step 7: Verify CI is green**

Confirm both `ci.yml` (build/test/lint/docs-check) and `acceptance.yml` (acceptance tests + the broadened drift-check) pass on the latest commit.

---

### Task 5: Full verification pass

**Files:** None created or modified — this task only runs the project's full verification suite across everything Tasks 1-4 added.

**Interfaces:**
- Consumes: everything from Tasks 1-4.
- Produces: nothing new — this is a checkpoint, not a deliverable.

- [ ] **Step 1: Run the full unit test suite**

Run: `go test ./... -v -count=1`
Expected: PASS across all packages, including `TestWriteExampleIfAbsent` and every pre-existing test.

- [ ] **Step 2: Run `go vet` and `gofmt` across the whole repo**

Run: `go vet ./... && gofmt -l .`
Expected: both produce no output.

- [ ] **Step 3: Run the linter**

Run: `golangci-lint run ./...`
Expected: no findings.

- [ ] **Step 4: Confirm the example count**

Run: `find examples/resources -mindepth 1 -maxdepth 1 -type d | wc -l`
Expected: `70` (6 hand-written + 64 generated).

Run: `find examples/data-sources -mindepth 1 -maxdepth 1 -type d | wc -l`
Expected: `6`.

Run: `find examples/actions -mindepth 1 -maxdepth 1 -type d | wc -l`
Expected: `4`.

- [ ] **Step 5: Confirm `docs/` exists and is non-trivial**

Run: `find docs -maxdepth 2 -iname "*.md" | grep -v superpowers | wc -l`
Expected: a number greater than 1 (at minimum `docs/index.md` plus per-resource/data-source/action pages) — if this is `0`, Task 4's docs generation didn't complete; resolve before proceeding.

- [ ] **Step 6: Commit if any step above required fixes (otherwise nothing to commit)**

```bash
git add -A
git commit -m "Fix lint findings in docs/examples work"
```
