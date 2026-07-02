# Tag Manager discovery data sources — design spec

## 1. Goal and scope

Phase 6 of the original provider design (`docs/superpowers/specs/2026-06-30-matomo-tagmanager-provider-design.md`
§8, §12). Adds five read-only data sources that wrap Matomo Tag Manager's
`getAvailable*` discovery API methods:

- `matomo_tagmanager_contexts`
- `matomo_tagmanager_environments`
- `matomo_tagmanager_tag_types`
- `matomo_tagmanager_trigger_types`
- `matomo_tagmanager_variable_types`

These exist so Terraform configs can discover valid values for the
generic fallback resources' `type` argument and the Phase 7 publish
action's `environment` argument, without leaving Terraform or reading
Matomo's admin UI/PHP source by hand (as this project's own development
has repeatedly had to do).

Out of scope: any change to the generic `matomo_tagmanager_{tag,trigger,variable}`
resources themselves (e.g. adding a `type` validator that calls out to
Matomo) — these data sources provide the information; wiring it into
validation, if wanted, is a separate follow-up.

## 2. Matomo API methods (confirmed against `matomo-org/tag-manager`, branch `5.x-dev`)

- `TagManager.getAvailableContexts` — no parameters. Returns
  `[{id, name}, ...]` (`Context/BaseContext.php`'s `toArray()`).
- `TagManager.getAvailableEnvironments` — no parameters. Returns
  `[{id, name}, ...]` (`Model/Environment.php`'s `getEnvironments()`,
  confirmed via `tests/Integration/Model/EnvironmentTest.php`'s
  `test_getEnvironments()`: `[{'id' => 'live', 'name' => 'Live'}, ...]`).
- `TagManager.getAvailableTagTypesInContext` / `...TriggerTypesInContext` /
  `...VariableTypesInContext` — already wrapped by
  `Client.GetAvailableTagTypes`/`GetAvailableTriggerTypes`/`GetAvailableVariableTypes`
  in `internal/matomo/tagmanager_templates.go` (used by `tools/gen`).
  Each takes `idContext` and returns `[]Template{ID, Name, Description,
  Category, Parameters}` (categories pre-flattened by the existing
  client method).

`getAvailableEnvironmentsWithPublishCapability` (a `getAvailableEnvironments`
variant filtered by the calling user's publish permission on a given
site) is explicitly out of scope — it needs an `idSite`, which doesn't
fit this design's parameter-less `environments` data source, and no use
case for it has come up.

## 3. New client code

New file `internal/matomo/tagmanager_discovery.go`:

```go
package matomo

import "context"

// Context describes one Tag Manager context (e.g. "web", "amp", "mobile"),
// as returned by TagManager.getAvailableContexts.
type Context struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GetAvailableContexts returns every context this Matomo instance
// supports (only contexts with at least one available tag type are
// included - confirmed via API.php's getAvailableContexts()).
func (c *Client) GetAvailableContexts(ctx context.Context) ([]Context, error) {
	var contexts []Context
	if err := c.call(ctx, "TagManager.getAvailableContexts", nil, &contexts); err != nil {
		return nil, err
	}
	return contexts, nil
}

// Environment describes one Tag Manager publish environment (e.g.
// "live", "staging", "dev"), as returned by
// TagManager.getAvailableEnvironments.
type Environment struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GetAvailableEnvironments returns every environment configured on this
// Matomo instance.
func (c *Client) GetAvailableEnvironments(ctx context.Context) ([]Environment, error) {
	var environments []Environment
	if err := c.call(ctx, "TagManager.getAvailableEnvironments", nil, &environments); err != nil {
		return nil, err
	}
	return environments, nil
}
```

(Exact `c.call` signature/nil-params handling to match whatever
`GetAvailableTagTypes` etc. already do — verified against
`tagmanager_templates.go` at implementation time, not re-derived here.)

No changes to `Template`, `TemplateParam`, or the three existing
`GetAvailable*Types` methods.

## 4. Data source schemas

All five follow `internal/provider/data_source_site.go`'s existing shape:
a struct holding `*matomo.Client`, `Metadata`/`Schema`/`Configure`/`Read`,
registered as `datasource.DataSource` + `datasource.DataSourceWithConfigure`.
One file per data source under `internal/provider/`:

- `data_source_tagmanager_contexts.go`
- `data_source_tagmanager_environments.go`
- `data_source_tagmanager_tag_types.go`
- `data_source_tagmanager_trigger_types.go`
- `data_source_tagmanager_variable_types.go`

### 4.1 `matomo_tagmanager_contexts`

```hcl
data "matomo_tagmanager_contexts" "all" {}

output "context_ids" {
  value = [for c in data.matomo_tagmanager_contexts.all.contexts : c.id]
}
```

Schema:
- `id` (computed `types.String`) — synthetic, always `"contexts"`. Data
  sources require an `id` attribute by Plugin Framework convention; this
  data source has no natural key since it takes no input.
- `contexts` (computed `types.List` of `types.Object{id: String, name: String}`).

`Read()`: calls `client.GetAvailableContexts(ctx)`, maps each `Context`
into the object list, sets `id = "contexts"`.

### 4.2 `matomo_tagmanager_environments`

Same shape as 4.1, substituting `Environment`/`environments`, synthetic
`id = "environments"`.

### 4.3 `matomo_tagmanager_tag_types` (and 4.4/4.5, identical shape for triggers/variables)

```hcl
data "matomo_tagmanager_tag_types" "web" {
  context = "web"
}

output "tag_type_ids" {
  value = [for t in data.matomo_tagmanager_tag_types.web.tag_types : t.id]
}
```

Schema:
- `id` (computed `types.String`) — set to the `context` value on read,
  so it's stable and meaningful (unlike the parameter-less pair above,
  this data source does have a natural key: the context it was queried
  for).
