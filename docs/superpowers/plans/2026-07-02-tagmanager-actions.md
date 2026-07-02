# Tag Manager Actions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add four Terraform actions (`matomo_tagmanager_create_container_version`, `matomo_tagmanager_publish_container_version`, `matomo_tagmanager_enable_preview_mode`, `matomo_tagmanager_disable_preview_mode`) wrapping Matomo Tag Manager's version/publish/preview lifecycle operations.

**Architecture:** Four new `internal/matomo` client methods on top of the existing `Client`/`call` machinery; four new `internal/provider/action_tagmanager_*.go` files, each implementing `action.Action` + `action.ActionWithConfigure` (parse `container_id` via the existing `parseContainerID` helper, call the client method(s), report progress via `resp.SendProgress`); the provider gains a `provider.ProviderWithActions` implementation and `Configure()` additionally sets `resp.ActionData`.

**Tech Stack:** Go, `terraform-plugin-framework`'s `action`/`action/schema` packages (already vendored at v1.19.0 — no dependency bump needed), `terraform-plugin-testing` for acceptance tests, `net/http/httptest` for client unit tests.

## Global Constraints

- Matomo API methods used (confirmed against `matomo-org/tag-manager`'s `API.php`): `TagManager.createContainerVersion($idSite, $idContainer, $name, $description='', $idContainerVersion=null)` returns a version id; `TagManager.publishContainerVersion($idSite, $idContainer, $idContainerVersion, $environment)` requires a pre-existing version snapshot (cannot publish the mutable draft directly); `TagManager.enablePreviewMode($idSite, $idContainer, $idContainerVersion=null)` / `disablePreviewMode(...)` both default to the draft when the version is omitted.
- **Hard platform constraint:** this provider's vendored `terraform-plugin-framework` (v1.19.0) has no action-output mechanism — `action.Schema` is config-only, `action.InvokeResponse` has only `Diagnostics`+`SendProgress`. Actions cannot expose values to other actions/resources.
- `matomo_tagmanager_publish_container_version` therefore takes NO version-id input: it auto-snapshots the draft (name `"terraform-release-"+time.Now().UTC().Format(time.RFC3339)`) and immediately publishes that snapshot to the given `environment`, in one `Invoke` call.
- `matomo_tagmanager_enable_preview_mode` / `disable_preview_mode` take only `container_id` — no version field, always target the draft (Matomo's default when `idContainerVersion` is omitted).
- `matomo_tagmanager_create_container_version`'s `name` is Required (Matomo validates 1-50 chars; no provider-side length validator — Matomo's own error is authoritative).
- `environment` on the publish action is a plain required string, no static `stringvalidator.OneOf(...)` — same free-text philosophy as the generic resources' `type` field; point users at the already-shipped `matomo_tagmanager_environments` data source in the field description.
- No `internal/provider`-layer unit tests for the actions (no mock-client pattern exists anywhere in this codebase for resources/data sources/actions).
- No `examples/`/`tfplugindocs` work (Phase 8, out of scope).
- CI's `hashicorp/setup-terraform@v3` step has no `terraform_version` pinned (defaults to latest), which is well past the 1.14 minimum actions require — no CI changes needed for action support.
- Full spec: `docs/superpowers/specs/2026-07-02-tagmanager-actions-design.md`.

---

## File Structure

- Create: `internal/matomo/tagmanager_container_versions.go` — `CreateContainerVersion`, `PublishContainerVersion`, `EnablePreviewMode`, `DisablePreviewMode` client methods.
- Create: `internal/matomo/tagmanager_container_versions_test.go` — unit tests for all four.
- Create: `internal/provider/action_tagmanager_create_container_version.go` — the create-version action.
- Create: `internal/provider/action_tagmanager_create_container_version_acc_test.go` — its acceptance test.
- Create: `internal/provider/action_tagmanager_publish_container_version.go` — the publish action.
- Create: `internal/provider/action_tagmanager_publish_container_version_acc_test.go` — its acceptance test.
- Create: `internal/provider/action_tagmanager_enable_preview_mode.go` — the enable-preview action.
- Create: `internal/provider/action_tagmanager_disable_preview_mode.go` — the disable-preview action (same file structure as enable, separate file per the one-action-per-file convention).
- Create: `internal/provider/action_tagmanager_preview_mode_acc_test.go` — acceptance tests for both preview-mode actions together (they're trivial and share a container setup).
- Modify: `internal/provider/provider.go` — add `Actions()` method, `resp.ActionData = client` in `Configure()`.

---

### Task 1: `matomo.Client` container-version and preview-mode methods

**Files:**
- Create: `internal/matomo/tagmanager_container_versions.go`
- Test: `internal/matomo/tagmanager_container_versions_test.go`

**Interfaces:**
- Consumes: `(c *Client) call(ctx context.Context, method string, params url.Values, out interface{}) error` (`internal/matomo/client.go:54`).
- Produces: `func (c *Client) CreateContainerVersion(ctx context.Context, idSite int, idContainer, name, description string) (int, error)`, `func (c *Client) PublishContainerVersion(ctx context.Context, idSite int, idContainer string, idContainerVersion int, environment string) error`, `func (c *Client) EnablePreviewMode(ctx context.Context, idSite int, idContainer string) error`, `func (c *Client) DisablePreviewMode(ctx context.Context, idSite int, idContainer string) error` — Tasks 2/3/4 call these directly.

- [ ] **Step 1: Write the failing tests**

```go
// internal/matomo/tagmanager_container_versions_test.go
package matomo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestCreateContainerVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		if got := r.Form.Get("method"); got != "TagManager.createContainerVersion" {
			t.Errorf("method = %q, want TagManager.createContainerVersion", got)
		}
		if got := r.Form.Get("idSite"); got != "1" {
			t.Errorf("idSite = %q, want 1", got)
		}
		if got := r.Form.Get("idContainer"); got != "abc123" {
			t.Errorf("idContainer = %q, want abc123", got)
		}
		if got := r.Form.Get("name"); got != "release-1" {
			t.Errorf("name = %q, want release-1", got)
		}
		if got := r.Form.Get("description"); got != "a description" {
			t.Errorf("description = %q, want %q", got, "a description")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int{"value": 42})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", srv.Client())
	id, err := client.CreateContainerVersion(context.Background(), 1, "abc123", "release-1", "a description")
	if err != nil {
		t.Fatalf("CreateContainerVersion() error = %v", err)
	}
	if id != 42 {
		t.Errorf("CreateContainerVersion() = %d, want 42", id)
	}
}

func TestPublishContainerVersion(t *testing.T) {
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		gotForm = r.Form
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]string{{"idcontainerversion": "42"}})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", srv.Client())
	err := client.PublishContainerVersion(context.Background(), 1, "abc123", 42, "live")
	if err != nil {
		t.Fatalf("PublishContainerVersion() error = %v", err)
	}
	if got := gotForm.Get("method"); got != "TagManager.publishContainerVersion" {
		t.Errorf("method = %q, want TagManager.publishContainerVersion", got)
	}
	if got := gotForm.Get("idContainerVersion"); got != "42" {
		t.Errorf("idContainerVersion = %q, want 42", got)
	}
	if got := gotForm.Get("environment"); got != "live" {
		t.Errorf("environment = %q, want live", got)
	}
}

func TestEnablePreviewMode(t *testing.T) {
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		gotForm = r.Form
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", srv.Client())
	if err := client.EnablePreviewMode(context.Background(), 1, "abc123"); err != nil {
		t.Fatalf("EnablePreviewMode() error = %v", err)
	}
	if got := gotForm.Get("method"); got != "TagManager.enablePreviewMode" {
		t.Errorf("method = %q, want TagManager.enablePreviewMode", got)
	}
	if gotForm.Has("idContainerVersion") {
		t.Errorf("idContainerVersion should not be set, got %q", gotForm.Get("idContainerVersion"))
	}
}

func TestDisablePreviewMode(t *testing.T) {
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		gotForm = r.Form
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", srv.Client())
	if err := client.DisablePreviewMode(context.Background(), 1, "abc123"); err != nil {
		t.Fatalf("DisablePreviewMode() error = %v", err)
	}
	if got := gotForm.Get("method"); got != "TagManager.disablePreviewMode" {
		t.Errorf("method = %q, want TagManager.disablePreviewMode", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/matomo/... -run 'TestCreateContainerVersion|TestPublishContainerVersion|TestEnablePreviewMode|TestDisablePreviewMode' -v`
Expected: FAIL — `undefined: (*Client).CreateContainerVersion` etc. (none of the four methods exist yet).

- [ ] **Step 3: Write the implementation**

```go
// internal/matomo/tagmanager_container_versions.go
package matomo

import (
	"context"
	"net/url"
	"strconv"
)

// CreateContainerVersion snapshots a container's current draft into a
// new, named, immutable version and returns its id. name must be 1-50
// characters (a Matomo-side constraint - not re-validated here).
func (c *Client) CreateContainerVersion(ctx context.Context, idSite int, idContainer, name, description string) (int, error) {
	v := url.Values{
		"idSite":      {strconv.Itoa(idSite)},
		"idContainer": {idContainer},
		"name":        {name},
		"description": {description},
	}
	var out struct {
		Value int `json:"value"`
	}
	if err := c.call(ctx, "TagManager.createContainerVersion", v, &out); err != nil {
		return 0, err
	}
	return out.Value, nil
}

// PublishContainerVersion publishes an existing container version
// snapshot to the given environment. idContainerVersion must refer to a
// version already created via CreateContainerVersion - Matomo does not
// allow publishing the mutable draft directly.
func (c *Client) PublishContainerVersion(ctx context.Context, idSite int, idContainer string, idContainerVersion int, environment string) error {
	v := url.Values{
		"idSite":             {strconv.Itoa(idSite)},
		"idContainer":        {idContainer},
		"idContainerVersion": {strconv.Itoa(idContainerVersion)},
		"environment":        {environment},
	}
	return c.call(ctx, "TagManager.publishContainerVersion", v, nil)
}

// EnablePreviewMode turns on preview mode for a container's current
// draft version.
func (c *Client) EnablePreviewMode(ctx context.Context, idSite int, idContainer string) error {
	v := url.Values{
		"idSite":      {strconv.Itoa(idSite)},
		"idContainer": {idContainer},
	}
	return c.call(ctx, "TagManager.enablePreviewMode", v, nil)
}

// DisablePreviewMode turns off preview mode for a container's current
// draft version.
func (c *Client) DisablePreviewMode(ctx context.Context, idSite int, idContainer string) error {
	v := url.Values{
		"idSite":      {strconv.Itoa(idSite)},
		"idContainer": {idContainer},
	}
	return c.call(ctx, "TagManager.disablePreviewMode", v, nil)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/matomo/... -run 'TestCreateContainerVersion|TestPublishContainerVersion|TestEnablePreviewMode|TestDisablePreviewMode' -v`
Expected: PASS (all 4 tests).

- [ ] **Step 5: Full package check and commit**

Run: `go build ./... && go vet ./... && gofmt -l internal/matomo/tagmanager_container_versions.go internal/matomo/tagmanager_container_versions_test.go`
Expected: no output from any command.

```bash
git add internal/matomo/tagmanager_container_versions.go internal/matomo/tagmanager_container_versions_test.go
git commit -m "Add CreateContainerVersion, PublishContainerVersion, EnablePreviewMode, DisablePreviewMode client methods"
```

---

### Task 2: Provider action wiring + `matomo_tagmanager_create_container_version`

**Files:**
- Modify: `internal/provider/provider.go`
- Create: `internal/provider/action_tagmanager_create_container_version.go`
- Create: `internal/provider/action_tagmanager_create_container_version_acc_test.go`

**Interfaces:**
- Consumes: `matomo.Client.CreateContainerVersion(ctx, idSite, idContainer, name, description) (int, error)` (Task 1). `parseContainerID(id string) (siteID int, idContainer string, err error)` (`internal/provider/ids.go:27`). `action.Action`/`action.ActionWithConfigure` interfaces, `action/schema.Schema`/`StringAttribute`. `testAccPreCheck(t)`/`testAccProtoV6ProviderFactories`/`testAccMatomoClient(t)` from `internal/provider/acc_test_helpers.go`.
- Produces: `func NewCreateContainerVersionAction() action.Action`, and the provider's `Actions()` method + `ActionData` wiring — Tasks 3/4 add their own actions to the same `Actions()` slice and reuse the same `Configure` pattern.

- [ ] **Step 1: Wire the provider for actions**

Edit `internal/provider/provider.go`. Add the `action` import:

```go
import (
	"context"
	"crypto/tls"
	"net/http"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

var (
	_ provider.Provider           = &MatomoProvider{}
	_ provider.ProviderWithActions = &MatomoProvider{}
)
```

In `Configure()`, right after `resp.DataSourceData = client` (currently the last line before the closing brace, at `internal/provider/provider.go:103`), add:

```go
	client := matomo.NewClient(baseURL, apiToken, httpClient)
	resp.ResourceData = client
	resp.DataSourceData = client
	resp.ActionData = client
```

Add the new `Actions()` method after the existing `DataSources()` method:

```go
func (p *MatomoProvider) Actions(_ context.Context) []func() action.Action {
	return []func() action.Action{
		NewCreateContainerVersionAction,
	}
}
```

(Tasks 3 and 4 will append their own constructors to this same slice.)

- [ ] **Step 2: Write the create-container-version action**

```go
// internal/provider/action_tagmanager_create_container_version.go
package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

var (
	_ action.Action              = &createContainerVersionAction{}
	_ action.ActionWithConfigure = &createContainerVersionAction{}
)

func NewCreateContainerVersionAction() action.Action {
	return &createContainerVersionAction{}
}

type createContainerVersionAction struct {
	client *matomo.Client
}

type createContainerVersionActionModel struct {
	ContainerID types.String `tfsdk:"container_id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
}

func (a *createContainerVersionAction) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tagmanager_create_container_version"
}

