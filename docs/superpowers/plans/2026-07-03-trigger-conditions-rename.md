# Trigger Conditions Rename + Typed Parity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rename the generic trigger resource's `condition.actual` attribute to `condition.variable` (clearer name, matches what it actually represents - a reference to a built-in or user-defined Variable), and bring every typed `matomo_tagmanager_trigger_<type>` resource to full parity with the generic trigger resource's `condition{}` support, which it currently lacks entirely.

**Architecture:** The rename touches only the generic trigger resource (`resource_tagmanager_trigger.go`) and its existing acceptance test/docs. Typed-trigger condition support is added entirely to the *shared* runtime (`typed_trigger_resource.go`) - the `condition` schema Block is injected into each generated type's schema at request time (in `Schema()`), and `typedTriggerCommon` gains a `Condition` field alongside the existing `ID`/`ContainerID`/`Name` - so every current and future generated trigger type gets conditions automatically, with zero changes to any `generated_trigger_*.go` file and zero need to regenerate anything via `tools/gen` (which requires live Matomo API access this environment may not always have).

**Tech Stack:** Go, terraform-plugin-framework (`schema.ListNestedBlock`), existing `matomo.Condition`/`matomo.TriggerParams` client types.

## Global Constraints

- This is a breaking schema change to an already-shipped attribute
  (`condition.actual` -> `condition.variable`) - acceptable now because no
  real tagged release has gone out yet. Do not add a deprecation/compatibility
  shim for the old name.
- Typed-trigger condition support must require ZERO changes to any
  `internal/provider/generated_trigger_*.go` file and zero changes to
  `tools/gen` - achieved by injecting the shared `condition` schema Block at
  runtime in `typed_trigger_resource.go`'s `Schema()` method, not by
  regenerating per-type schema functions.
- Reuse the existing `triggerConditionModel`, `conditionsToParams`, and
  `conditionsFromAPI` (all defined in `resource_tagmanager_trigger.go`) for
  the typed-trigger side too - do not define a second, parallel condition
  type/conversion pair.
- This environment cannot run `make docs` (network-blocked access to
  `checkpoint-api.hashicorp.com` for the Terraform CLI binary tfplugindocs
  needs) or acceptance tests requiring a live Matomo instance (no Docker
  daemon). Hand-edit `docs/resources/tagmanager_trigger.md` directly for the
  rename; defer all `TestAcc*`/`TF_ACC=1` runs to CI, matching this project's
  established convention throughout its history.

---

### Task 1: Rename `condition.actual` to `condition.variable` on the generic trigger resource

**Files:**
- Modify: `internal/provider/resource_tagmanager_trigger.go:31-35,90-99,116-138`
- Modify: `internal/provider/resource_tagmanager_trigger_acc_test.go:95,100,108,111`
- Modify: `docs/resources/tagmanager_trigger.md:46`

**Interfaces:**
- Consumes: nothing from other tasks (this is the first task).
- Produces: `triggerConditionModel` (renamed field `Actual` stays the same Go
  field name and Go type `types.String`, only its `tfsdk` tag changes from
  `"actual"` to `"variable"`), `conditionsToParams(conditions
  []triggerConditionModel) []matomo.Condition`, and `conditionsFromAPI(conditions
  []matomo.Condition) []triggerConditionModel` - Task 2 reuses all three of
  these exactly as they exist after this task, with no signature changes.

- [ ] **Step 1: Rename the schema attribute**

In `internal/provider/resource_tagmanager_trigger.go`, change the `condition`
block's nested attribute (currently at lines 90-99):

```go
			"condition": schema.ListNestedBlock{
				Description: "Conditions that must all match for this trigger to fire.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"comparison": schema.StringAttribute{Required: true},
						"variable":   schema.StringAttribute{Required: true, Description: "A reference to a Matomo built-in variable (e.g. \"PagePath\" - see the matomo_tagmanager_builtin_variable data source) or a user-defined variable macro (e.g. \"{{My Variable}}\")."},
						"value":      schema.StringAttribute{Required: true},
					},
				},
			},
```

(The `matomo_tagmanager_builtin_variable` data source mentioned in the
description does not exist yet as of this plan - it's built in a separate,
later plan. The description text is still accurate today: `variable` already
accepts either form, the data source is purely a future convenience for
producing the first form.)

- [ ] **Step 2: Rename the Go model field's tfsdk tag**