- `context` (required `types.String`) — the Tag Manager context to query
  (e.g. `"web"`). No default; per the approved design decision, context
  must always be explicit since available types can vary by context.
- `tag_types` / `trigger_types` / `variable_types` (computed `types.List`
  of `types.Object{id: String, name: String, description: String,
  category: String}`) — summary fields only. The full `Parameters`
  slice on `matomo.Template` (each type's configurable settings —
  name/type/description/condition/defaultValue/availableValues) is
  intentionally NOT exposed; that level of detail mirrors `tools/gen`'s
  internal schema-parsing concerns, and nothing in this design's stated
  use cases (discovering/validating `type` strings) needs it. Revisit
  only if a concrete need for field-level introspection from HCL shows up.

`Read()`: calls the corresponding existing `Client.GetAvailable*Types(ctx, context)`,
maps `[]matomo.Template` to the object list, sets `id = context`.

## 5. Provider registration

`internal/provider/provider.go`'s `DataSources()`:

```go
func (p *MatomoProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewSiteDataSource,
		NewTagManagerContextsDataSource,
		NewTagManagerEnvironmentsDataSource,
		NewTagManagerTagTypesDataSource,
		NewTagManagerTriggerTypesDataSource,
		NewTagManagerVariableTypesDataSource,
	}
}
```

## 6. Testing

- **Unit tests** (`internal/matomo/tagmanager_discovery_test.go`): one
  `httptest`-fixture test per new client method, following
  `tagmanager_templates_test.go`'s existing pattern (fixture server
  returns the real wire shape, assert the decoded Go values). Covers the
  empty-list case and the multi-item case.
- No `internal/provider`-layer unit tests: matching the existing
  convention (`data_source_site.go` has no non-acceptance test file),
  data source `Read()` correctness is covered by the client-layer unit
  tests above plus the acceptance tests below - there is no
  `resource.UnitTest`-based mock-client pattern used anywhere in this
  codebase for resources/data sources, and introducing one here would be
  new project-wide precedent, not a local decision.
- **Acceptance tests** (`TF_ACC=1`, `internal/provider/data_source_tagmanager_*_acc_test.go`):
  one per data source against live Matomo (docker-compose fixture),
  asserting the list is non-empty and (for the three type-listing data
  sources) that a known built-in type ID (e.g. `"CustomHtml"` for tags)
  appears in the result. Follows the existing acceptance test file
  naming/registration pattern (`testAccPreCheck`, `testAccProtoV6ProviderFactories`).

## 7. Documentation

`tfplugindocs generate` picks these up automatically once schemas have
`Description` fields set (existing project convention — see `docs/`
generation step in CI). Each new data source file needs `examples/data-sources/<name>/data-source.tf`
per the existing `examples/` layout Phase 8 established for other
resources/data sources, so `tfplugindocs` has a real example to embed.

## 8. Explicitly out of scope

- `TagManager.getAvailableEnvironmentsWithPublishCapability` (needs
  `idSite`, no stated use case).
- Any validator wiring in the generic `matomo_tagmanager_{tag,trigger,variable}`
  resources that consumes these data sources (e.g. a `type` string
  validator) — these data sources only provide the information.
- Exposing per-type `Parameters` detail (field name/type/condition/etc)
  through the tag/trigger/variable type data sources.