func (a *createContainerVersionAction) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Snapshots a Tag Manager container's current draft into a new, named, immutable version. Does not publish it - see matomo_tagmanager_publish_container_version for that.",
		Attributes: map[string]schema.Attribute{
			"container_id": schema.StringAttribute{
				Required:    true,
				Description: "The container's id (matomo_tagmanager_container.x.id).",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The new version's name. Matomo requires 1-50 characters.",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "The new version's description.",
			},
		},
	}
}

func (a *createContainerVersionAction) Configure(_ context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*matomo.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", "expected *matomo.Client")
		return
	}
	a.client = client
}

func (a *createContainerVersionAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var config createContainerVersionActionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, err := parseContainerID(config.ContainerID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid container_id", err.Error())
		return
	}

	description := config.Description.ValueString()

	resp.SendProgress(action.InvokeProgressEvent{Message: "Creating container version..."})
	versionID, err := a.client.CreateContainerVersion(ctx, siteID, idContainer, config.Name.ValueString(), description)
	if err != nil {
		resp.Diagnostics.AddError("Error creating Matomo Tag Manager container version", err.Error())
		return
	}
	resp.SendProgress(action.InvokeProgressEvent{Message: "Created container version."})
	_ = versionID // the created version's id cannot be exposed as an action output (see design spec §2) - it is only ever visible via the progress messages above.
}
```

- [ ] **Step 3: Write its acceptance test**

```go
// internal/provider/action_tagmanager_create_container_version_acc_test.go
package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCreateContainerVersionAction(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Generated CreateContainerVersion Acceptance Site"
  urls = ["https://acc-create-container-version.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Generated CreateContainerVersion Acceptance Container"
}

action "matomo_tagmanager_create_container_version" "release" {
  config {
    container_id = matomo_tagmanager_container.test.id
    name         = "acceptance-test-version"
    description  = "created by an acceptance test"
  }
}

resource "terraform_data" "trigger" {
  input = "trigger"
  lifecycle {
    action_trigger {
      events  = [after_create]
      actions = [action.matomo_tagmanager_create_container_version.release]
    }
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("matomo_tagmanager_container.test", "id"),
				),
			},
		},
	})
}
```

- [ ] **Step 4: Verify build**

Run: `go build ./... && go vet ./... && gofmt -l internal/provider/provider.go internal/provider/action_tagmanager_create_container_version.go internal/provider/action_tagmanager_create_container_version_acc_test.go`
Expected: no output.

Run: `go test ./internal/provider/... -v -count=1`
Expected: all PASS/SKIP as appropriate; `TestAccCreateContainerVersionAction` SKIPs (no `TF_ACC`/live Matomo in this sandbox).

If a live Matomo instance IS reachable, run `TF_ACC=1 go test ./internal/provider/... -run TestAccCreateContainerVersionAction -v`. If `terraform_data`'s `action_trigger` lifecycle block syntax errors against the installed Terraform CLI version (this is a newer, still-stabilizing HCL construct — the exact block name/shape may differ slightly from what's shown above), that is expected per the spec's §2 caveat that action invocation syntax may still be in flux; adjust the config to whatever syntax the installed `terraform version` actually accepts before treating it as a real test failure. Note this in your task report either way.

- [ ] **Step 5: Commit**

```bash
git add internal/provider/provider.go internal/provider/action_tagmanager_create_container_version.go internal/provider/action_tagmanager_create_container_version_acc_test.go
git commit -m "Add matomo_tagmanager_create_container_version action"
```

---

### Task 3: `matomo_tagmanager_publish_container_version`

**Files:**
- Create: `internal/provider/action_tagmanager_publish_container_version.go`
- Create: `internal/provider/action_tagmanager_publish_container_version_acc_test.go`
- Modify: `internal/provider/provider.go` (`Actions()` only)

**Interfaces:**
- Consumes: `matomo.Client.CreateContainerVersion(ctx, idSite, idContainer, name, description) (int, error)` and `matomo.Client.PublishContainerVersion(ctx, idSite, idContainer string, idContainerVersion int, environment string) error` (both Task 1). `parseContainerID` (`internal/provider/ids.go:27`).
- Produces: `func NewPublishContainerVersionAction() action.Action` — Task 4 appends its own two constructors to the same `Actions()` slice this task extends.

- [ ] **Step 1: Write the publish action**

```go
// internal/provider/action_tagmanager_publish_container_version.go
package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

