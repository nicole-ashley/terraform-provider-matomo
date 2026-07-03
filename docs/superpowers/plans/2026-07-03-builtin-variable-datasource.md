# Built-in Trigger-Condition Variable Data Source Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a data source exposing Matomo's 66 built-in "pre-configured" trigger-condition variables (`PagePath`, `PageHostname`, `ClickId`, etc.) as named, referenceable attributes, so users get real editor autocomplete instead of having to memorize exact PascalCase identifier strings.

**Architecture:** A single new data source, `matomo_tagmanager_builtin_variable`, with no configuration/query arguments and a fixed schema of ~66 `Computed` string attributes (one per built-in variable, snake_case name -> the real Matomo identifier as its value) - entirely static, no live Matomo API call in its `Read()`. This is purely a discoverability layer; it adds no validation and rejects nothing (see the design spec's section 4 - a plugin can contribute variables beyond this list, and user-defined `{{Name}}` references are inherently open-ended, so this data source must never be treated as, or imply, a completeness gate).

**Tech Stack:** Go, terraform-plugin-framework (`datasource.DataSource`), no new client/API dependency (this data source never calls Matomo).

## Global Constraints

- This data source's values are static, hardcoded Go string constants - not
  fetched from any Matomo API call. `Read()` must not call `d.client` at all
  for this reason. Do not add error handling for a network/API failure that
  cannot happen for this specific resource.
- This data source's values must exactly match Matomo's real, currently-live
  built-in variable Type IDs (verified below) - a wrong string here would
  silently produce a condition that never matches at runtime with no error,
  exactly the failure mode this whole design exists to avoid elsewhere. Do
  not guess a name that hasn't been confirmed by Task 1.
- No validation is added anywhere as a side effect of this work (per the
  design spec's corrected section 4) - this task only adds new,
  purely-additive schema. Do not touch `condition.variable`'s validators.
- This environment cannot run live acceptance tests requiring a real Matomo
  instance (no Docker daemon) or `make docs` (network-blocked). Defer
  `TestAcc*`/`TF_ACC=1` runs and the docs drift-check to CI, matching this
  project's established convention.

---

### Task 1: Confirm the built-in variable catalog against live Matomo

**Files:**
- Create: `internal/provider/data_source_tagmanager_builtin_variable_investigation_acc_test.go`
  (temporary - deleted at the end of this task, its only purpose is
  confirming the exact identifier list before Task 2 hand-codes it)

**Interfaces:**
- Consumes: `(*matomo.Client).GetAvailableVariableTypes(ctx context.Context, idContext string) ([]matomo.Template, error)`
  (`internal/matomo/tagmanager_templates.go:74`, already exists, already used
  by `data_source_tagmanager_variable_types.go`).
- Produces: a confirmed, final list of built-in variable Type IDs, recorded
  in this task's report - Task 2's schema/model is written using exactly
  that confirmed list (not the candidate list below, if investigation finds
  any discrepancy).

**Candidate list** (derived from `Template/Variable/PreConfigured/*.php`'s
directory listing in `matomo-org/tag-manager`'s `5.x-dev` branch - filename
with `Variable.php`/`.php` suffix stripped; `BaseDataLayerVariable` and
`BasePreConfiguredVariable` already excluded as confirmed-abstract, non-selectable
base classes):

```
BrowserLanguage, ClickButton, ClickClasses, ClickDestinationUrl, ClickElement,
ClickId, ClickNodeName, ClickText, ContainerId, ContainerRevision,
ContainerVersion, DnsLookupTime, Environment, ErrorLine, ErrorMessage,
ErrorUrl, FirstDirectory, FormClasses, FormDestination, FormElement, FormId,
FormName, HistoryHashNewPath, HistoryHashNewSearch, HistoryHashNewUrl,
HistoryHashNew, HistoryHashOldPath, HistoryHashOldSearch, HistoryHashOldUrl,
HistoryHashOld, HistorySource, IsoDate, LocalDate, LocalHour, LocalTime,
PageHash, PageHostname, PageLoadTimeTotal, PageOrigin, PagePath,
PageRenderTime, PageTitle, PageUrl, PreviewMode, RandomNumber, Referrer,
ScreenHeight, ScreenHeightAvailable, ScreenWidth, ScreenWidthAvailable,
ScrollHorizontalPercentage, ScrollLeftPixel, ScrollSource, ScrollTopPixel,
ScrollVerticalPercentage, SeoCanonicalUrl, SeoNumH1, SeoNumH2, UserAgent,
UtcDate, VisibleElementClasses, VisibleElementId, VisibleElementNodeName,
VisibleElementText, VisibleElementUrl, Weekday
```

(66 entries. Two of these - `PagePath` and `PageHostname` - are already
independently confirmed correct, since they're exercised live in
`resource_tagmanager_trigger_acc_test.go`'s existing `TestAccTagManagerTriggerResource_withConditions`
test.)

- [ ] **Step 1: Write a throwaway diagnostic acceptance test**

Create `internal/provider/data_source_tagmanager_builtin_variable_investigation_acc_test.go`:

```go
package provider

import (
	"context"
	"testing"
)

// Throwaway investigation, not a real test - deleted once its output
// confirms or corrects the candidate built-in variable list this plan's
// Task 2 depends on. See docs/superpowers/plans/2026-07-03-builtin-variable-datasource.md.
func TestAccInvestigateBuiltinVariables(t *testing.T) {
	testAccPreCheck(t)
	client := testAccMatomoClient(t)

	templates, err := client.GetAvailableVariableTypes(context.Background(), "web")
	if err != nil {
		t.Fatalf("GetAvailableVariableTypes: %v", err)
	}
	t.Logf("got %d variable types from live Matomo:", len(templates))
	for _, tmpl := range templates {
		t.Logf("  id=%q name=%q category=%q", tmpl.ID, tmpl.Name, tmpl.Category)
	}
}
```

(`testAccMatomoClient(t)` already exists as a test helper - used the same
way by the release-actions acceptance tests for out-of-band verification.)

- [ ] **Step 2: Run it and capture the output**

This requires a live Matomo instance - run via this repo's normal acceptance
workflow (`TF_ACC=1 go test ./internal/provider/... -run TestAccInvestigateBuiltinVariables -v`
against the docker-compose fixture, or via CI if run locally is unavailable).
Expected: a log line per variable type Matomo's discovery API returns for the
"web" context.

- [ ] **Step 3: Compare the output against the candidate list above**

Two possible outcomes:

- **The 66 candidates (or some subset/superset) appear in the discovery
  output.** Then Matomo's discovery API *does* expose these, and the final
  list for Task 2 is whatever the live output actually shows (correct any
  candidate names Step 2's output disagrees with, and note any entries this
  candidate list missed).
- **None of the 66 appear in the discovery output at all** (this is the
  likely outcome, since these live in a separate `PreConfigured/` subdirectory
  from the `Template/Variable/*.php` types this same discovery call already
  successfully surfaces for the *user-creatable* variable types this
  provider generates typed resources for - `Template/Variable/PreConfigured/`
  was never part of that registry). If so, the candidate list above is the
  final list Task 2 uses as-is - it was derived correctly from source, just
  not independently confirmable through this particular API call. Record
  this finding in the task report either way.

- [ ] **Step 4: Delete the throwaway test**

```bash
rm internal/provider/data_source_tagmanager_builtin_variable_investigation_acc_test.go
```

- [ ] **Step 5: Commit**

Only if Step 1's file was ever committed accidentally - normally there's
nothing to commit for this task, since the throwaway test is created and
deleted within it. If you did commit it, remove it in a follow-up commit:

```bash
git add internal/provider/data_source_tagmanager_builtin_variable_investigation_acc_test.go
git commit -m "Remove throwaway built-in variable investigation test"
```

---

### Task 2: Create the `matomo_tagmanager_builtin_variable` data source

**Files:**
- Create: `internal/provider/data_source_tagmanager_builtin_variable.go`
- Create: `internal/provider/data_source_tagmanager_builtin_variable_acc_test.go`
- Modify: `internal/provider/provider.go:123-132` (register the new data source)

**Interfaces:**
- Consumes: the confirmed final variable-name list from Task 1's report.
- Produces: `data.matomo_tagmanager_builtin_variable.<label>.<snake_case_name>`
  attributes usable anywhere a `condition.variable` string is accepted
  (Plan 1's `condition.variable`, or the equivalent typed-trigger attribute) -
  nothing in this codebase consumes this programmatically; it's a pure
  documentation/ergonomics leaf.

- [ ] **Step 1: Create the data source**

Create `internal/provider/data_source_tagmanager_builtin_variable.go`. The
snake_case attribute names below assume Task 1 confirmed the candidate list
unchanged - substitute Task 1's corrected list if it differs. `ID`/`Category`
naming follow this file's own compound-word splitting (`IsoDate` ->
`iso_date`, `SeoNumH1` -> `seo_num_h1`, `HistoryHashNewPath` ->
`history_hash_new_path`, etc. - ordinary snake_case of the PascalCase Type
ID):

```go
package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &tagManagerBuiltinVariableDataSource{}

func NewTagManagerBuiltinVariableDataSource() datasource.DataSource {
	return &tagManagerBuiltinVariableDataSource{}
}

// tagManagerBuiltinVariableDataSource has no configuration and makes no API
// call - every attribute value is a fixed constant, confirmed against
// Matomo's own Template/Variable/PreConfigured/*.php source (see this
// resource's implementation plan for how each was confirmed). This exists
// purely so these identifiers are referenceable (and therefore
// editor-autocompletable) instead of typed as bare strings - it is not
// validation and does not represent every variable name Matomo or a
// third-party plugin might accept.
type tagManagerBuiltinVariableDataSource struct{}

// builtinVariableIDs maps each Terraform-facing snake_case attribute name to
// its real Matomo Type ID, confirmed per this resource's implementation
// plan Task 1.
var builtinVariableIDs = map[string]string{
	"browser_language":              "BrowserLanguage",
	"click_button":                  "ClickButton",
	"click_classes":                 "ClickClasses",
	"click_destination_url":         "ClickDestinationUrl",
	"click_element":                 "ClickElement",
	"click_id":                      "ClickId",
	"click_node_name":               "ClickNodeName",
	"click_text":                    "ClickText",
	"container_id":                  "ContainerId",
	"container_revision":            "ContainerRevision",
	"container_version":             "ContainerVersion",
	"dns_lookup_time":               "DnsLookupTime",
	"environment":                   "Environment",
	"error_line":                    "ErrorLine",
	"error_message":                 "ErrorMessage",
	"error_url":                     "ErrorUrl",
	"first_directory":               "FirstDirectory",
	"form_classes":                  "FormClasses",
	"form_destination":              "FormDestination",
	"form_element":                  "FormElement",
	"form_id":                       "FormId",
	"form_name":                     "FormName",
	"history_hash_new_path":         "HistoryHashNewPath",
	"history_hash_new_search":       "HistoryHashNewSearch",
	"history_hash_new_url":          "HistoryHashNewUrl",
	"history_hash_new":              "HistoryHashNew",
	"history_hash_old_path":         "HistoryHashOldPath",
	"history_hash_old_search":       "HistoryHashOldSearch",
	"history_hash_old_url":          "HistoryHashOldUrl",
	"history_hash_old":              "HistoryHashOld",
	"history_source":                "HistorySource",
	"iso_date":                      "IsoDate",
	"local_date":                    "LocalDate",
	"local_hour":                    "LocalHour",
	"local_time":                    "LocalTime",
	"page_hash":                     "PageHash",
	"page_hostname":                 "PageHostname",
	"page_load_time_total":         "PageLoadTimeTotal",
	"page_origin":                   "PageOrigin",
	"page_path":                     "PagePath",
	"page_render_time":              "PageRenderTime",
	"page_title":                    "PageTitle",
	"page_url":                      "PageUrl",
	"preview_mode":                  "PreviewMode",
	"random_number":                 "RandomNumber",
	"referrer":                      "Referrer",
	"screen_height":                 "ScreenHeight",
	"screen_height_available":       "ScreenHeightAvailable",
	"screen_width":                  "ScreenWidth",
	"screen_width_available":        "ScreenWidthAvailable",
	"scroll_horizontal_percentage":  "ScrollHorizontalPercentage",
	"scroll_left_pixel":             "ScrollLeftPixel",
	"scroll_source":                 "ScrollSource",
	"scroll_top_pixel":              "ScrollTopPixel",
	"scroll_vertical_percentage":    "ScrollVerticalPercentage",
	"seo_canonical_url":             "SeoCanonicalUrl",
	"seo_num_h1":                    "SeoNumH1",
	"seo_num_h2":                    "SeoNumH2",
	"user_agent":                    "UserAgent",
	"utc_date":                      "UtcDate",
	"visible_element_classes":       "VisibleElementClasses",
	"visible_element_id":            "VisibleElementId",
	"visible_element_node_name":     "VisibleElementNodeName",
	"visible_element_text":          "VisibleElementText",
	"visible_element_url":           "VisibleElementUrl",
	"weekday":                       "Weekday",
}

func (d *tagManagerBuiltinVariableDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tagmanager_builtin_variable"
}

func (d *tagManagerBuiltinVariableDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:    true,
			Description: "Always \"builtin\" - this data source has no configuration, so its id is a fixed constant.",
		},
	}
	for name, id := range builtinVariableIDs {
		attrs[name] = schema.StringAttribute{
			Computed:    true,
			Description: "Matomo's built-in \"" + id + "\" trigger-condition variable identifier.",
		}
	}
	resp.Schema = schema.Schema{
		Description: "Matomo's built-in, always-available trigger-condition variables (e.g. PagePath, ClickId), exposed as named attributes so they can be referenced instead of typed as bare strings. This is a discoverability aid only - it is not an exhaustive or validated list of every value condition.variable accepts (third-party plugins can contribute more, and any user-defined matomo_tagmanager_variable* resource is referenceable via a {{Name}} macro regardless of this data source).",
		Attributes:  attrs,
	}
}

func (d *tagManagerBuiltinVariableDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	attrTypes := map[string]attr.Type{"id": types.StringType}
	attrValues := map[string]attr.Value{"id": types.StringValue("builtin")}
	for name, id := range builtinVariableIDs {
		attrTypes[name] = types.StringType
		attrValues[name] = types.StringValue(id)
	}

	obj, diags := types.ObjectValue(attrTypes, attrValues)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, obj)...)
}
```

`Read` builds the whole state as a single `types.ObjectValue` (rather than a
67-field Go struct with `tfsdk` tags) since this data source's schema is
data-driven from the same `builtinVariableIDs` map used to build it in
`Schema()` - a struct-reflection approach would need the map and the struct
kept in sync by hand for no benefit. Add the missing import this needs:
`"github.com/hashicorp/terraform-plugin-framework/attr"` (`types` is already
imported).

- [ ] **Step 2: Register the data source**

In `internal/provider/provider.go`, add `NewTagManagerBuiltinVariableDataSource`
to the `DataSources` method's returned slice (`provider.go:123-132`):

```go
func (p *MatomoProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewSiteDataSource,
		NewTagManagerContextsDataSource,
		NewTagManagerEnvironmentsDataSource,
		NewTagManagerTagTypesDataSource,
		NewTagManagerTriggerTypesDataSource,
		NewTagManagerVariableTypesDataSource,
		NewTagManagerBuiltinVariableDataSource,
	}
}
```

- [ ] **Step 3: Write the acceptance test**

Create `internal/provider/data_source_tagmanager_builtin_variable_acc_test.go`:

```go
package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTagManagerBuiltinVariableDataSource(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

data "matomo_tagmanager_builtin_variable" "this" {}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.matomo_tagmanager_builtin_variable.this", "id", "builtin"),
					resource.TestCheckResourceAttr("data.matomo_tagmanager_builtin_variable.this", "page_path", "PagePath"),
					resource.TestCheckResourceAttr("data.matomo_tagmanager_builtin_variable.this", "page_hostname", "PageHostname"),
					resource.TestCheckResourceAttr("data.matomo_tagmanager_builtin_variable.this", "click_id", "ClickId"),
				),
			},
		},
	})
}
```

This doesn't require `testAccMatomoClient`/a live Matomo API call for its own
assertions (the data source itself makes none), but keeps `testAccPreCheck`
and the standard acceptance-test shape for consistency with every other data
source in this codebase, and because `provider "matomo" {}` still needs valid
credentials configured to initialize at all.

- [ ] **Step 4: Verify the package builds**

Run: `go build -o /dev/null . && go vet ./...`
Expected: both succeed. This confirms the `attr`/`path` imports and the
`builtinVariableIDs`-driven `Schema()`/`Read()` compile correctly.

- [ ] **Step 5: Commit**

```bash
git add internal/provider/data_source_tagmanager_builtin_variable.go internal/provider/data_source_tagmanager_builtin_variable_acc_test.go internal/provider/provider.go
git commit -m "Add matomo_tagmanager_builtin_variable data source"
```

---

### Task 3: Documentation and full verification pass

**Files:**
- Modify: `docs/resources/tagmanager_trigger.md` (add a usage example showing
  the data-source-reference form of `condition.variable`, per the design
  spec's requirement to document both forms)
- Modify: `internal/provider/typed_trigger_conditions_acc_test.go`'s
  companion doc, if Plan 1 has already merged - otherwise skip (see Step 1)

**Interfaces:**
- Consumes: everything from Tasks 1-2.
- Produces: nothing consumed elsewhere - this is the last task.

- [ ] **Step 1: Check whether Plan 1 has merged yet**

Run: `git log origin/main --oneline | grep -i "trigger condition"`
If Plan 1's commits (rename `actual` to `variable`, typed trigger conditions)
are already on `main`, proceed to Step 2. If not, this task's doc example
should still reference `variable` (Plan 1's rename) since both plans target
the same eventual `main` state regardless of merge order - do not write
`actual` into any new documentation.

- [ ] **Step 2: Add a "both forms" example to the generic trigger's docs**

In `docs/resources/tagmanager_trigger.md`, after the existing `## Example
Usage` section's code block, add:

```markdown
`condition.variable` accepts either a bare Matomo built-in variable
identifier, or a reference to the `matomo_tagmanager_builtin_variable` data
source for editor autocomplete on the known built-in names (this is a
discoverability aid only - both forms are wire-identical, and neither is an
exhaustive list of every value Matomo accepts, since third-party plugins can
contribute more and any `matomo_tagmanager_variable*` resource is
referenceable via a `{{Name}}` macro regardless):

```terraform
data "matomo_tagmanager_builtin_variable" "this" {}

resource "matomo_tagmanager_trigger" "checkout" {
  container_id = matomo_tagmanager_container.main.id
  type         = "PageView"
  name         = "Checkout Page"

  condition {
    comparison = "equals"
    variable   = data.matomo_tagmanager_builtin_variable.this.page_path
    value      = "/checkout"
  }
}
```
```

This environment cannot run `make docs` to regenerate the rest of this page
from schema - only this hand-written prose addition is needed here (the
schema-derived attribute tables below it are unaffected by this task and
regenerate correctly from Plan 1/Task 2's actual schema changes once CI runs
`make docs`).

- [ ] **Step 3: Full local verification pass**

Run:

```bash
go build -o /dev/null . && \
go vet ./... && \
go test ./... -count=1 && \
golangci-lint run ./...
```

Expected: all four succeed; the new acceptance test
(`TestAccTagManagerBuiltinVariableDataSource`) shows as skipped (not failed),
same reasoning as Plan 1's Task 3.

- [ ] **Step 4: Report status**

Summarize Task 1's investigation finding (did the candidates match live
Matomo's discovery output, or were they confirmed only by source
inspection), the local verification results from Step 3, and that CI's
docs-drift-check and the new acceptance test are the remaining
live-verification steps.
