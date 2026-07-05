# Tag Manager Schema Completeness (description, priority, container flags) - Design

## 1. Goal and scope

Fix three real schema gaps in the current provider surface, each confirmed
against Matomo's live PHP source (vendored copy of `matomo-org/tag-manager`,
`5.x-dev` branch, from earlier investigation work) rather than assumed:

1. **`description`** - Matomo's tags, triggers, and variables each carry their
   own `description` field, entirely separate from a container's description.
   Only `matomo_tagmanager_container` exposes this today; the typed generated
   resources and the three generic fallback resources have no way to set it.
2. **Tag `priority`** - a per-tag execution-order field (`tag.priority
   SMALLINT UNSIGNED`, lower value fires earlier) that exists nowhere in this
   provider's Go client or Terraform schema, typed or generic.
3. **Container flags** - `ignoreGtmDataLayer`, `activelySyncGtmDataLayer`,
   `isTagFireLimitAllowedInPreviewMode` are real `addContainer`/
   `updateContainer` boolean parameters, entirely absent from both the Go
   client's `Container` struct and `matomo_tagmanager_container`'s schema.

All three are additive (new optional fields on existing resources) - no
existing field changes shape or is renamed. One spec, one plan, one PR.

## 2. Confirmed wire facts

Evidence (file:line citations from the vendored PHP source):

- **description**: `Dao/{Tags,Triggers,Variables}Dao.php` each declare a
  `description VARCHAR(...) NOT NULL` column; `API.php`'s
  `addContainerTag`/`updateContainerTag` (and the Trigger/Variable
  equivalents) each take `$description = ''` as an optional parameter,
  documented `@param string|null $description Optional ... description.`
- **priority**: `Dao/TagsDao.php` declares `priority SMALLINT(5) UNSIGNED NOT
  NULL`. `API.php`'s `addContainerTag`/`updateContainerTag` take `$priority =
  999` as a positional parameter alongside `fireTriggerIds`/`blockTriggerIds`.
  Validated via `NumberRange(0, NumberRange::MAX_SMALL_INT_UNSIGNED)`. Tags are
  fetched `ORDER BY priority, created_date ASC` - this is a real, meaningful
  field affecting multi-tag fire order, not decorative.
- **container flags**: `Dao/ContainersDao.php` declares all three as
  `TINYINT(1) UNSIGNED NOT NULL`. `API.php`'s `addContainer`/`updateContainer`
  take `$ignoreGtmDataLayer = 0, $isTagFireLimitAllowedInPreviewMode = 0,
  $activelySyncGtmDataLayer = 0` - all three default to `0`/false at the API
  layer (the DB column's `activelySyncGtmDataLayer DEFAULT 1` only matters for
  a raw SQL insert bypassing the API, which this provider never does).

## 3. `description` on tags, triggers, variables

### 3.1 Typed resources

Following the exact precedent set by typed trigger conditions (previous
design): add `Description types.String \`tfsdk:"description"\`` to
`typedTagCommon`, `typedTriggerCommon`, `typedVariableCommon`
(`internal/provider/typed_{tag,trigger,variable}_resource.go`), and a matching
common-attribute entry in `tools/gen/templates/schema.go.tmpl`'s shared
attributes block so every current and future generated resource picks it up
automatically on the next `tools/gen` run - no per-type hand-editing.

Schema shape (identical across all three kinds):

```go
"description": schema.StringAttribute{
    Optional:      true,
    Computed:      true,
    PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
},
```

Unlike the enum-constrained optional strings elsewhere in this codebase
(which omit the wire key entirely when unset, because Matomo's field
validators reject an empty string on constrained fields), `description` has
no such constraint - Matomo's own default *is* `""`. So the shared
runtime's Create/Update always sends `description`, using `""` when the
config value is null:

```go
description := common.Description.ValueString() // "" if null - always legal here
```

Read always sets `Description = types.StringValue(resp.Description)` (never
conditionally null), since Matomo always returns the field.

### 3.2 Generic resources

`resource_tagmanager_tag.go`, `resource_tagmanager_trigger.go`,
`resource_tagmanager_variable.go` each gain the identical `description`
attribute and the same always-send/always-read convention, wired through
their existing Create/Update/Read bodies.

### 3.3 Client layer

`internal/matomo/tagmanager_{tags,triggers,variables}.go`: each of
`Tag`/`TagParams`, `Trigger`/`TriggerParams`, `Variable`/`VariableParams`
gains `Description string \`json:"description"\``, and each
`AddContainer{Tag,Trigger,Variable}`/`UpdateContainer{Tag,Trigger,Variable}`
client method gains a `description string` parameter, sent as the
`description` form value.

## 4. Tag `priority`

### 4.1 Typed resources