var (
	_ action.Action              = &publishContainerVersionAction{}
	_ action.ActionWithConfigure = &publishContainerVersionAction{}
)

func NewPublishContainerVersionAction() action.Action {
	return &publishContainerVersionAction{}
}

type publishContainerVersionAction struct {
	client *matomo.Client
}

type publishContainerVersionActionModel struct {
	ContainerID types.String `tfsdk:"container_id"`
	Environment types.String `tfsdk:"environment"`
}

func (a *publishContainerVersionAction) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tagmanager_publish_container_version"
}

func (a *publishContainerVersionAction) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Publishes a Tag Manager container's current draft to an environment. Internally snapshots the draft into a new version, then publishes that snapshot in the same invocation - there is no separate version input, this action always publishes whatever the draft currently holds.",
		Attributes: map[string]schema.Attribute{
			"container_id": schema.StringAttribute{
				Required:    true,
				Description: "The container's id (matomo_tagmanager_container.x.id).",
			},
			"environment": schema.StringAttribute{
				Required:    true,
				Description: "The environment to publish to, e.g. \"live\". See the matomo_tagmanager_environments data source for valid values on a given instance.",
			},
		},
	}
}

func (a *publishContainerVersionAction) Configure(_ context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*matomo.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", "expected *matomo.Client")
		return
	}
	a.client = client
}

