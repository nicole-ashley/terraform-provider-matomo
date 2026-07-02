# Tag Manager Discovery Data Sources Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add five read-only Terraform data sources (`matomo_tagmanager_contexts`, `matomo_tagmanager_environments`, `matomo_tagmanager_tag_types`, `matomo_tagmanager_trigger_types`, `matomo_tagmanager_variable_types`) that expose Matomo Tag Manager's `getAvailable*` discovery API methods.

**Architecture:** Two new `internal/matomo` client methods (`GetAvailableContexts`, `GetAvailableEnvironments`) alongside the three that already exist (`GetAvailableTagTypes`/`GetAvailableTriggerTypes`/`GetAvailableVariableTypes`); five new `internal/provider` data source files following `data_source_site.go`'s existing shape (`Metadata`/`Schema`/`Configure`/`Read`, registered in `provider.go`'s `DataSources()`); each `Read()` maps a client response directly into a `tfsdk`-tagged Go struct that the framework serializes into state.

**Tech Stack:** Go, `terraform-plugin-framework` (`datasource`/`datasource/schema` packages, `schema.ListNestedAttribute` for nested list output), `terraform-plugin-testing` for acceptance tests, standard library `net/http/httptest` for client unit tests.

## Global Constraints

- Matomo API methods used: `TagManager.getAvailableContexts` (no params, returns `[{id, name}, ...]`), `TagManager.getAvailableEnvironments` (no params, returns `[{id, name}, ...]`) — both confirmed against `matomo-org/tag-manager` `API.php`/`Model/Environment.php`/`Context/BaseContext.php`.
- The three type-listing data sources (`tag_types`/`trigger_types`/`variable_types`) reuse the **existing** `Client.GetAvailableTagTypes`/`GetAvailableTriggerTypes`/`GetAvailableVariableTypes(ctx, idContext) ([]matomo.Template, error)` in `internal/matomo/tagmanager_templates.go` — do not add new client methods for these three.
- `context` is a **required** input on the three type-listing data sources (no default) — Matomo's available types can vary by context.
- Type-listing data sources expose only summary fields (`id`, `name`, `description`, `category`) — never `matomo.Template.Parameters`.
- No `internal/provider`-layer unit tests (no mock-`matomo.Client` pattern exists anywhere in this codebase for resources/data sources — `data_source_site.go` itself has none). Coverage is client-layer `httptest` unit tests + acceptance tests only.
- No `examples/` or `tfplugindocs` work — this repo has no docs-generation CI step yet (that's Phase 8, out of scope here).
- Full spec: `docs/superpowers/specs/2026-07-02-tagmanager-discovery-datasources-design.md`.

---

## File Structure

- Create: `internal/matomo/tagmanager_discovery.go` — `Context`, `Environment` types + `GetAvailableContexts`/`GetAvailableEnvironments` client methods.
- Create: `internal/matomo/tagmanager_discovery_test.go` — unit tests for both new client methods.
- Create: `internal/provider/data_source_tagmanager_contexts.go` — `matomo_tagmanager_contexts` data source.
- Create: `internal/provider/data_source_tagmanager_contexts_acc_test.go` — its acceptance test.
- Create: `internal/provider/data_source_tagmanager_environments.go` — `matomo_tagmanager_environments` data source.
- Create: `internal/provider/data_source_tagmanager_environments_acc_test.go` — its acceptance test.
- Create: `internal/provider/data_source_tagmanager_tag_types.go` — `matomo_tagmanager_tag_types` data source.
- Create: `internal/provider/data_source_tagmanager_tag_types_acc_test.go` — its acceptance test.
- Create: `internal/provider/data_source_tagmanager_trigger_types.go` — `matomo_tagmanager_trigger_types` data source.
- Create: `internal/provider/data_source_tagmanager_trigger_types_acc_test.go` — its acceptance test.
- Create: `internal/provider/data_source_tagmanager_variable_types.go` — `matomo_tagmanager_variable_types` data source.
- Create: `internal/provider/data_source_tagmanager_variable_types_acc_test.go` — its acceptance test.
- Modify: `internal/provider/provider.go:118-122` (`DataSources()`) — register all five new constructors.

---

### Task 1: `matomo.Client` contexts and environments methods

**Files:**
- Create: `internal/matomo/tagmanager_discovery.go`
- Test: `internal/matomo/tagmanager_discovery_test.go`

**Interfaces:**
- Consumes: `(c *Client) call(ctx context.Context, method string, params url.Values, out interface{}) error` (`internal/matomo/client.go:54`) — pass `nil` for `params` since both methods take no arguments (`call` treats `nil` as `url.Values{}` and fills in `module`/`method`/`format`/`token_auth` itself).
- Produces: `type Context struct { ID, Name string }`, `type Environment struct { ID, Name string }`, `func (c *Client) GetAvailableContexts(ctx context.Context) ([]Context, error)`, `func (c *Client) GetAvailableEnvironments(ctx context.Context) ([]Environment, error)` — Task 3/4 data sources call these directly.

- [ ] **Step 1: Write the failing tests**

```go
// internal/matomo/tagmanager_discovery_test.go
package matomo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetAvailableContexts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]string{
			{"id": "web", "name": "Web"},
			{"id": "amp", "name": "AMP"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", srv.Client())
	contexts, err := client.GetAvailableContexts(context.Background())
	if err != nil {
		t.Fatalf("GetAvailableContexts() error = %v", err)
	}
	if len(contexts) != 2 {
		t.Fatalf("len(contexts) = %d, want 2", len(contexts))
	}
	if contexts[0] != (Context{ID: "web", Name: "Web"}) {
		t.Errorf("contexts[0] = %+v, want {web Web}", contexts[0])
	}
	if contexts[1] != (Context{ID: "amp", Name: "AMP"}) {
		t.Errorf("contexts[1] = %+v, want {amp AMP}", contexts[1])
	}
}

func TestGetAvailableEnvironments(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]string{
			{"id": "live", "name": "Live"},
			{"id": "dev", "name": "Dev"},
			{"id": "staging", "name": "Staging"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", srv.Client())
	environments, err := client.GetAvailableEnvironments(context.Background())
	if err != nil {
		t.Fatalf("GetAvailableEnvironments() error = %v", err)
	}
	if len(environments) != 3 {
		t.Fatalf("len(environments) = %d, want 3", len(environments))
	}
	if environments[0] != (Environment{ID: "live", Name: "Live"}) {
		t.Errorf("environments[0] = %+v, want {live Live}", environments[0])
	}
}

func TestGetAvailableContexts_empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]string{})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", srv.Client())
	contexts, err := client.GetAvailableContexts(context.Background())
	if err != nil {
		t.Fatalf("GetAvailableContexts() error = %v", err)
	}
	if len(contexts) != 0 {
		t.Fatalf("len(contexts) = %d, want 0", len(contexts))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/matomo/... -run TestGetAvailableContexts -v`
Expected: FAIL — `undefined: Context` / `undefined: NewClient(...).GetAvailableContexts` (the type and method don't exist yet).

- [ ] **Step 3: Write the implementation**

```go
// internal/matomo/tagmanager_discovery.go
package matomo

import "context"

// Context describes one Tag Manager context (e.g. "web", "amp", "mobile"),
// as returned by TagManager.getAvailableContexts. Only contexts with at
// least one available tag type are included (confirmed against
// matomo-org/tag-manager's API.php: getAvailableContexts() filters out
// any context whose getAvailableTagTypesInContext() call returns empty).
type Context struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GetAvailableContexts returns every context this Matomo instance
// supports.
func (c *Client) GetAvailableContexts(ctx context.Context) ([]Context, error) {
	var contexts []Context
	if err := c.call(ctx, "TagManager.getAvailableContexts", nil, &contexts); err != nil {
		return nil, err
	}
	return contexts, nil
}

// Environment describes one Tag Manager publish environment (e.g.
// "live", "dev", "staging"), as returned by
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

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/matomo/... -run 'TestGetAvailableContexts|TestGetAvailableEnvironments' -v`
Expected: PASS (all 3 tests: `TestGetAvailableContexts`, `TestGetAvailableEnvironments`, `TestGetAvailableContexts_empty`).

- [ ] **Step 5: Full package check and commit**

Run: `go build ./... && go vet ./... && gofmt -l internal/matomo/tagmanager_discovery.go internal/matomo/tagmanager_discovery_test.go`
Expected: no output from any command (clean build, clean vet, no formatting diffs).

```bash
git add internal/matomo/tagmanager_discovery.go internal/matomo/tagmanager_discovery_test.go
git commit -m "Add GetAvailableContexts and GetAvailableEnvironments client methods"
```

---

### Task 2: `matomo_tagmanager_contexts` and `matomo_tagmanager_environments` data sources

**Files:**
- Create: `internal/provider/data_source_tagmanager_contexts.go`
- Create: `internal/provider/data_source_tagmanager_contexts_acc_test.go`
- Create: `internal/provider/data_source_tagmanager_environments.go`
- Create: `internal/provider/data_source_tagmanager_environments_acc_test.go`
- Modify: `internal/provider/provider.go:118-122`

**Interfaces:**
- Consumes: `matomo.Client.GetAvailableContexts(ctx) ([]matomo.Context, error)`, `matomo.Client.GetAvailableEnvironments(ctx) ([]matomo.Environment, error)` (Task 1). `datasource.DataSource`/`datasource.DataSourceWithConfigure` interfaces, `resp.State.Set`/`req.Config.Get` pattern from `internal/provider/data_source_site.go:37-121`. `testAccPreCheck(t)`/`testAccProtoV6ProviderFactories` from `internal/provider/acc_test_helpers.go`.
- Produces: `func NewTagManagerContextsDataSource() datasource.DataSource`, `func NewTagManagerEnvironmentsDataSource() datasource.DataSource` — Task 5 registers both in `provider.go`.

- [ ] **Step 1: Write `matomo_tagmanager_contexts`**

```go
// internal/provider/data_source_tagmanager_contexts.go
package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

var (
	_ datasource.DataSource              = &tagManagerContextsDataSource{}
	_ datasource.DataSourceWithConfigure = &tagManagerContextsDataSource{}
)

func NewTagManagerContextsDataSource() datasource.DataSource {
	return &tagManagerContextsDataSource{}
}

type tagManagerContextsDataSource struct {
	client *matomo.Client
}

type tagManagerContextModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

type tagManagerContextsDataSourceModel struct {
	ID       types.String              `tfsdk:"id"`
	Contexts []tagManagerContextModel  `tfsdk:"contexts"`
}

func (d *tagManagerContextsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tagmanager_contexts"
}

func (d *tagManagerContextsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists every Tag Manager context (e.g. \"web\", \"amp\") this Matomo instance supports.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Synthetic identifier for this data source, always \"contexts\".",
			},
			"contexts": schema.ListNestedAttribute{
				Computed:    true,
				Description: "Every available context.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":   schema.StringAttribute{Computed: true, Description: "The context's id, e.g. \"web\"."},
						"name": schema.StringAttribute{Computed: true, Description: "The context's display name."},
					},
				},
			},
		},
	}
}

func (d *tagManagerContextsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*matomo.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", "expected *matomo.Client")
		return
	}
	d.client = client
}

func (d *tagManagerContextsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	contexts, err := d.client.GetAvailableContexts(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing Matomo Tag Manager contexts", err.Error())
		return
	}

	state := tagManagerContextsDataSourceModel{
		ID:       types.StringValue("contexts"),
		Contexts: make([]tagManagerContextModel, len(contexts)),
	}
	for i, c := range contexts {
		state.Contexts[i] = tagManagerContextModel{ID: types.StringValue(c.ID), Name: types.StringValue(c.Name)}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
```

- [ ] **Step 2: Write its acceptance test**

```go
// internal/provider/data_source_tagmanager_contexts_acc_test.go
package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTagManagerContextsDataSource(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

data "matomo_tagmanager_contexts" "all" {}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.matomo_tagmanager_contexts.all", "id", "contexts"),
					resource.TestCheckResourceAttr("data.matomo_tagmanager_contexts.all", "contexts.0.id", "web"),
					resource.TestCheckResourceAttr("data.matomo_tagmanager_contexts.all", "contexts.0.name", "Web"),
				),
			},
		},
	})
}
```

- [ ] **Step 3: Write `matomo_tagmanager_environments`**

```go
// internal/provider/data_source_tagmanager_environments.go
package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

var (
	_ datasource.DataSource              = &tagManagerEnvironmentsDataSource{}
	_ datasource.DataSourceWithConfigure = &tagManagerEnvironmentsDataSource{}
)

func NewTagManagerEnvironmentsDataSource() datasource.DataSource {
	return &tagManagerEnvironmentsDataSource{}
}

type tagManagerEnvironmentsDataSource struct {
	client *matomo.Client
}

type tagManagerEnvironmentModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

type tagManagerEnvironmentsDataSourceModel struct {
	ID           types.String                 `tfsdk:"id"`
	Environments []tagManagerEnvironmentModel `tfsdk:"environments"`
}

func (d *tagManagerEnvironmentsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tagmanager_environments"
}

func (d *tagManagerEnvironmentsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists every Tag Manager publish environment (e.g. \"live\", \"dev\", \"staging\") configured on this Matomo instance.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Synthetic identifier for this data source, always \"environments\".",
			},
			"environments": schema.ListNestedAttribute{
				Computed:    true,
				Description: "Every configured environment.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":   schema.StringAttribute{Computed: true, Description: "The environment's id, e.g. \"live\"."},
						"name": schema.StringAttribute{Computed: true, Description: "The environment's display name."},
					},
				},
			},
		},
	}
}

func (d *tagManagerEnvironmentsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*matomo.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", "expected *matomo.Client")
		return
	}
	d.client = client
}

func (d *tagManagerEnvironmentsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	environments, err := d.client.GetAvailableEnvironments(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing Matomo Tag Manager environments", err.Error())
		return
	}

	state := tagManagerEnvironmentsDataSourceModel{
		ID:           types.StringValue("environments"),
		Environments: make([]tagManagerEnvironmentModel, len(environments)),
	}
	for i, e := range environments {
		state.Environments[i] = tagManagerEnvironmentModel{ID: types.StringValue(e.ID), Name: types.StringValue(e.Name)}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
```

- [ ] **Step 4: Write its acceptance test**

```go
// internal/provider/data_source_tagmanager_environments_acc_test.go
package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTagManagerEnvironmentsDataSource(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

data "matomo_tagmanager_environments" "all" {}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.matomo_tagmanager_environments.all", "id", "environments"),
					resource.TestCheckResourceAttr("data.matomo_tagmanager_environments.all", "environments.0.id", "live"),
					resource.TestCheckResourceAttr("data.matomo_tagmanager_environments.all", "environments.0.name", "Live"),
				),
			},
		},
	})
}
```

- [ ] **Step 5: Register both in the provider (temporary, so `go build` succeeds for this task's verification)**

Edit `internal/provider/provider.go:118-122`:

```go
func (p *MatomoProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewSiteDataSource,
		NewTagManagerContextsDataSource,
		NewTagManagerEnvironmentsDataSource,
	}
}
```

- [ ] **Step 6: Verify build and run unit-level checks**

Run: `go build ./... && go vet ./... && gofmt -l internal/provider/data_source_tagmanager_contexts.go internal/provider/data_source_tagmanager_contexts_acc_test.go internal/provider/data_source_tagmanager_environments.go internal/provider/data_source_tagmanager_environments_acc_test.go internal/provider/provider.go`
Expected: no output (clean build/vet/format). Acceptance tests are NOT run here — they require `TF_ACC=1` and a live Matomo instance, which per this repo's existing convention (see `docs/superpowers/plans/2026-07-01-matomo-acceptance-testing.md`) run in CI's `acceptance.yml` workflow, not locally in this sandbox. If a live Matomo instance is reachable, run `TF_ACC=1 go test ./internal/provider/... -run 'TestAccTagManagerContextsDataSource|TestAccTagManagerEnvironmentsDataSource' -v` and expect PASS; otherwise skip and rely on CI.

- [ ] **Step 7: Commit**

```bash
git add internal/provider/data_source_tagmanager_contexts.go internal/provider/data_source_tagmanager_contexts_acc_test.go internal/provider/data_source_tagmanager_environments.go internal/provider/data_source_tagmanager_environments_acc_test.go internal/provider/provider.go
git commit -m "Add matomo_tagmanager_contexts and matomo_tagmanager_environments data sources"
```

---

### Task 3: `matomo_tagmanager_tag_types` data source

**Files:**
- Create: `internal/provider/data_source_tagmanager_tag_types.go`
- Create: `internal/provider/data_source_tagmanager_tag_types_acc_test.go`
- Modify: `internal/provider/provider.go` (`DataSources()`)

**Interfaces:**
- Consumes: `matomo.Client.GetAvailableTagTypes(ctx context.Context, idContext string) ([]matomo.Template, error)` (`internal/matomo/tagmanager_templates.go:60-62`, already exists). `matomo.Template` fields: `ID, Name, Description, Category string` plus `Parameters []TemplateParam` (not used here — spec section 4.3 excludes it).
- Produces: `func NewTagManagerTagTypesDataSource() datasource.DataSource` — Task 5 registers it in `provider.go`.

- [ ] **Step 1: Write the data source**

```go
// internal/provider/data_source_tagmanager_tag_types.go
package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

var (
	_ datasource.DataSource              = &tagManagerTagTypesDataSource{}
	_ datasource.DataSourceWithConfigure = &tagManagerTagTypesDataSource{}
)

func NewTagManagerTagTypesDataSource() datasource.DataSource {
	return &tagManagerTagTypesDataSource{}
}

type tagManagerTagTypesDataSource struct {
	client *matomo.Client
}

type tagManagerTypeSummaryModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Category    types.String `tfsdk:"category"`
}

type tagManagerTagTypesDataSourceModel struct {
	ID       types.String                  `tfsdk:"id"`
	Context  types.String                  `tfsdk:"context"`
	TagTypes []tagManagerTypeSummaryModel  `tfsdk:"tag_types"`
}

func (d *tagManagerTagTypesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tagmanager_tag_types"
}

func tagManagerTypeSummarySchema(description string) schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		Computed:    true,
		Description: description,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"id":          schema.StringAttribute{Computed: true, Description: "The type's id, e.g. \"CustomHtml\"."},
				"name":        schema.StringAttribute{Computed: true, Description: "The type's display name."},
				"description": schema.StringAttribute{Computed: true, Description: "The type's description."},
				"category":    schema.StringAttribute{Computed: true, Description: "The category this type is grouped under in Matomo's UI."},
			},
		},
	}
}

func (d *tagManagerTagTypesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists every tag type available in a given Tag Manager context, including third-party-plugin-contributed ones.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Set to the context value this data source was queried with.",
			},
			"context": schema.StringAttribute{
				Required:    true,
				Description: "The Tag Manager context to query, e.g. \"web\". See the matomo_tagmanager_contexts data source for valid values.",
			},
			"tag_types": tagManagerTypeSummarySchema("Every tag type available in the given context."),
		},
	}
}

func (d *tagManagerTagTypesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*matomo.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", "expected *matomo.Client")
		return
	}
	d.client = client
}

func (d *tagManagerTagTypesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config tagManagerTagTypesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	templates, err := d.client.GetAvailableTagTypes(ctx, config.Context.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error listing Matomo Tag Manager tag types", err.Error())
		return
	}

	config.ID = config.Context
	config.TagTypes = make([]tagManagerTypeSummaryModel, len(templates))
	for i, t := range templates {
		config.TagTypes[i] = tagManagerTypeSummaryModel{
			ID:          types.StringValue(t.ID),
			Name:        types.StringValue(t.Name),
			Description: types.StringValue(t.Description),
			Category:    types.StringValue(t.Category),
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
```

- [ ] **Step 2: Write its acceptance test**

```go
// internal/provider/data_source_tagmanager_tag_types_acc_test.go
package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTagManagerTagTypesDataSource(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

data "matomo_tagmanager_tag_types" "web" {
  context = "web"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.matomo_tagmanager_tag_types.web", "id", "web"),
					resource.TestCheckResourceAttrSet("data.matomo_tagmanager_tag_types.web", "tag_types.#"),
					resource.TestCheckTypeSetElemNestedAttrs("data.matomo_tagmanager_tag_types.web", "tag_types.*", map[string]string{
						"id": "CustomHtml",
					}),
				),
			},
		},
	})
}
```

- [ ] **Step 3: Register it in the provider**

Edit `internal/provider/provider.go:118-124`:

```go
func (p *MatomoProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewSiteDataSource,
		NewTagManagerContextsDataSource,
		NewTagManagerEnvironmentsDataSource,
		NewTagManagerTagTypesDataSource,
	}
}
```

- [ ] **Step 4: Verify build**

Run: `go build ./... && go vet ./... && gofmt -l internal/provider/data_source_tagmanager_tag_types.go internal/provider/data_source_tagmanager_tag_types_acc_test.go internal/provider/provider.go`
Expected: no output. As in Task 2, run the acceptance test with `TF_ACC=1` only if a live Matomo instance is reachable; otherwise it runs in CI.

- [ ] **Step 5: Commit**

```bash
git add internal/provider/data_source_tagmanager_tag_types.go internal/provider/data_source_tagmanager_tag_types_acc_test.go internal/provider/provider.go
git commit -m "Add matomo_tagmanager_tag_types data source"
```

---

### Task 4: `matomo_tagmanager_trigger_types` and `matomo_tagmanager_variable_types` data sources

**Files:**
- Create: `internal/provider/data_source_tagmanager_trigger_types.go`
- Create: `internal/provider/data_source_tagmanager_trigger_types_acc_test.go`
- Create: `internal/provider/data_source_tagmanager_variable_types.go`
- Create: `internal/provider/data_source_tagmanager_variable_types_acc_test.go`
- Modify: `internal/provider/provider.go` (`DataSources()`)

**Interfaces:**
- Consumes: `matomo.Client.GetAvailableTriggerTypes(ctx, idContext) ([]matomo.Template, error)`, `matomo.Client.GetAvailableVariableTypes(ctx, idContext) ([]matomo.Template, error)` (both already exist, `internal/matomo/tagmanager_templates.go:66-76`). `tagManagerTypeSummaryModel` and `tagManagerTypeSummarySchema(description string) schema.ListNestedAttribute` (Task 3, same package `provider` — reused directly, not redefined).
- Produces: `func NewTagManagerTriggerTypesDataSource() datasource.DataSource`, `func NewTagManagerVariableTypesDataSource() datasource.DataSource` — Task 5 registers both.

- [ ] **Step 1: Write `matomo_tagmanager_trigger_types`**

```go
// internal/provider/data_source_tagmanager_trigger_types.go
package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

var (
	_ datasource.DataSource              = &tagManagerTriggerTypesDataSource{}
	_ datasource.DataSourceWithConfigure = &tagManagerTriggerTypesDataSource{}
)

func NewTagManagerTriggerTypesDataSource() datasource.DataSource {
	return &tagManagerTriggerTypesDataSource{}
}

type tagManagerTriggerTypesDataSource struct {
	client *matomo.Client
}

type tagManagerTriggerTypesDataSourceModel struct {
	ID           types.String                 `tfsdk:"id"`
	Context      types.String                 `tfsdk:"context"`
	TriggerTypes []tagManagerTypeSummaryModel `tfsdk:"trigger_types"`
}

func (d *tagManagerTriggerTypesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tagmanager_trigger_types"
}

func (d *tagManagerTriggerTypesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists every trigger type available in a given Tag Manager context, including third-party-plugin-contributed ones.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Set to the context value this data source was queried with.",
			},
			"context": schema.StringAttribute{
				Required:    true,
				Description: "The Tag Manager context to query, e.g. \"web\". See the matomo_tagmanager_contexts data source for valid values.",
			},
			"trigger_types": tagManagerTypeSummarySchema("Every trigger type available in the given context."),
		},
	}
}

func (d *tagManagerTriggerTypesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*matomo.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", "expected *matomo.Client")
		return
	}
	d.client = client
}

func (d *tagManagerTriggerTypesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config tagManagerTriggerTypesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	templates, err := d.client.GetAvailableTriggerTypes(ctx, config.Context.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error listing Matomo Tag Manager trigger types", err.Error())
		return
	}

	config.ID = config.Context
	config.TriggerTypes = make([]tagManagerTypeSummaryModel, len(templates))
	for i, t := range templates {
		config.TriggerTypes[i] = tagManagerTypeSummaryModel{
			ID:          types.StringValue(t.ID),
			Name:        types.StringValue(t.Name),
			Description: types.StringValue(t.Description),
			Category:    types.StringValue(t.Category),
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
```

- [ ] **Step 2: Write its acceptance test**

```go
// internal/provider/data_source_tagmanager_trigger_types_acc_test.go
package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTagManagerTriggerTypesDataSource(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

data "matomo_tagmanager_trigger_types" "web" {
  context = "web"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.matomo_tagmanager_trigger_types.web", "id", "web"),
					resource.TestCheckResourceAttrSet("data.matomo_tagmanager_trigger_types.web", "trigger_types.#"),
					resource.TestCheckTypeSetElemNestedAttrs("data.matomo_tagmanager_trigger_types.web", "trigger_types.*", map[string]string{
						"id": "PageView",
					}),
				),
			},
		},
	})
}
```

- [ ] **Step 3: Write `matomo_tagmanager_variable_types`**

```go
// internal/provider/data_source_tagmanager_variable_types.go
package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

var (
	_ datasource.DataSource              = &tagManagerVariableTypesDataSource{}
	_ datasource.DataSourceWithConfigure = &tagManagerVariableTypesDataSource{}
)

func NewTagManagerVariableTypesDataSource() datasource.DataSource {
	return &tagManagerVariableTypesDataSource{}
}

type tagManagerVariableTypesDataSource struct {
	client *matomo.Client
}

type tagManagerVariableTypesDataSourceModel struct {
	ID            types.String                 `tfsdk:"id"`
	Context       types.String                 `tfsdk:"context"`
	VariableTypes []tagManagerTypeSummaryModel `tfsdk:"variable_types"`
}

func (d *tagManagerVariableTypesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tagmanager_variable_types"
}

func (d *tagManagerVariableTypesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists every variable type available in a given Tag Manager context (excluding Matomo's ~70 pre-configured built-in variables, which Matomo itself filters out of this API response), including third-party-plugin-contributed ones.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Set to the context value this data source was queried with.",
			},
			"context": schema.StringAttribute{
				Required:    true,
				Description: "The Tag Manager context to query, e.g. \"web\". See the matomo_tagmanager_contexts data source for valid values.",
			},
			"variable_types": tagManagerTypeSummarySchema("Every variable type available in the given context."),
		},
	}
}

func (d *tagManagerVariableTypesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*matomo.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", "expected *matomo.Client")
		return
	}
	d.client = client
}

func (d *tagManagerVariableTypesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config tagManagerVariableTypesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	templates, err := d.client.GetAvailableVariableTypes(ctx, config.Context.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error listing Matomo Tag Manager variable types", err.Error())
		return
	}

	config.ID = config.Context
	config.VariableTypes = make([]tagManagerTypeSummaryModel, len(templates))
	for i, t := range templates {
		config.VariableTypes[i] = tagManagerTypeSummaryModel{
			ID:          types.StringValue(t.ID),
			Name:        types.StringValue(t.Name),
			Description: types.StringValue(t.Description),
			Category:    types.StringValue(t.Category),
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
```

- [ ] **Step 4: Write its acceptance test**

```go
// internal/provider/data_source_tagmanager_variable_types_acc_test.go
package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTagManagerVariableTypesDataSource(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

data "matomo_tagmanager_variable_types" "web" {
  context = "web"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.matomo_tagmanager_variable_types.web", "id", "web"),
					resource.TestCheckResourceAttrSet("data.matomo_tagmanager_variable_types.web", "variable_types.#"),
					resource.TestCheckTypeSetElemNestedAttrs("data.matomo_tagmanager_variable_types.web", "variable_types.*", map[string]string{
						"id": "Constant",
					}),
				),
			},
		},
	})
}
```

- [ ] **Step 5: Register both in the provider (final state)**

Edit `internal/provider/provider.go:118-127`:

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

- [ ] **Step 6: Verify build**

Run: `go build ./... && go vet ./... && gofmt -l internal/provider/data_source_tagmanager_trigger_types.go internal/provider/data_source_tagmanager_trigger_types_acc_test.go internal/provider/data_source_tagmanager_variable_types.go internal/provider/data_source_tagmanager_variable_types_acc_test.go internal/provider/provider.go`
Expected: no output. Run acceptance tests with `TF_ACC=1` only if a live Matomo instance is reachable; otherwise CI covers it.

- [ ] **Step 7: Commit**

```bash
git add internal/provider/data_source_tagmanager_trigger_types.go internal/provider/data_source_tagmanager_trigger_types_acc_test.go internal/provider/data_source_tagmanager_variable_types.go internal/provider/data_source_tagmanager_variable_types_acc_test.go internal/provider/provider.go
git commit -m "Add matomo_tagmanager_trigger_types and matomo_tagmanager_variable_types data sources"
```

---

### Task 5: Full verification pass

**Files:** None created or modified — this task only runs the project's full verification suite across everything Tasks 1-4 added, matching the pattern of every prior phase's final task (e.g. Task 19 "Full verification pass" in `docs/superpowers/plans/2026-06-30-matomo-provider-foundation.md`).

**Interfaces:**
- Consumes: everything from Tasks 1-4.
- Produces: nothing new — this is a checkpoint, not a deliverable.

- [ ] **Step 1: Run the full unit test suite**

Run: `go test ./... -v -count=1`
Expected: PASS across all packages, including the 3 new tests in `internal/matomo/tagmanager_discovery_test.go` and every pre-existing test. No `TestAcc*` tests run here since `TF_ACC` is unset — the framework skips them via `testAccPreCheck(t)`.

- [ ] **Step 2: Run `go vet` and `gofmt` across the whole repo**

Run: `go vet ./... && gofmt -l .`
Expected: `go vet` produces no output; `gofmt -l .` produces no output (no unformatted files).

- [ ] **Step 3: Run the linter**

Run: `golangci-lint run ./...`
Expected: no findings. If findings appear in the 5 new data source files or `tagmanager_discovery.go`/`tagmanager_discovery_test.go`, fix them before proceeding (common ones for this pattern: unused imports if a field was trimmed, or `gofmt`/`goimports` import-grouping order — run `make fmt` and re-check).

- [ ] **Step 4: Confirm all 5 new data sources are registered**

Run: `grep -n "NewTagManager.*DataSource" internal/provider/provider.go`
Expected: exactly 5 lines matching `NewTagManagerContextsDataSource`, `NewTagManagerEnvironmentsDataSource`, `NewTagManagerTagTypesDataSource`, `NewTagManagerTriggerTypesDataSource`, `NewTagManagerVariableTypesDataSource`.

- [ ] **Step 5: If a live Matomo instance is reachable, run the new acceptance tests together**

Run: `TF_ACC=1 go test ./internal/provider/... -run 'TestAccTagManagerContextsDataSource|TestAccTagManagerEnvironmentsDataSource|TestAccTagManagerTagTypesDataSource|TestAccTagManagerTriggerTypesDataSource|TestAccTagManagerVariableTypesDataSource' -v`
Expected: PASS for all 5. If no live Matomo instance is reachable in this environment (expected in most sandboxes per this repo's established convention — see `docs/superpowers/plans/2026-07-01-matomo-acceptance-testing.md`), skip this step; the pushed branch's `acceptance.yml` CI workflow will run them against the docker-compose fixture.

- [ ] **Step 6: Commit if step 3 required fixes (otherwise nothing to commit — Task 4 already committed the final state)**

```bash
git add -A
git commit -m "Fix lint findings in Tag Manager discovery data sources"
```