Add `Priority types.Int64 \`tfsdk:"priority"\`` to `typedTagCommon` and the
common-attributes block in `tools/gen/templates/schema.go.tmpl` (tag template
only - triggers/variables have no priority):

```go
"priority": schema.Int64Attribute{
    Optional:      true,
    Computed:      true,
    PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
},
```

Matomo's own default is `999` when the param is omitted - so, matching the
`description` convention, Create/Update always sends a value: `priority :=
999` if the config value is null, else the configured value. Read always sets
`Priority = types.Int64Value(int64(resp.Priority))`.

### 4.2 Generic resource

`resource_tagmanager_tag.go` gains the identical `priority` attribute and
always-send/always-read convention.

### 4.3 Client layer

`internal/matomo/tagmanager_tags.go`: `Tag`/`TagParams` gain `Priority int
\`json:"priority"\``. `AddContainerTag`/`UpdateContainerTag` gain a `priority
int` parameter, sent as the `priority` form value (positioned alongside the
existing `fireTriggerIds`/`blockTriggerIds` parameters per Matomo's real
signature).

## 5. Container flags

Simpler than sections 3-4: `resource_tagmanager_container.go` is hand-written,
not generated, so this is a direct schema + client change with no `tools/gen`
involvement.

### 5.1 Schema

Three new attributes on `matomo_tagmanager_container`, each following the same
Optional+Computed+UseStateForUnknown shape (Matomo default `false` for all
three):

```go
"ignore_gtm_data_layer": schema.BoolAttribute{
    Optional:      true,
    Computed:      true,
    PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
},
"actively_sync_gtm_data_layer": schema.BoolAttribute{
    Optional:      true,
    Computed:      true,
    PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
},
"is_tag_fire_limit_allowed_in_preview_mode": schema.BoolAttribute{
    Optional:      true,
    Computed:      true,
    PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
},
```

Model fields: `IgnoreGtmDataLayer`, `ActivelySyncGtmDataLayer`,
`IsTagFireLimitAllowedInPreviewMode`, all `types.Bool`. Create/Update always
send all three (`false` when the config value is null, matching Matomo's own
API-layer default). Read always sets all three from the response (never
conditionally null).

### 5.2 Client layer

`internal/matomo/tagmanager_containers.go`: `Container` gains

```go
IgnoreGtmDataLayer                 bool `json:"ignoreGtmDataLayer"`
ActivelySyncGtmDataLayer           bool `json:"activelySyncGtmDataLayer"`
IsTagFireLimitAllowedInPreviewMode bool `json:"isTagFireLimitAllowedInPreviewMode"`
```

`AddContainer`/`UpdateContainer` each gain three `bool` parameters, sent as
`"0"`/`"1"` form values via the existing `boolToIntString` helper
(`internal/matomo/sites.go:100-105`, already used by `AddSite`/`UpdateSite`) -
reused as-is, not reimplemented.

## 6. Testing

- Unit tests for the new client methods' request encoding (Description/
  Priority/the three bools sent as expected form values) alongside existing
  tests in `internal/matomo/tagmanager_{tags,containers}_test.go` (create if
  no existing unit-test file covers these methods).
- Acceptance test additions (not new test functions where an existing
  `_basic`/`_import` test already covers the resource - extend those configs):
  - `matomo_tagmanager_container`: assert all three new bools round-trip, both
    explicitly set and left unconfigured (defaulting to `false`).
  - `matomo_tagmanager_tag` (generic) and at least one typed tag resource
    (e.g. `matomo_tagmanager_tag_customhtml`): assert `description` and
    `priority` round-trip, both set and unconfigured.
  - `matomo_tagmanager_trigger` (generic) and at least one typed trigger
    resource: assert `description` round-trips.
  - `matomo_tagmanager_variable` (generic) and at least one typed variable
    resource: assert `description` round-trips.
- `tools/gen`'s existing regenerate-and-diff CI step (from the previous
  design) is the correctness check for the generated-file changes - no new
  test infrastructure needed there, just running the generator and committing
  the result, as established practice.
- Docs: `tfplugindocs`-regenerated markdown for every touched resource. This
  sandbox cannot run `make docs` (network-blocked) - reconstruct the docs diff
  by hand from the schema `Description` fields and existing doc structure, as
  done in the previous design's CI-log-reconstruction workaround, or run it in
  CI/locally outside the sandbox if available.

## 7. Out of scope

- No changes to any existing field's shape, name, or behavior.
- No validation beyond Matomo's own (e.g. no client-side range check on
  `priority` beyond what `Int64Attribute` already provides) - Matomo's own API
  validates server-side and returns an error Terraform surfaces normally.
- No change to how `description` behaves on containers (already correct,
  already Optional+Computed+matching this exact pattern) - this design only
  extends the same shape to tags/triggers/variables.