func (a *publishContainerVersionAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var config publishContainerVersionActionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, err := parseContainerID(config.ContainerID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid container_id", err.Error())
		return
	}

	resp.SendProgress(action.InvokeProgressEvent{Message: "Snapshotting current draft..."})
	versionName := "terraform-release-" + time.Now().UTC().Format(time.RFC3339)
	versionID, err := a.client.CreateContainerVersion(ctx, siteID, idContainer, versionName, "")
	if err != nil {
		resp.Diagnostics.AddError("Error snapshotting Matomo Tag Manager container draft", err.Error())
		return
	}

	environment := config.Environment.ValueString()
	resp.SendProgress(action.InvokeProgressEvent{Message: fmt.Sprintf("Publishing version %d to environment %q...", versionID, environment)})
	if err := a.client.PublishContainerVersion(ctx, siteID, idContainer, versionID, environment); err != nil {
		resp.Diagnostics.AddError("Error publishing Matomo Tag Manager container version", err.Error())
		return
	}
	resp.SendProgress(action.InvokeProgressEvent{Message: "Published."})
}
```

- [ ] **Step 2: Write its acceptance test**

```go
// internal/provider/action_tagmanager_publish_container_version_acc_test.go
package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPublishContainerVersionAction(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Generated PublishContainerVersion Acceptance Site"
  urls = ["https://acc-publish-container-version.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Generated PublishContainerVersion Acceptance Container"
}

action "matomo_tagmanager_publish_container_version" "go_live" {
  config {
    container_id = matomo_tagmanager_container.test.id
    environment  = "live"
  }
}

resource "terraform_data" "trigger" {
  input = "trigger"
  lifecycle {
    action_trigger {
      events  = [after_create]
      actions = [action.matomo_tagmanager_publish_container_version.go_live]
    }
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("matomo_tagmanager_container.test", "id"),
				),
			},
		},
	})
}
```

- [ ] **Step 3: Register it in the provider**

Edit `internal/provider/provider.go`'s `Actions()`:

```go
func (p *MatomoProvider) Actions(_ context.Context) []func() action.Action {
	return []func() action.Action{
		NewCreateContainerVersionAction,
		NewPublishContainerVersionAction,
	}
}
```

- [ ] **Step 4: Verify build**

Run: `go build ./... && go vet ./... && gofmt -l internal/provider/action_tagmanager_publish_container_version.go internal/provider/action_tagmanager_publish_container_version_acc_test.go internal/provider/provider.go`
Expected: no output.

Run: `go test ./internal/provider/... -v -count=1`
Expected: all PASS/SKIP as appropriate; `TestAccPublishContainerVersionAction` SKIPs without live Matomo. As in Task 2, if the `action_trigger` HCL syntax needs adjusting for the installed Terraform CLI version when a live instance IS available, that's expected per the spec's caveat — note it in your report.

- [ ] **Step 5: Commit**

```bash
git add internal/provider/action_tagmanager_publish_container_version.go internal/provider/action_tagmanager_publish_container_version_acc_test.go internal/provider/provider.go
git commit -m "Add matomo_tagmanager_publish_container_version action"
```

---

### Task 4: `matomo_tagmanager_enable_preview_mode` and `matomo_tagmanager_disable_preview_mode`

**Files:**
- Create: `internal/provider/action_tagmanager_enable_preview_mode.go`
- Create: `internal/provider/action_tagmanager_disable_preview_mode.go`
- Create: `internal/provider/action_tagmanager_preview_mode_acc_test.go`
- Modify: `internal/provider/provider.go` (`Actions()`, final state)

**Interfaces:**
- Consumes: `matomo.Client.EnablePreviewMode(ctx, idSite, idContainer) error` and `matomo.Client.DisablePreviewMode(ctx, idSite, idContainer) error` (both Task 1). `parseContainerID` (`internal/provider/ids.go:27`).
- Produces: `func NewEnablePreviewModeAction() action.Action`, `func NewDisablePreviewModeAction() action.Action` — both appended to `Actions()`, the plan's final state.

- [ ] **Step 1: Write the enable-preview-mode action**

```go
// internal/provider/action_tagmanager_enable_preview_mode.go
package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