Change `triggerConditionModel` (currently at lines 31-35):

```go
type triggerConditionModel struct {
	Comparison types.String `tfsdk:"comparison"`
	Actual     types.String `tfsdk:"variable"`
	Value      types.String `tfsdk:"value"`
}
```

(Only the tfsdk struct tag changes, from `"actual"` to `"variable"` - the Go
field name `Actual` is left as-is deliberately, since renaming it too would
touch every call site in this file for zero added clarity within Go code that
already says `c.Actual` in a `conditionsToParams`/`conditionsFromAPI` context
where "the actual value of this condition" reads fine locally; only the
Terraform-facing name needed to change.)

- [ ] **Step 3: Update the two acceptance test files' `actual` references**

In `internal/provider/resource_tagmanager_trigger_acc_test.go`, change lines
95 and 100 (HCL config text) and 108 and 111 (`TestCheckResourceAttr` calls):

```go
  condition {
    comparison = "equals"
    variable   = "PagePath"
    value      = "/checkout"
  }
  condition {
    comparison = "contains"
    variable   = "PageHostname"
    value      = "example.com"
  }
```

```go
					resource.TestCheckResourceAttr("matomo_tagmanager_trigger.test", "condition.0.variable", "PagePath"),
					resource.TestCheckResourceAttr("matomo_tagmanager_trigger.test", "condition.1.variable", "PageHostname"),
```

- [ ] **Step 4: Update the docs page by hand**

In `docs/resources/tagmanager_trigger.md`, change line 46 from:

```
- `actual` (String) A Matomo "actual value" identifier (e.g. "url_path") or a variable macro reference (e.g. "{{My Variable}}").
```

to (matching the new schema description from Step 1, and alphabetically
resorted since tfplugindocs lists nested-block attributes alphabetically -
`comparison`, then `value`, then `variable` in that order):

```
- `comparison` (String)
- `value` (String)
- `variable` (String) A reference to a Matomo built-in variable (e.g. "PagePath" - see the matomo_tagmanager_builtin_variable data source) or a user-defined variable macro (e.g. "{{My Variable}}").
```

Read the current "Nested Schema for `condition`" section first
(`docs/resources/tagmanager_trigger.md:41-48` as of this plan) to confirm the
exact surrounding lines before replacing - the "Required:" header line above
the three attributes stays as-is.

- [ ] **Step 5: Verify the package builds and existing unit tests still pass**

Run: `go build -o /dev/null . && go test ./internal/provider/... -count=1`
Expected: builds clean; `ok` for the `internal/provider` package (the
acceptance test changed in Step 3 is gated behind `TestAccPreCheck`/`TF_ACC`
and will skip, not fail, in this sandbox - this only confirms the package
still compiles with the renamed field).

- [ ] **Step 6: Commit**

```bash
git add internal/provider/resource_tagmanager_trigger.go internal/provider/resource_tagmanager_trigger_acc_test.go docs/resources/tagmanager_trigger.md
git commit -m "Rename generic trigger condition's actual attribute to variable"
```

---

### Task 2: Typed trigger conditions parity

**Files:**
- Modify: `internal/provider/typed_trigger_resource.go` (all of it - `typedTriggerCommon` struct, `Schema()`, `Create()`, `Update()`, `Read()`)
- Create: `internal/provider/typed_trigger_conditions_acc_test.go`

**Interfaces:**
- Consumes: `triggerConditionModel`, `conditionsToParams`, `conditionsFromAPI`
  (all from Task 1's `resource_tagmanager_trigger.go`, package-level, no
  import needed - same package `provider`).
- Consumes: `triggerCustomeventModel` (from
  `internal/provider/generated_trigger_customevent.go`, unmodified by this
  task) - specifically its `Meta().TypeID` = `"CustomEvent"`, `Meta().ResourceName`
  = `"matomo_tagmanager_trigger_customevent"`, and its one type-specific field
  `EventName types.String \`tfsdk:"event_name"\`` (`Required`) - used as the
  concrete generated type to test conditions against in this task's
  acceptance test.
- Produces: nothing new consumed by later tasks in this plan (this is the
  last task).

- [ ] **Step 1: Add the `Condition` field to `typedTriggerCommon`**

In `internal/provider/typed_trigger_resource.go`, change:

```go
type typedTriggerCommon struct {
	ID          types.String `tfsdk:"id"`
	ContainerID types.String `tfsdk:"container_id"`
	Name        types.String `tfsdk:"name"`
}
```