var (
	_ action.Action              = &enablePreviewModeAction{}
	_ action.ActionWithConfigure = &enablePreviewModeAction{}
)

func NewEnablePreviewModeAction() action.Action {
	return &enablePreviewModeAction{}
}

type enablePreviewModeAction struct {
	client *matomo.Client
}

type previewModeActionModel struct {
	ContainerID types.String `tfsdk:"container_id"`
}

func (a *enablePreviewModeAction) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tagmanager_enable_preview_mode"
}

func (a *enablePreviewModeAction) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Enables preview mode for a Tag Manager container's current draft version.",
		Attributes: map[string]schema.Attribute{
			"container_id": schema.StringAttribute{
				Required:    true,
				Description: "The container's id (matomo_tagmanager_container.x.id).",
			},
		},
	}
}

func (a *enablePreviewModeAction) Configure(_ context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*matomo.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", "expected *matomo.Client")
		return
	}
	a.client = client
}

func (a *enablePreviewModeAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var config previewModeActionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, err := parseContainerID(config.ContainerID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid container_id", err.Error())
		return
	}

	resp.SendProgress(action.InvokeProgressEvent{Message: "Enabling preview mode..."})
	if err := a.client.EnablePreviewMode(ctx, siteID, idContainer); err != nil {
		resp.Diagnostics.AddError("Error enabling Matomo Tag Manager preview mode", err.Error())
		return
	}
	resp.SendProgress(action.InvokeProgressEvent{Message: "Preview mode enabled."})
}
```

- [ ] **Step 2: Write the disable-preview-mode action**

```go
// internal/provider/action_tagmanager_disable_preview_mode.go
package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

var (
	_ action.Action              = &disablePreviewModeAction{}
	_ action.ActionWithConfigure = &disablePreviewModeAction{}
)

func NewDisablePreviewModeAction() action.Action {
	return &disablePreviewModeAction{}
}

type disablePreviewModeAction struct {
	client *matomo.Client
}

func (a *disablePreviewModeAction) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tagmanager_disable_preview_mode"
}

func (a *disablePreviewModeAction) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Disables preview mode for a Tag Manager container's current draft version.",
		Attributes: map[string]schema.Attribute{
			"container_id": schema.StringAttribute{
				Required:    true,
				Description: "The container's id (matomo_tagmanager_container.x.id).",
			},
		},
	}
}

func (a *disablePreviewModeAction) Configure(_ context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*matomo.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", "expected *matomo.Client")
		return
	}
	a.client = client
}

func (a *disablePreviewModeAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var config previewModeActionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, err := parseContainerID(config.ContainerID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid container_id", err.Error())
		return
	}

	resp.SendProgress(action.InvokeProgressEvent{Message: "Disabling preview mode..."})
	if err := a.client.DisablePreviewMode(ctx, siteID, idContainer); err != nil {
		resp.Diagnostics.AddError("Error disabling Matomo Tag Manager preview mode", err.Error())
		return
	}
	resp.SendProgress(action.InvokeProgressEvent{Message: "Preview mode disabled."})
}
```

Note: `disablePreviewModeAction.Invoke` reuses the `previewModeActionModel` struct defined in `action_tagmanager_enable_preview_mode.go` (same package, same shape: just `ContainerID`) — do not redeclare it in this file.

- [ ] **Step 3: Write the shared acceptance test file**

```go
// internal/provider/action_tagmanager_preview_mode_acc_test.go
package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccEnablePreviewModeAction(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Generated EnablePreviewMode Acceptance Site"
  urls = ["https://acc-enable-preview-mode.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Generated EnablePreviewMode Acceptance Container"
}

action "matomo_tagmanager_enable_preview_mode" "preview" {
  config {
    container_id = matomo_tagmanager_container.test.id
  }
}

resource "terraform_data" "trigger" {
  input = "trigger"
  lifecycle {
    action_trigger {
      events  = [after_create]
      actions = [action.matomo_tagmanager_enable_preview_mode.preview]
    }
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("matomo_tagmanager_container.test", "id"),
				),
			},
		},
	})
}