to:

```go
type typedTriggerCommon struct {
	ID          types.String            `tfsdk:"id"`
	ContainerID types.String            `tfsdk:"container_id"`
	Name        types.String            `tfsdk:"name"`
	Condition   []triggerConditionModel `tfsdk:"condition"`
}
```

- [ ] **Step 2: Inject the shared `condition` Block into every generated type's schema**

Change `Schema()`:

```go
func (r *typedTriggerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = r.newModel().Meta().Schema
}
```

to:

```go
// Schema injects the shared "condition" block into every generated trigger
// type's own schema, rather than each generated_trigger_*.go file declaring
// it independently - this is what makes conditions apply automatically to
// every current and future generated trigger type with zero regeneration.
func (r *typedTriggerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := r.newModel().Meta().Schema
	if s.Blocks == nil {
		s.Blocks = map[string]schema.Block{}
	}
	s.Blocks["condition"] = schema.ListNestedBlock{
		Description: "Conditions that must all match for this trigger to fire.",
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"comparison": schema.StringAttribute{Required: true},
				"variable":   schema.StringAttribute{Required: true, Description: "A reference to a Matomo built-in variable (e.g. \"PagePath\" - see the matomo_tagmanager_builtin_variable data source) or a user-defined variable macro (e.g. \"{{My Variable}}\")."},
				"value":      schema.StringAttribute{Required: true},
			},
		},
	}
	resp.Schema = s
}
```

This needs a new import - add `"github.com/hashicorp/terraform-plugin-framework/resource/schema"`
to `typed_trigger_resource.go`'s import block (it's not currently imported
there; `resource/schema` and `resource` are separate packages and this file
today only imports `resource`, `path`, and `types`).

- [ ] **Step 3: Wire `Conditions` into `Create` and `Update`**

Change the `matomo.TriggerParams{...}` literal in `Create` (currently):

```go
	idTrigger, err := r.client.AddContainerTrigger(ctx, siteID, idContainer, versionID, matomo.TriggerParams{
		Type:       model.Meta().TypeID,
		Name:       common.Name.ValueString(),
		Parameters: model.ToParams(),
	})
```

to:

```go
	idTrigger, err := r.client.AddContainerTrigger(ctx, siteID, idContainer, versionID, matomo.TriggerParams{
		Type:       model.Meta().TypeID,
		Name:       common.Name.ValueString(),
		Parameters: model.ToParams(),
		Conditions: conditionsToParams(common.Condition),
	})
```

And the equivalent literal in `Update` (currently):

```go
	if err := r.client.UpdateContainerTrigger(ctx, siteID, idContainer, versionID, idTrigger, matomo.TriggerParams{
		Type:       model.Meta().TypeID,
		Name:       common.Name.ValueString(),
		Parameters: model.ToParams(),
	}); err != nil {
```

to:

```go
	if err := r.client.UpdateContainerTrigger(ctx, siteID, idContainer, versionID, idTrigger, matomo.TriggerParams{
		Type:       model.Meta().TypeID,
		Name:       common.Name.ValueString(),
		Parameters: model.ToParams(),
		Conditions: conditionsToParams(common.Condition),
	}); err != nil {
```

- [ ] **Step 4: Populate `Condition` on read, in both `Create` and `Read`**

In `Create`, after the existing `common.Name = types.StringValue(trig.Name)`
line (added when the trigger is read back post-creation), add:

```go
	common.Condition = conditionsFromAPI(trig.Conditions)
```

In `Read`, after the existing `common.Name = types.StringValue(trig.Name)`
line, add the same:

```go
	common.Condition = conditionsFromAPI(trig.Conditions)
```

- [ ] **Step 5: Write the acceptance test**

Create `internal/provider/typed_trigger_conditions_acc_test.go`:

```go
package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// Exercises condition support on a single generated trigger type
// (CustomEvent), proving typed_trigger_resource.go's shared Schema()
// injection and Create/Update/Read wiring work end-to-end against a real
// Matomo instance - conditions are shared runtime, not per-type generated
// code, so one type is sufficient coverage (see typed_trigger_resource.go's
// Schema() doc comment).
func TestAccTypedTriggerConditions_customevent(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Typed Trigger Conditions Acceptance Site"
  urls = ["https://acc-typed-trigger-conditions-test.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Typed Trigger Conditions Acceptance Container"
}

resource "matomo_tagmanager_trigger_customevent" "test" {
  container_id = matomo_tagmanager_container.test.id
  name         = "Acceptance Conditions Trigger"
  event_name   = "add_to_cart"
  condition {
    comparison = "equals"
    variable   = "PagePath"
    value      = "/checkout"
  }
  condition {
    comparison = "contains"
    variable   = "PageHostname"
    value      = "example.com"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("matomo_tagmanager_trigger_customevent.test", "condition.#", "2"),
					resource.TestCheckResourceAttr("matomo_tagmanager_trigger_customevent.test", "condition.0.comparison", "equals"),
					resource.TestCheckResourceAttr("matomo_tagmanager_trigger_customevent.test", "condition.0.variable", "PagePath"),
					resource.TestCheckResourceAttr("matomo_tagmanager_trigger_customevent.test", "condition.0.value", "/checkout"),
					resource.TestCheckResourceAttr("matomo_tagmanager_trigger_customevent.test", "condition.1.comparison", "contains"),
					resource.TestCheckResourceAttr("matomo_tagmanager_trigger_customevent.test", "condition.1.variable", "PageHostname"),
					resource.TestCheckResourceAttr("matomo_tagmanager_trigger_customevent.test", "condition.1.value", "example.com"),
				),
			},
			{
				ResourceName:      "matomo_tagmanager_trigger_customevent.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
```

This mirrors `TestAccTagManagerTriggerResource_withConditions`
(`resource_tagmanager_trigger_acc_test.go`) exactly, substituting the typed
`matomo_tagmanager_trigger_customevent` resource (with its required
`event_name` field) for the generic one, plus an import-verify step (the
generic test doesn't have one; typed resources' existing acceptance test
pattern - see any `generated_trigger_*_acc_test.go` - always includes one, so
this stays consistent with that convention).

- [ ] **Step 6: Verify the package builds**

Run: `go build -o /dev/null . && go vet ./...`
Expected: both succeed with no output. This confirms `typed_trigger_resource.go`'s
new `schema` import is used correctly and every generated trigger file (which
embeds `typedTriggerCommon`) still compiles with the added `Condition` field -
a compile failure here would mean some generated file's model conflicts with
the new embedded field name, which would be a real, structural problem to
stop and investigate rather than a step to explain away.

- [ ] **Step 7: Commit**

```bash
git add internal/provider/typed_trigger_resource.go internal/provider/typed_trigger_conditions_acc_test.go
git commit -m "Add trigger condition support to typed trigger resources

Conditions are wired into the shared typed_trigger_resource.go runtime
(schema injection + Create/Update/Read), not per generated type, so
every current and future generated trigger type gets full condition
parity with the generic trigger resource automatically."
```

---

### Task 3: Full verification pass

**Files:** none (verification only).

**Interfaces:**
- Consumes: everything from Tasks 1-2.
- Produces: confidence the whole branch is ready for CI/review before
  opening a PR.

- [ ] **Step 1: Full local build/lint/test pass**

Run:

```bash
go build -o /dev/null . && \
go vet ./... && \
go test ./... -count=1 && \
golangci-lint run ./...
```

Expected: all four succeed. `go test` will show the two new/changed
acceptance tests (`TestAccTagManagerTriggerResource_withConditions`,
`TestAccTypedTriggerConditions_customevent`) as skipped (not failed) with a
message like `SKIP: Acceptance tests skipped unless TF_ACC=1 set` -
confirmed this way rather than run live, per this environment's Docker
limitation (see Global Constraints).

- [ ] **Step 2: Confirm the docs hand-edit matches what `tfplugindocs` would generate**

This environment cannot run `make docs` (network-blocked, see Global
Constraints) - the docs hand-edit from Task 1 Step 4 must instead be verified
once this branch's CI run completes (`ci.yml`'s "Check docs are up to date"
step, which regenerates and diffs `docs/`). If that step fails, the job log
will show the exact diff needed (the same pattern used successfully earlier
in this project's history to fix hand-edited docs pages without local
`terraform`/network access) - fix and push a follow-up commit rather than
guessing further by hand.

- [ ] **Step 3: Report status**

Summarize the local verification results (Step 1) and note that CI's
docs-drift-check and the two acceptance tests are the remaining
live-verification steps, to be confirmed once this branch's CI run completes.