func TestAccDisablePreviewModeAction(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Generated DisablePreviewMode Acceptance Site"
  urls = ["https://acc-disable-preview-mode.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Generated DisablePreviewMode Acceptance Container"
}

action "matomo_tagmanager_disable_preview_mode" "preview" {
  config {
    container_id = matomo_tagmanager_container.test.id
  }
}

resource "terraform_data" "trigger" {
  input = "trigger"
  lifecycle {
    action_trigger {
      events  = [after_create]
      actions = [action.matomo_tagmanager_disable_preview_mode.preview]
    }
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("matomo_tagmanager_container.test", "id"),
				),
			},
		},
	})
}
```

- [ ] **Step 4: Register both in the provider (final state)**

Edit `internal/provider/provider.go`'s `Actions()`:

```go
func (p *MatomoProvider) Actions(_ context.Context) []func() action.Action {
	return []func() action.Action{
		NewCreateContainerVersionAction,
		NewPublishContainerVersionAction,
		NewEnablePreviewModeAction,
		NewDisablePreviewModeAction,
	}
}
```

- [ ] **Step 5: Verify build**

Run: `go build ./... && go vet ./... && gofmt -l internal/provider/action_tagmanager_enable_preview_mode.go internal/provider/action_tagmanager_disable_preview_mode.go internal/provider/action_tagmanager_preview_mode_acc_test.go internal/provider/provider.go`
Expected: no output.

Run: `go test ./internal/provider/... -v -count=1`
Expected: all PASS/SKIP as appropriate; both new `TestAcc*` SKIP without live Matomo.

- [ ] **Step 6: Commit**

```bash
git add internal/provider/action_tagmanager_enable_preview_mode.go internal/provider/action_tagmanager_disable_preview_mode.go internal/provider/action_tagmanager_preview_mode_acc_test.go internal/provider/provider.go
git commit -m "Add matomo_tagmanager_enable_preview_mode and matomo_tagmanager_disable_preview_mode actions"
```

---

### Task 5: Full verification pass

**Files:** None created or modified — this task only runs the project's full verification suite across everything Tasks 1-4 added.

**Interfaces:**
- Consumes: everything from Tasks 1-4.
- Produces: nothing new — this is a checkpoint, not a deliverable.

- [ ] **Step 1: Run the full unit test suite**

Run: `go test ./... -v -count=1`
Expected: PASS across all packages, including the 4 new tests in `internal/matomo/tagmanager_container_versions_test.go` and every pre-existing test. No `TestAcc*` tests run since `TF_ACC` is unset.

- [ ] **Step 2: Run `go vet` and `gofmt` across the whole repo**

Run: `go vet ./... && gofmt -l .`
Expected: both produce no output.

- [ ] **Step 3: Run the linter**

Run: `golangci-lint run ./...`
Expected: no findings. If findings appear (common for this pattern: an unused `versionID` variable if the underscore-discard in Task 2's create action was dropped, or import-order nits), fix them before proceeding.

- [ ] **Step 4: Confirm all 4 new actions are registered**

Run: `grep -n "New.*Action" internal/provider/provider.go`
Expected: exactly 4 lines matching `NewCreateContainerVersionAction`, `NewPublishContainerVersionAction`, `NewEnablePreviewModeAction`, `NewDisablePreviewModeAction`.

- [ ] **Step 5: Confirm the provider implements ProviderWithActions**

Run: `grep -n "ProviderWithActions\|ActionData" internal/provider/provider.go`
Expected: one line asserting `_ provider.ProviderWithActions = &MatomoProvider{}`, one line setting `resp.ActionData = client`.

- [ ] **Step 6: If a live Matomo instance is reachable, run the new acceptance tests together**

Run: `TF_ACC=1 go test ./internal/provider/... -run 'TestAccCreateContainerVersionAction|TestAccPublishContainerVersionAction|TestAccEnablePreviewModeAction|TestAccDisablePreviewModeAction' -v`
Expected: PASS for all 4. Per Task 2/3's notes, the `action_trigger`/`after_create` HCL syntax may need adjusting to whatever the installed Terraform CLI version actually accepts — this is expected given the spec's §2 caveat about the actions API still stabilizing, not a code defect. If no live Matomo instance is reachable (expected in most sandboxes, per this repo's established convention), skip this step; the pushed branch's `acceptance.yml` CI workflow will run them against the docker-compose fixture.

- [ ] **Step 7: Commit if step 3 or step 6 required fixes (otherwise nothing to commit)**

```bash
git add -A
git commit -m "Fix lint findings / acceptance test HCL syntax in Tag Manager actions"
```
