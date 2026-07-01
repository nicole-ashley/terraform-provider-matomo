# Matomo Provider Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a working, releasable `terraform-provider-matomo` covering spec phases 1-4: the Matomo API client (Sites, Custom Dimensions, Tag Manager containers/tags/triggers/variables), and the resources `matomo_site`, `matomo_custom_dimension`, `matomo_tagmanager_container`, and the generic `matomo_tagmanager_tag`/`_trigger`/`_variable`.

**Architecture:** `internal/matomo` is a typed HTTP client over Matomo's `module=API` endpoint, one file per API module/responsibility, unit-tested against `httptest` fixtures. `internal/provider` holds the `terraform-plugin-framework` provider and resources, each backed by the client, also unit-tested via `httptest` (no live Matomo). Composite resource IDs (site → container/dimension → tag/trigger/variable) are built/parsed by shared helpers in `internal/provider/ids.go`.

**Tech Stack:** Go, `github.com/hashicorp/terraform-plugin-framework`, `github.com/hashicorp/terraform-plugin-testing`, standard library `net/http`/`net/http/httptest`.

## Global Constraints

- Spec: `docs/superpowers/specs/2026-06-30-matomo-tagmanager-provider-design.md` — every task below implements a section of it; constraints here are copied verbatim from it.
- Registry namespace: `nicole-ashley/matomo`. Module path: `github.com/nicole-ashley/terraform-provider-matomo`.
- Target Matomo version: latest stable core + Tag Manager plugin only.
- No `map[string]interface{}` in client method signatures — typed request/response structs only.
- Composite IDs: `matomo_site` id = `"{site_id}"`; `matomo_custom_dimension`/`matomo_tagmanager_container` id = `"{site_id}/{index_or_container_id}"`; `matomo_tagmanager_{tag,trigger,variable}` id = `"{site_id}/{container_id}/{entity_id}"`. Import ID == resource `.id` exactly.
- `matomo_tagmanager_{tag,trigger,variable}` always target the container's draft version; no version ID is ever exposed on these resources.
- `matomo_custom_dimension.index`/`.scope` are `RequiresReplace`; Delete sets `active = false` (Matomo has no delete API for dimensions).
- Provider config: `base_url` (or `MATOMO_BASE_URL`), `api_token` (sensitive, or `MATOMO_API_TOKEN`), optional `insecure_skip_verify`. No default `site_id`.
- Every resource/data source/client method gets unit test coverage for success and Matomo-error-envelope paths; no live Matomo required for any test in this plan.
- Out of scope for this plan (later plans): typed per-type tag/trigger/variable resources + codegen, `matomo_tagmanager_*` data sources, actions (create/publish version, preview mode), docs generation, release pipeline.

---

## File Structure

```
go.mod
main.go
internal/
  matomo/
    client.go                 # Client, doRequest, APIError
    client_test.go
    sites.go                  # AddSite, UpdateSite, DeleteSite, GetSiteFromID, GetAllSites
    sites_test.go
    customdimensions.go       # ConfigureNewCustomDimension, ConfigureExistingCustomDimension, GetConfiguredCustomDimensions
    customdimensions_test.go
    tagmanager_containers.go  # AddContainer, UpdateContainer, DeleteContainer, GetContainer, GetContainers
    tagmanager_containers_test.go
    tagmanager_versions.go    # GetContainerVersions (draft resolution only, in this plan)
    tagmanager_versions_test.go
    tagmanager_tags.go        # AddContainerTag, UpdateContainerTag, DeleteContainerTag, GetContainerTag, Pause/ResumeContainerTag
    tagmanager_tags_test.go
    tagmanager_triggers.go    # AddContainerTrigger, UpdateContainerTrigger, DeleteContainerTrigger, GetContainerTrigger
    tagmanager_triggers_test.go
    tagmanager_variables.go   # AddContainerVariable, UpdateContainerVariable, DeleteContainerVariable, GetContainerVariable
    tagmanager_variables_test.go
  provider/
    provider.go                       # Provider struct, Schema, Configure, Resources(), DataSources()
    provider_test.go
    ids.go                            # composite ID build/parse helpers
    ids_test.go
    resource_site.go
    resource_site_test.go
    data_source_site.go
    data_source_site_test.go
    resource_custom_dimension.go
    resource_custom_dimension_test.go
    resource_tagmanager_container.go
    resource_tagmanager_container_test.go
    draft_version.go                  # resolveDraftVersionID(ctx, client, siteID, containerID)
    draft_version_test.go
    resource_tagmanager_tag.go
    resource_tagmanager_tag_test.go
    resource_tagmanager_trigger.go
    resource_tagmanager_trigger_test.go
    resource_tagmanager_variable.go
    resource_tagmanager_variable_test.go
.golangci.yml
GNUmakefile
.github/workflows/ci.yml
```

---

### Task 1: Repo skeleton — go.mod, main.go, empty provider, CI

**Files:**
- Create: `go.mod`
- Create: `main.go`
- Create: `internal/provider/provider.go`
- Create: `internal/provider/provider_test.go`
- Create: `GNUmakefile`
- Create: `.golangci.yml`
- Create: `.github/workflows/ci.yml`

**Interfaces:**
- Produces: `provider.New() func() provider.Provider` — factory used by `main.go` and by every later resource/data-source test via `providerserver.NewProtocol6WithError`.

- [ ] **Step 1: Initialize the Go module**

Run:
```bash
go mod init github.com/nicole-ashley/terraform-provider-matomo
go get github.com/hashicorp/terraform-plugin-framework@latest
go get github.com/hashicorp/terraform-plugin-framework-validators@latest
go get github.com/hashicorp/terraform-plugin-go@latest
go get github.com/hashicorp/terraform-plugin-testing@latest
```
Expected: `go.mod` and `go.sum` created with no errors.

- [ ] **Step 2: Write the empty provider**

`internal/provider/provider.go`:
```go
package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var _ provider.Provider = &MatomoProvider{}

type MatomoProvider struct {
	version string
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &MatomoProvider{version: version}
	}
}

func (p *MatomoProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "matomo"
	resp.Version = p.version
}

func (p *MatomoProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"base_url": schema.StringAttribute{
				Optional:    true,
				Description: "Base URL of the Matomo instance, e.g. https://analytics.example.com. May also be set via the MATOMO_BASE_URL environment variable.",
			},
			"api_token": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Matomo API token (token_auth). May also be set via the MATOMO_API_TOKEN environment variable.",
			},
			"insecure_skip_verify": schema.BoolAttribute{
				Optional:    true,
				Description: "Skip TLS certificate verification. Only use for self-hosted instances with internal CAs.",
			},
		},
	}
}

func (p *MatomoProvider) Configure(_ context.Context, _ provider.ConfigureRequest, _ *provider.ConfigureResponse) {
}

func (p *MatomoProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{}
}

func (p *MatomoProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}
```

This is intentionally a no-op shell — `Configure` is filled in during Task 2 once `matomo.Client` exists, and `Resources`/`DataSources` get one entry added per later task.

- [ ] **Step 3: Write a smoke test for the provider schema**

`internal/provider/provider_test.go`:
```go
package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
)

func TestMatomoProvider_Metadata(t *testing.T) {
	p := New("test")()
	resp := &provider.MetadataResponse{}
	p.Metadata(nil, provider.MetadataRequest{}, resp)

	if resp.TypeName != "matomo" {
		t.Errorf("TypeName = %q, want %q", resp.TypeName, "matomo")
	}
}

func TestMatomoProvider_Schema(t *testing.T) {
	p := New("test")()
	resp := &provider.SchemaResponse{}
	p.Schema(nil, provider.SchemaRequest{}, resp)

	for _, attr := range []string{"base_url", "api_token", "insecure_skip_verify"} {
		if _, ok := resp.Schema.Attributes[attr]; !ok {
			t.Errorf("schema missing attribute %q", attr)
		}
	}
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/provider/... -run TestMatomoProvider -v`
Expected: both tests `PASS`.

- [ ] **Step 5: Write main.go**

```go
package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/provider"
)

var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "run the provider with support for debuggers")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/nicole-ashley/matomo",
		Debug:   debug,
	}

	if err := providerserver.Serve(context.Background(), provider.New(version), opts); err != nil {
		log.Fatal(err.Error())
	}
}
```

- [ ] **Step 6: Build the provider binary**

Run: `go build -o /dev/null .`
Expected: exits 0, no output.

- [ ] **Step 7: Add the Makefile**

`GNUmakefile`:
```makefile
default: test

.PHONY: build
build:
	go build -o /dev/null .

.PHONY: test
test:
	go test ./... -v -count=1

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: fmt
fmt:
	gofmt -s -w .
	goimports -w .
```

- [ ] **Step 8: Add golangci-lint config**

`.golangci.yml`:
```yaml
linters:
  enable:
    - errcheck
    - govet
    - staticcheck
    - gofmt
    - goimports
    - ineffassign
    - unused
run:
  timeout: 5m
```

- [ ] **Step 9: Add CI workflow**

`.github/workflows/ci.yml`:
```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: go build -o /dev/null .
      - run: go test ./... -v -count=1

  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - uses: golangci/golangci-lint-action@v6
        with:
          version: latest
```

- [ ] **Step 10: Commit**

```bash
git add go.mod go.sum main.go internal/provider/provider.go internal/provider/provider_test.go GNUmakefile .golangci.yml .github/workflows/ci.yml
git commit -m "feat: scaffold provider, CI, and build tooling"
```

---

### Task 2: `matomo.Client` core — request building, auth, error envelope

**Files:**
- Create: `internal/matomo/client.go`
- Test: `internal/matomo/client_test.go`

**Interfaces:**
- Consumes: nothing (first client file).
- Produces:
  - `type Client struct { ... }`
  - `func NewClient(baseURL, apiToken string, httpClient *http.Client) *Client`
  - `func (c *Client) call(ctx context.Context, method string, params url.Values, out interface{}) error` — unexported, used by every other `internal/matomo/*.go` file in later tasks.
  - `type APIError struct { Message string }` with `func (e *APIError) Error() string`.

- [ ] **Step 1: Write the failing test for a successful call**

`internal/matomo/client_test.go`:
```go
package matomo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestClient_call_success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "SitesManager.getSiteFromId" {
			t.Errorf("method = %q, want SitesManager.getSiteFromId", got)
		}
		if got := r.URL.Query().Get("token_auth"); got != "test-token" {
			t.Errorf("token_auth = %q, want test-token", got)
		}
		if got := r.URL.Query().Get("format"); got != "JSON" {
			t.Errorf("format = %q, want JSON", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"idsite": 3, "name": "Example"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())

	var out struct {
		IDSite int    `json:"idsite"`
		Name   string `json:"name"`
	}
	err := c.call(context.Background(), "SitesManager.getSiteFromId", url.Values{"idSite": {"3"}}, &out)
	if err != nil {
		t.Fatalf("call() error = %v", err)
	}
	if out.IDSite != 3 || out.Name != "Example" {
		t.Errorf("out = %+v, want IDSite=3 Name=Example", out)
	}
}

func TestClient_call_apiError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"result": "error", "message": "Website id Not found"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())

	var out map[string]any
	err := c.call(context.Background(), "SitesManager.getSiteFromId", url.Values{"idSite": {"999"}}, &out)
	if err == nil {
		t.Fatal("call() error = nil, want APIError")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("err type = %T, want *APIError", err)
	}
	if apiErr.Message != "Website id Not found" {
		t.Errorf("apiErr.Message = %q, want %q", apiErr.Message, "Website id Not found")
	}
	if apiErr.Error() != "Website id Not found" {
		t.Errorf("apiErr.Error() = %q, want %q", apiErr.Error(), "Website id Not found")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/matomo/... -v`
Expected: FAIL — `package matomo` has no `NewClient`/`APIError` (compile error).

- [ ] **Step 3: Write the client**

`internal/matomo/client.go`:
```go
package matomo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Client is a thin, typed wrapper over the Matomo HTTP API
// (module=API&method=...&format=JSON). It is not a generic RPC proxy:
// each Matomo API method gets its own typed Go method elsewhere in this
// package, built on top of call.
type Client struct {
	baseURL    string
	apiToken   string
	httpClient *http.Client
}

// NewClient creates a Matomo API client. baseURL is the Matomo instance's
// root URL (e.g. "https://analytics.example.com"), without a trailing
// slash or "/index.php" suffix.
func NewClient(baseURL, apiToken string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiToken:   apiToken,
		httpClient: httpClient,
	}
}

// APIError represents Matomo's {"result":"error","message":"..."} envelope.
type APIError struct {
	Message string
}

func (e *APIError) Error() string {
	return e.Message
}

type errorEnvelope struct {
	Result  string `json:"result"`
	Message string `json:"message"`
}

// call invokes a Matomo API method and decodes the JSON response into out.
// params must not set "module", "method", "format", or "token_auth" — call
// sets those itself.
func (c *Client) call(ctx context.Context, method string, params url.Values, out interface{}) error {
	if params == nil {
		params = url.Values{}
	}
	params.Set("module", "API")
	params.Set("method", method)
	params.Set("format", "JSON")
	params.Set("token_auth", c.apiToken)

	reqURL := fmt.Sprintf("%s/index.php?%s", c.baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("matomo: building request for %s: %w", method, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("matomo: calling %s: %w", method, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("matomo: reading response for %s: %w", method, err)
	}

	var env errorEnvelope
	if err := json.Unmarshal(body, &env); err == nil && env.Result == "error" {
		return &APIError{Message: env.Message}
	}

	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("matomo: decoding response for %s: %w", method, err)
	}
	return nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/matomo/... -v`
Expected: `TestClient_call_success` and `TestClient_call_apiError` both `PASS`.

- [ ] **Step 5: Commit**

```bash
git add internal/matomo/client.go internal/matomo/client_test.go
git commit -m "feat(matomo): add API client core with error envelope handling"
```

---

### Task 3: Sites client methods

**Files:**
- Create: `internal/matomo/sites.go`
- Test: `internal/matomo/sites_test.go`

**Interfaces:**
- Consumes: `Client.call` (Task 2).
- Produces:
  - `type Site struct { IDSite int; Name string; Timezone string; Currency string; URLs []string; Ecommerce bool; ExcludedIPs []string; ExcludeUnknownUrls bool; Type string; Group string }`
  - `type AddSiteParams struct { Name string; URLs []string; Timezone, Currency, Group, Type *string; Ecommerce, ExcludeUnknownUrls *bool; ExcludedIPs []string }`
  - `type UpdateSiteParams struct { AddSiteParams }`
  - `func (c *Client) AddSite(ctx context.Context, p AddSiteParams) (int, error)`
  - `func (c *Client) UpdateSite(ctx context.Context, idSite int, p UpdateSiteParams) error`
  - `func (c *Client) DeleteSite(ctx context.Context, idSite int) error`
  - `func (c *Client) GetSiteFromID(ctx context.Context, idSite int) (*Site, error)`
  - `func (c *Client) GetAllSites(ctx context.Context) ([]Site, error)`

- [ ] **Step 1: Write the failing tests**

`internal/matomo/sites_test.go`:
```go
package matomo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_AddSite(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "SitesManager.addSite" {
			t.Errorf("method = %q, want SitesManager.addSite", got)
		}
		if got := r.URL.Query().Get("siteName"); got != "Example" {
			t.Errorf("siteName = %q, want Example", got)
		}
		if got := r.URL.Query()["urls[]"]; len(got) != 1 || got[0] != "https://example.com" {
			t.Errorf("urls[] = %v, want [https://example.com]", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"value": "3"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	id, err := c.AddSite(context.Background(), AddSiteParams{
		Name: "Example",
		URLs: []string{"https://example.com"},
	})
	if err != nil {
		t.Fatalf("AddSite() error = %v", err)
	}
	if id != 3 {
		t.Errorf("AddSite() id = %d, want 3", id)
	}
}

func TestClient_AddSite_apiError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"result": "error", "message": "You can't add a website"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	_, err := c.AddSite(context.Background(), AddSiteParams{Name: "Example"})
	if _, ok := err.(*APIError); !ok {
		t.Fatalf("AddSite() error type = %T, want *APIError", err)
	}
}

func TestClient_UpdateSite(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "SitesManager.updateSite" {
			t.Errorf("method = %q, want SitesManager.updateSite", got)
		}
		if got := r.URL.Query().Get("idSite"); got != "3" {
			t.Errorf("idSite = %q, want 3", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	err := c.UpdateSite(context.Background(), 3, UpdateSiteParams{AddSiteParams{Name: "Renamed"}})
	if err != nil {
		t.Fatalf("UpdateSite() error = %v", err)
	}
}

func TestClient_DeleteSite(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "SitesManager.deleteSite" {
			t.Errorf("method = %q, want SitesManager.deleteSite", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	if err := c.DeleteSite(context.Background(), 3); err != nil {
		t.Fatalf("DeleteSite() error = %v", err)
	}
}

func TestClient_GetSiteFromID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "SitesManager.getSiteFromId" {
			t.Errorf("method = %q, want SitesManager.getSiteFromId", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"idsite": "3", "name": "Example", "timezone": "UTC", "currency": "USD",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	site, err := c.GetSiteFromID(context.Background(), 3)
	if err != nil {
		t.Fatalf("GetSiteFromID() error = %v", err)
	}
	if site.IDSite != 3 || site.Name != "Example" {
		t.Errorf("site = %+v, want IDSite=3 Name=Example", site)
	}
}

func TestClient_GetAllSites(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "SitesManager.getAllSites" {
			t.Errorf("method = %q, want SitesManager.getAllSites", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"idsite": "1", "name": "A"},
			{"idsite": "2", "name": "B"},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	sites, err := c.GetAllSites(context.Background())
	if err != nil {
		t.Fatalf("GetAllSites() error = %v", err)
	}
	if len(sites) != 2 {
		t.Fatalf("len(sites) = %d, want 2", len(sites))
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/matomo/... -run TestClient_.*Site -v`
Expected: FAIL — compile error, `sites.go` does not exist yet.

- [ ] **Step 3: Write the implementation**

`internal/matomo/sites.go`:
```go
package matomo

import (
	"context"
	"net/url"
	"strconv"
)

// Site is a Matomo website as returned by SitesManager.getSiteFromId /
// getAllSites.
type Site struct {
	IDSite              int      `json:"idsite,string"`
	Name                string   `json:"name"`
	Timezone            string   `json:"timezone"`
	Currency            string   `json:"currency"`
	URLs                []string `json:"-"`
	Ecommerce           bool     `json:"-"`
	ExcludedIPs         string   `json:"excluded_ips"`
	ExcludeUnknownUrls  bool     `json:"-"`
	Type                string   `json:"type"`
	Group               string   `json:"group"`
}

// AddSiteParams holds the subset of SitesManager.addSite's parameters this
// provider exposes. Pointer fields are optional; nil means "let Matomo use
// its default."
type AddSiteParams struct {
	Name                string
	URLs                []string
	Timezone            *string
	Currency            *string
	Group               *string
	Type                *string
	Ecommerce           *bool
	ExcludeUnknownUrls  *bool
	ExcludedIPs         []string
}

// UpdateSiteParams mirrors AddSiteParams; all fields apply to
// SitesManager.updateSite, where a nil/zero field leaves the existing value
// unchanged.
type UpdateSiteParams struct {
	AddSiteParams
}

func (p AddSiteParams) toValues() url.Values {
	v := url.Values{}
	v.Set("siteName", p.Name)
	for _, u := range p.URLs {
		v.Add("urls[]", u)
	}
	if p.Timezone != nil {
		v.Set("timezone", *p.Timezone)
	}
	if p.Currency != nil {
		v.Set("currency", *p.Currency)
	}
	if p.Group != nil {
		v.Set("group", *p.Group)
	}
	if p.Type != nil {
		v.Set("type", *p.Type)
	}
	if p.Ecommerce != nil {
		v.Set("ecommerce", boolToIntString(*p.Ecommerce))
	}
	if p.ExcludeUnknownUrls != nil {
		v.Set("excludeUnknownUrls", boolToIntString(*p.ExcludeUnknownUrls))
	}
	for _, ip := range p.ExcludedIPs {
		v.Add("excludedIps[]", ip)
	}
	return v
}

func boolToIntString(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// AddSite creates a website and returns its new site ID.
func (c *Client) AddSite(ctx context.Context, p AddSiteParams) (int, error) {
	var out struct {
		Value int `json:"value,string"`
	}
	if err := c.call(ctx, "SitesManager.addSite", p.toValues(), &out); err != nil {
		return 0, err
	}
	return out.Value, nil
}

// UpdateSite modifies an existing website.
func (c *Client) UpdateSite(ctx context.Context, idSite int, p UpdateSiteParams) error {
	v := p.toValues()
	v.Set("idSite", strconv.Itoa(idSite))
	return c.call(ctx, "SitesManager.updateSite", v, nil)
}

// DeleteSite removes a website. Does not delete its logs/archives.
func (c *Client) DeleteSite(ctx context.Context, idSite int) error {
	v := url.Values{"idSite": {strconv.Itoa(idSite)}}
	return c.call(ctx, "SitesManager.deleteSite", v, nil)
}

// GetSiteFromID retrieves a website's details by ID.
func (c *Client) GetSiteFromID(ctx context.Context, idSite int) (*Site, error) {
	v := url.Values{"idSite": {strconv.Itoa(idSite)}}
	var site Site
	if err := c.call(ctx, "SitesManager.getSiteFromId", v, &site); err != nil {
		return nil, err
	}
	return &site, nil
}

// GetAllSites returns every website (requires a superuser token).
func (c *Client) GetAllSites(ctx context.Context) ([]Site, error) {
	var sites []Site
	if err := c.call(ctx, "SitesManager.getAllSites", nil, &sites); err != nil {
		return nil, err
	}
	return sites, nil
}
```

Note: `Site.IDSite` uses the `,string` JSON tag because Matomo's JSON API
returns numeric IDs as quoted strings (e.g. `"idsite": "3"`); the `AddSite`
response's `value` field follows the same convention.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/matomo/... -run TestClient_.*Site -v`
Expected: all six tests `PASS`.

- [ ] **Step 5: Commit**

```bash
git add internal/matomo/sites.go internal/matomo/sites_test.go
git commit -m "feat(matomo): add Sites API client methods"
```

---

### Task 4: Custom Dimensions client methods

**Files:**
- Create: `internal/matomo/customdimensions.go`
- Test: `internal/matomo/customdimensions_test.go`

**Interfaces:**
- Consumes: `Client.call` (Task 2).
- Produces:
  - `type CustomDimension struct { ID int; Name string; Index int; Scope string; Active bool; CaseSensitive bool; Extractions []DimensionExtraction }`
  - `type DimensionExtraction struct { DimensionID int; Pattern string }`
  - `func (c *Client) ConfigureNewCustomDimension(ctx context.Context, idSite int, name, scope string, active bool) (int, error)`
  - `func (c *Client) ConfigureExistingCustomDimension(ctx context.Context, idDimension, idSite int, name string, active bool) error`
  - `func (c *Client) GetConfiguredCustomDimensions(ctx context.Context, idSite int) ([]CustomDimension, error)`

- [ ] **Step 1: Write the failing tests**

`internal/matomo/customdimensions_test.go`:
```go
package matomo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_ConfigureNewCustomDimension(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "CustomDimensions.configureNewCustomDimension" {
			t.Errorf("method = %q, want CustomDimensions.configureNewCustomDimension", got)
		}
		if got := r.URL.Query().Get("scope"); got != "visit" {
			t.Errorf("scope = %q, want visit", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "1"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	id, err := c.ConfigureNewCustomDimension(context.Background(), 3, "Product Category", "visit", true)
	if err != nil {
		t.Fatalf("ConfigureNewCustomDimension() error = %v", err)
	}
	if id != 1 {
		t.Errorf("id = %d, want 1", id)
	}
}

func TestClient_ConfigureExistingCustomDimension(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "CustomDimensions.configureExistingCustomDimension" {
			t.Errorf("method = %q, want CustomDimensions.configureExistingCustomDimension", got)
		}
		if got := r.URL.Query().Get("idDimension"); got != "1" {
			t.Errorf("idDimension = %q, want 1", got)
		}
		if got := r.URL.Query().Get("active"); got != "0" {
			t.Errorf("active = %q, want 0", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	err := c.ConfigureExistingCustomDimension(context.Background(), 1, 3, "Product Category", false)
	if err != nil {
		t.Fatalf("ConfigureExistingCustomDimension() error = %v", err)
	}
}

func TestClient_GetConfiguredCustomDimensions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "CustomDimensions.getConfiguredCustomDimensions" {
			t.Errorf("method = %q, want CustomDimensions.getConfiguredCustomDimensions", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": "1", "name": "Product Category", "index": "1", "scope": "visit", "active": true, "case_sensitive": false},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	dims, err := c.GetConfiguredCustomDimensions(context.Background(), 3)
	if err != nil {
		t.Fatalf("GetConfiguredCustomDimensions() error = %v", err)
	}
	if len(dims) != 1 || dims[0].Index != 1 || dims[0].Scope != "visit" {
		t.Errorf("dims = %+v, want one dimension with Index=1 Scope=visit", dims)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/matomo/... -run TestClient_.*CustomDimension -v`
Expected: FAIL — compile error, `customdimensions.go` does not exist yet.

- [ ] **Step 3: Write the implementation**

`internal/matomo/customdimensions.go`:
```go
package matomo

import (
	"context"
	"net/url"
	"strconv"
)

// DimensionExtraction is one extraction rule for a CustomDimension.
type DimensionExtraction struct {
	DimensionID int    `json:"dimension"`
	Pattern     string `json:"pattern"`
}

// CustomDimension is a Matomo custom dimension as returned by
// CustomDimensions.getConfiguredCustomDimensions. Index is the dimension's
// slot number within its Scope and is what other Matomo API calls
// (including Tag Manager's MatomoConfiguration variable) refer to it by.
type CustomDimension struct {
	ID            int                    `json:"id,string"`
	Name          string                 `json:"name"`
	Index         int                    `json:"index,string"`
	Scope         string                 `json:"scope"`
	Active        bool                   `json:"active"`
	CaseSensitive bool                   `json:"case_sensitive"`
	Extractions   []DimensionExtraction  `json:"extractions"`
}

// ConfigureNewCustomDimension creates a new custom dimension in the next
// available slot for the given scope ("visit" or "action") and returns the
// slot index Matomo assigned it.
func (c *Client) ConfigureNewCustomDimension(ctx context.Context, idSite int, name, scope string, active bool) (int, error) {
	v := url.Values{
		"idSite": {strconv.Itoa(idSite)},
		"name":   {name},
		"scope":  {scope},
		"active": {boolToIntString(active)},
	}
	var out struct {
		ID int `json:"id,string"`
	}
	if err := c.call(ctx, "CustomDimensions.configureNewCustomDimension", v, &out); err != nil {
		return 0, err
	}
	return out.ID, nil
}

// ConfigureExistingCustomDimension updates an already-configured dimension's
// name and active state. Matomo has no API to delete a custom dimension;
// setting active=false is the closest available equivalent.
func (c *Client) ConfigureExistingCustomDimension(ctx context.Context, idDimension, idSite int, name string, active bool) error {
	v := url.Values{
		"idDimension": {strconv.Itoa(idDimension)},
		"idSite":      {strconv.Itoa(idSite)},
		"name":        {name},
		"active":      {boolToIntString(active)},
	}
	return c.call(ctx, "CustomDimensions.configureExistingCustomDimension", v, nil)
}

// GetConfiguredCustomDimensions lists every custom dimension configured for
// a site, across both scopes.
func (c *Client) GetConfiguredCustomDimensions(ctx context.Context, idSite int) ([]CustomDimension, error) {
	v := url.Values{"idSite": {strconv.Itoa(idSite)}}
	var dims []CustomDimension
	if err := c.call(ctx, "CustomDimensions.getConfiguredCustomDimensions", v, &dims); err != nil {
		return nil, err
	}
	return dims, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/matomo/... -run TestClient_.*CustomDimension -v`
Expected: all three tests `PASS`.

- [ ] **Step 5: Commit**

```bash
git add internal/matomo/customdimensions.go internal/matomo/customdimensions_test.go
git commit -m "feat(matomo): add Custom Dimensions API client methods"
```

---

### Task 5: Tag Manager container client methods

**Files:**
- Create: `internal/matomo/tagmanager_containers.go`
- Test: `internal/matomo/tagmanager_containers_test.go`

**Interfaces:**
- Consumes: `Client.call` (Task 2).
- Produces:
  - `type Container struct { IDContainer string; IDSite int; Context string; Name string; Description string }`
  - `func (c *Client) AddContainer(ctx context.Context, idSite int, context, name, description string) (string, error)`
  - `func (c *Client) UpdateContainer(ctx context.Context, idSite int, idContainer, name, description string) error`
  - `func (c *Client) DeleteContainer(ctx context.Context, idSite int, idContainer string) error`
  - `func (c *Client) GetContainer(ctx context.Context, idSite int, idContainer string) (*Container, error)`

- [ ] **Step 1: Write the failing tests**

`internal/matomo/tagmanager_containers_test.go`:
```go
package matomo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_AddContainer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "TagManager.addContainer" {
			t.Errorf("method = %q, want TagManager.addContainer", got)
		}
		if got := r.URL.Query().Get("context"); got != "web" {
			t.Errorf("context = %q, want web", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"idcontainer": "abc123"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	id, err := c.AddContainer(context.Background(), 3, "web", "Main", "")
	if err != nil {
		t.Fatalf("AddContainer() error = %v", err)
	}
	if id != "abc123" {
		t.Errorf("id = %q, want abc123", id)
	}
}

func TestClient_UpdateContainer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "TagManager.updateContainer" {
			t.Errorf("method = %q, want TagManager.updateContainer", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	if err := c.UpdateContainer(context.Background(), 3, "abc123", "Renamed", "desc"); err != nil {
		t.Fatalf("UpdateContainer() error = %v", err)
	}
}

func TestClient_DeleteContainer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "TagManager.deleteContainer" {
			t.Errorf("method = %q, want TagManager.deleteContainer", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	if err := c.DeleteContainer(context.Background(), 3, "abc123"); err != nil {
		t.Fatalf("DeleteContainer() error = %v", err)
	}
}

func TestClient_GetContainer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "TagManager.getContainer" {
			t.Errorf("method = %q, want TagManager.getContainer", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"idcontainer": "abc123", "idsite": "3", "context": "web", "name": "Main", "description": "",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	ct, err := c.GetContainer(context.Background(), 3, "abc123")
	if err != nil {
		t.Fatalf("GetContainer() error = %v", err)
	}
	if ct.IDContainer != "abc123" || ct.Context != "web" {
		t.Errorf("container = %+v, want IDContainer=abc123 Context=web", ct)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/matomo/... -run TestClient_.*Container -v`
Expected: FAIL — compile error, `tagmanager_containers.go` does not exist yet.

- [ ] **Step 3: Write the implementation**

`internal/matomo/tagmanager_containers.go`:
```go
package matomo

import (
	"context"
	"net/url"
	"strconv"
)

// Container is a Matomo Tag Manager container.
type Container struct {
	IDContainer string `json:"idcontainer"`
	IDSite      int    `json:"idsite,string"`
	Context     string `json:"context"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// AddContainer creates a new Tag Manager container and returns its ID.
func (c *Client) AddContainer(ctx context.Context, idSite int, tmContext, name, description string) (string, error) {
	v := url.Values{
		"idSite":      {strconv.Itoa(idSite)},
		"context":     {tmContext},
		"name":        {name},
		"description": {description},
	}
	var out struct {
		IDContainer string `json:"idcontainer"`
	}
	if err := c.call(ctx, "TagManager.addContainer", v, &out); err != nil {
		return "", err
	}
	return out.IDContainer, nil
}

// UpdateContainer updates a container's name and description.
func (c *Client) UpdateContainer(ctx context.Context, idSite int, idContainer, name, description string) error {
	v := url.Values{
		"idSite":      {strconv.Itoa(idSite)},
		"idContainer": {idContainer},
		"name":        {name},
		"description": {description},
	}
	return c.call(ctx, "TagManager.updateContainer", v, nil)
}

// DeleteContainer deletes a container and all its versions and releases.
func (c *Client) DeleteContainer(ctx context.Context, idSite int, idContainer string) error {
	v := url.Values{
		"idSite":      {strconv.Itoa(idSite)},
		"idContainer": {idContainer},
	}
	return c.call(ctx, "TagManager.deleteContainer", v, nil)
}

// GetContainer returns a container's details.
func (c *Client) GetContainer(ctx context.Context, idSite int, idContainer string) (*Container, error) {
	v := url.Values{
		"idSite":      {strconv.Itoa(idSite)},
		"idContainer": {idContainer},
	}
	var ct Container
	if err := c.call(ctx, "TagManager.getContainer", v, &ct); err != nil {
		return nil, err
	}
	return &ct, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/matomo/... -run TestClient_.*Container -v`
Expected: all four tests `PASS`.

- [ ] **Step 5: Commit**

```bash
git add internal/matomo/tagmanager_containers.go internal/matomo/tagmanager_containers_test.go
git commit -m "feat(matomo): add Tag Manager container API client methods"
```

---

### Task 6: Tag Manager `GetContainerVersions` client method

This plan only needs draft-version *resolution* (finding the draft's ID so
tag/trigger/variable resources can target it); `createContainerVersion` and
`publishContainerVersion` belong to the actions plan (spec phase 7).

**Files:**
- Create: `internal/matomo/tagmanager_versions.go`
- Test: `internal/matomo/tagmanager_versions_test.go`

**Interfaces:**
- Consumes: `Client.call` (Task 2).
- Produces:
  - `type ContainerVersion struct { IDContainerVersion string; Name string; IsDraft bool }`
  - `func (c *Client) GetContainerVersions(ctx context.Context, idSite int, idContainer string) ([]ContainerVersion, error)`

- [ ] **Step 1: Write the failing test**

`internal/matomo/tagmanager_versions_test.go`:
```go
package matomo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_GetContainerVersions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "TagManager.getContainerVersions" {
			t.Errorf("method = %q, want TagManager.getContainerVersions", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"idcontainerversion": "1", "name": "Draft", "isDraft": true},
			{"idcontainerversion": "2", "name": "v1", "isDraft": false},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	versions, err := c.GetContainerVersions(context.Background(), 3, "abc123")
	if err != nil {
		t.Fatalf("GetContainerVersions() error = %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("len(versions) = %d, want 2", len(versions))
	}
	if !versions[0].IsDraft || versions[0].IDContainerVersion != "1" {
		t.Errorf("versions[0] = %+v, want draft with IDContainerVersion=1", versions[0])
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/matomo/... -run TestClient_GetContainerVersions -v`
Expected: FAIL — compile error, `tagmanager_versions.go` does not exist yet.

- [ ] **Step 3: Write the implementation**

`internal/matomo/tagmanager_versions.go`:
```go
package matomo

import (
	"context"
	"net/url"
	"strconv"
)

// ContainerVersion is a Tag Manager container version.
type ContainerVersion struct {
	IDContainerVersion string `json:"idcontainerversion"`
	Name               string `json:"name"`
	IsDraft            bool   `json:"isDraft"`
}

// GetContainerVersions lists every version of a container, including the
// mutable draft (IsDraft == true).
func (c *Client) GetContainerVersions(ctx context.Context, idSite int, idContainer string) ([]ContainerVersion, error) {
	v := url.Values{
		"idSite":      {strconv.Itoa(idSite)},
		"idContainer": {idContainer},
	}
	var versions []ContainerVersion
	if err := c.call(ctx, "TagManager.getContainerVersions", v, &versions); err != nil {
		return nil, err
	}
	return versions, nil
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/matomo/... -run TestClient_GetContainerVersions -v`
Expected: `PASS`.

- [ ] **Step 5: Commit**

```bash
git add internal/matomo/tagmanager_versions.go internal/matomo/tagmanager_versions_test.go
git commit -m "feat(matomo): add GetContainerVersions client method"
```

---

### Task 7: Tag Manager tag client methods

Matomo's `TagManager.*` HTTP API takes complex parameters (`parameters`,
`fireTriggerIds`, `blockTriggerIds`) as JSON-encoded strings within the
query string — this is Matomo's documented convention for array-typed
parameters on its JSON API. **When this task is executed, the implementer
must verify the encoding against a real Matomo instance** (the
docker-compose fixture from the spec, stood up early in whichever later
plan adds acceptance tests) and adjust `tagParamsToValues` below if Matomo
expects a different form (e.g. repeated `parameters[name]=value` instead).
The unit tests in this task only verify the client's own encode/decode
round-trip, not Matomo's actual wire format.

**Files:**
- Create: `internal/matomo/tagmanager_tags.go`
- Test: `internal/matomo/tagmanager_tags_test.go`

**Interfaces:**
- Consumes: `Client.call` (Task 2).
- Produces:
  - `type Tag struct { IDTag string; Name string; Type string; Status string; Parameters map[string]string; FireTriggerIDs []string; BlockTriggerIDs []string }`
  - `type TagParams struct { Type, Name string; Parameters map[string]string; FireTriggerIDs, BlockTriggerIDs []string }`
  - `func (c *Client) AddContainerTag(ctx context.Context, idSite int, idContainer, idContainerVersion string, p TagParams) (string, error)`
  - `func (c *Client) UpdateContainerTag(ctx context.Context, idSite int, idContainer, idContainerVersion, idTag string, p TagParams) error`
  - `func (c *Client) DeleteContainerTag(ctx context.Context, idSite int, idContainer, idContainerVersion, idTag string) error`
  - `func (c *Client) GetContainerTag(ctx context.Context, idSite int, idContainer, idContainerVersion, idTag string) (*Tag, error)`
  - `func (c *Client) PauseContainerTag(ctx context.Context, idSite int, idContainer, idContainerVersion, idTag string) error`
  - `func (c *Client) ResumeContainerTag(ctx context.Context, idSite int, idContainer, idContainerVersion, idTag string) error`

- [ ] **Step 1: Write the failing tests**

`internal/matomo/tagmanager_tags_test.go`:
```go
package matomo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_AddContainerTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "TagManager.addContainerTag" {
			t.Errorf("method = %q, want TagManager.addContainerTag", got)
		}
		if got := r.URL.Query().Get("type"); got != "CustomHtml" {
			t.Errorf("type = %q, want CustomHtml", got)
		}
		var params map[string]string
		if err := json.Unmarshal([]byte(r.URL.Query().Get("parameters")), &params); err != nil {
			t.Fatalf("parameters not valid JSON: %v", err)
		}
		if params["customHtml"] != "<script></script>" {
			t.Errorf("parameters.customHtml = %q, want <script></script>", params["customHtml"])
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"idtag": "5"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	id, err := c.AddContainerTag(context.Background(), 3, "abc123", "1", TagParams{
		Type:       "CustomHtml",
		Name:       "My tag",
		Parameters: map[string]string{"customHtml": "<script></script>"},
	})
	if err != nil {
		t.Fatalf("AddContainerTag() error = %v", err)
	}
	if id != "5" {
		t.Errorf("id = %q, want 5", id)
	}
}

func TestClient_UpdateContainerTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "TagManager.updateContainerTag" {
			t.Errorf("method = %q, want TagManager.updateContainerTag", got)
		}
		if got := r.URL.Query().Get("idTag"); got != "5" {
			t.Errorf("idTag = %q, want 5", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	err := c.UpdateContainerTag(context.Background(), 3, "abc123", "1", "5", TagParams{Type: "CustomHtml", Name: "Renamed"})
	if err != nil {
		t.Fatalf("UpdateContainerTag() error = %v", err)
	}
}

func TestClient_DeleteContainerTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "TagManager.deleteContainerTag" {
			t.Errorf("method = %q, want TagManager.deleteContainerTag", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	if err := c.DeleteContainerTag(context.Background(), 3, "abc123", "1", "5"); err != nil {
		t.Fatalf("DeleteContainerTag() error = %v", err)
	}
}

func TestClient_GetContainerTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "TagManager.getContainerTag" {
			t.Errorf("method = %q, want TagManager.getContainerTag", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"idtag": "5", "name": "My tag", "type": "CustomHtml", "status": "active",
			"parameters": map[string]any{"customHtml": "<script></script>"},
			"fireTriggerIds": []string{"1"},
			"blockTriggerIds": []string{},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	tag, err := c.GetContainerTag(context.Background(), 3, "abc123", "1", "5")
	if err != nil {
		t.Fatalf("GetContainerTag() error = %v", err)
	}
	if tag.IDTag != "5" || tag.Status != "active" || tag.Parameters["customHtml"] != "<script></script>" {
		t.Errorf("tag = %+v, want IDTag=5 Status=active Parameters.customHtml=<script></script>", tag)
	}
	if len(tag.FireTriggerIDs) != 1 || tag.FireTriggerIDs[0] != "1" {
		t.Errorf("tag.FireTriggerIDs = %v, want [1]", tag.FireTriggerIDs)
	}
}

func TestClient_PauseResumeContainerTag(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.URL.Query().Get("method")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	if err := c.PauseContainerTag(context.Background(), 3, "abc123", "1", "5"); err != nil {
		t.Fatalf("PauseContainerTag() error = %v", err)
	}
	if gotMethod != "TagManager.pauseContainerTag" {
		t.Errorf("method = %q, want TagManager.pauseContainerTag", gotMethod)
	}

	if err := c.ResumeContainerTag(context.Background(), 3, "abc123", "1", "5"); err != nil {
		t.Fatalf("ResumeContainerTag() error = %v", err)
	}
	if gotMethod != "TagManager.resumeContainerTag" {
		t.Errorf("method = %q, want TagManager.resumeContainerTag", gotMethod)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/matomo/... -run TestClient_.*ContainerTag -v`
Expected: FAIL — compile error, `tagmanager_tags.go` does not exist yet.

- [ ] **Step 3: Write the implementation**

`internal/matomo/tagmanager_tags.go`:
```go
package matomo

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
)

// Tag is a Matomo Tag Manager tag within a container version.
type Tag struct {
	IDTag           string            `json:"idtag"`
	Name            string            `json:"name"`
	Type            string            `json:"type"`
	Status          string            `json:"status"`
	Parameters      map[string]string `json:"parameters"`
	FireTriggerIDs  []string          `json:"fireTriggerIds"`
	BlockTriggerIDs []string          `json:"blockTriggerIds"`
}

// TagParams holds the fields accepted by addContainerTag/updateContainerTag.
type TagParams struct {
	Type            string
	Name            string
	Parameters      map[string]string
	FireTriggerIDs  []string
	BlockTriggerIDs []string
}

func tagParamsToValues(idSite int, idContainer, idContainerVersion string, p TagParams) (url.Values, error) {
	v := url.Values{
		"idSite":             {strconv.Itoa(idSite)},
		"idContainer":        {idContainer},
		"idContainerVersion": {idContainerVersion},
		"type":               {p.Type},
		"name":               {p.Name},
	}
	params := p.Parameters
	if params == nil {
		params = map[string]string{}
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	v.Set("parameters", string(paramsJSON))

	fireIDs := p.FireTriggerIDs
	if fireIDs == nil {
		fireIDs = []string{}
	}
	fireJSON, err := json.Marshal(fireIDs)
	if err != nil {
		return nil, err
	}
	v.Set("fireTriggerIds", string(fireJSON))

	blockIDs := p.BlockTriggerIDs
	if blockIDs == nil {
		blockIDs = []string{}
	}
	blockJSON, err := json.Marshal(blockIDs)
	if err != nil {
		return nil, err
	}
	v.Set("blockTriggerIds", string(blockJSON))

	return v, nil
}

// AddContainerTag creates a tag in a container's version and returns its ID.
func (c *Client) AddContainerTag(ctx context.Context, idSite int, idContainer, idContainerVersion string, p TagParams) (string, error) {
	v, err := tagParamsToValues(idSite, idContainer, idContainerVersion, p)
	if err != nil {
		return "", err
	}
	var out struct {
		IDTag string `json:"idtag"`
	}
	if err := c.call(ctx, "TagManager.addContainerTag", v, &out); err != nil {
		return "", err
	}
	return out.IDTag, nil
}

// UpdateContainerTag updates an existing tag.
func (c *Client) UpdateContainerTag(ctx context.Context, idSite int, idContainer, idContainerVersion, idTag string, p TagParams) error {
	v, err := tagParamsToValues(idSite, idContainer, idContainerVersion, p)
	if err != nil {
		return err
	}
	v.Set("idTag", idTag)
	return c.call(ctx, "TagManager.updateContainerTag", v, nil)
}

// DeleteContainerTag deletes a tag from a container's version.
func (c *Client) DeleteContainerTag(ctx context.Context, idSite int, idContainer, idContainerVersion, idTag string) error {
	v := url.Values{
		"idSite":             {strconv.Itoa(idSite)},
		"idContainer":        {idContainer},
		"idContainerVersion": {idContainerVersion},
		"idTag":              {idTag},
	}
	return c.call(ctx, "TagManager.deleteContainerTag", v, nil)
}

// GetContainerTag returns a single tag's configuration.
func (c *Client) GetContainerTag(ctx context.Context, idSite int, idContainer, idContainerVersion, idTag string) (*Tag, error) {
	v := url.Values{
		"idSite":             {strconv.Itoa(idSite)},
		"idContainer":        {idContainer},
		"idContainerVersion": {idContainerVersion},
		"idTag":              {idTag},
	}
	var tag Tag
	if err := c.call(ctx, "TagManager.getContainerTag", v, &tag); err != nil {
		return nil, err
	}
	return &tag, nil
}

func tagIdentityValues(idSite int, idContainer, idContainerVersion, idTag string) url.Values {
	return url.Values{
		"idSite":             {strconv.Itoa(idSite)},
		"idContainer":        {idContainer},
		"idContainerVersion": {idContainerVersion},
		"idTag":              {idTag},
	}
}

// PauseContainerTag pauses a tag in the draft version. This does not take
// effect on a live (published) container until a new version is created
// and published.
func (c *Client) PauseContainerTag(ctx context.Context, idSite int, idContainer, idContainerVersion, idTag string) error {
	return c.call(ctx, "TagManager.pauseContainerTag", tagIdentityValues(idSite, idContainer, idContainerVersion, idTag), nil)
}

// ResumeContainerTag resumes a paused tag in the draft version. See
// PauseContainerTag's note on when this takes effect live.
func (c *Client) ResumeContainerTag(ctx context.Context, idSite int, idContainer, idContainerVersion, idTag string) error {
	return c.call(ctx, "TagManager.resumeContainerTag", tagIdentityValues(idSite, idContainer, idContainerVersion, idTag), nil)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/matomo/... -run TestClient_.*ContainerTag -v`
Expected: all five tests `PASS`.

- [ ] **Step 5: Commit**

```bash
git add internal/matomo/tagmanager_tags.go internal/matomo/tagmanager_tags_test.go
git commit -m "feat(matomo): add Tag Manager tag API client methods"
```

---

### Task 8: Tag Manager trigger client methods

Same wire-format caveat as Task 7 applies to `conditions` here — verify
against a live Matomo instance when this task is executed.

**Files:**
- Create: `internal/matomo/tagmanager_triggers.go`
- Test: `internal/matomo/tagmanager_triggers_test.go`

**Interfaces:**
- Consumes: `Client.call` (Task 2).
- Produces:
  - `type Condition struct { Comparison string; ActualValueVariableID string; ExpectedValue string }`
  - `type Trigger struct { IDTrigger string; Name string; Type string; Parameters map[string]string; Conditions []Condition }`
  - `type TriggerParams struct { Type, Name string; Parameters map[string]string; Conditions []Condition }`
  - `func (c *Client) AddContainerTrigger(ctx context.Context, idSite int, idContainer, idContainerVersion string, p TriggerParams) (string, error)`
  - `func (c *Client) UpdateContainerTrigger(ctx context.Context, idSite int, idContainer, idContainerVersion, idTrigger string, p TriggerParams) error`
  - `func (c *Client) DeleteContainerTrigger(ctx context.Context, idSite int, idContainer, idContainerVersion, idTrigger string) error`
  - `func (c *Client) GetContainerTrigger(ctx context.Context, idSite int, idContainer, idContainerVersion, idTrigger string) (*Trigger, error)`

- [ ] **Step 1: Write the failing tests**

`internal/matomo/tagmanager_triggers_test.go`:
```go
package matomo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_AddContainerTrigger(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "TagManager.addContainerTrigger" {
			t.Errorf("method = %q, want TagManager.addContainerTrigger", got)
		}
		var conditions []Condition
		if err := json.Unmarshal([]byte(r.URL.Query().Get("conditions")), &conditions); err != nil {
			t.Fatalf("conditions not valid JSON: %v", err)
		}
		if len(conditions) != 1 || conditions[0].Comparison != "equals" {
			t.Errorf("conditions = %+v, want one equals condition", conditions)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"idtrigger": "7"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	id, err := c.AddContainerTrigger(context.Background(), 3, "abc123", "1", TriggerParams{
		Type: "PageView",
		Name: "All page views",
		Conditions: []Condition{
			{Comparison: "equals", ActualValueVariableID: "url_path", ExpectedValue: "/checkout"},
		},
	})
	if err != nil {
		t.Fatalf("AddContainerTrigger() error = %v", err)
	}
	if id != "7" {
		t.Errorf("id = %q, want 7", id)
	}
}

func TestClient_UpdateContainerTrigger(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "TagManager.updateContainerTrigger" {
			t.Errorf("method = %q, want TagManager.updateContainerTrigger", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	err := c.UpdateContainerTrigger(context.Background(), 3, "abc123", "1", "7", TriggerParams{Type: "PageView", Name: "Renamed"})
	if err != nil {
		t.Fatalf("UpdateContainerTrigger() error = %v", err)
	}
}

func TestClient_DeleteContainerTrigger(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "TagManager.deleteContainerTrigger" {
			t.Errorf("method = %q, want TagManager.deleteContainerTrigger", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	if err := c.DeleteContainerTrigger(context.Background(), 3, "abc123", "1", "7"); err != nil {
		t.Fatalf("DeleteContainerTrigger() error = %v", err)
	}
}

func TestClient_GetContainerTrigger(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "TagManager.getContainerTrigger" {
			t.Errorf("method = %q, want TagManager.getContainerTrigger", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"idtrigger": "7", "name": "All page views", "type": "PageView",
			"parameters": map[string]any{},
			"conditions": []map[string]any{},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	trig, err := c.GetContainerTrigger(context.Background(), 3, "abc123", "1", "7")
	if err != nil {
		t.Fatalf("GetContainerTrigger() error = %v", err)
	}
	if trig.IDTrigger != "7" || trig.Type != "PageView" {
		t.Errorf("trigger = %+v, want IDTrigger=7 Type=PageView", trig)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/matomo/... -run TestClient_.*ContainerTrigger -v`
Expected: FAIL — compile error, `tagmanager_triggers.go` does not exist yet.

- [ ] **Step 3: Write the implementation**

`internal/matomo/tagmanager_triggers.go`:
```go
package matomo

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
)

// Condition is one trigger-firing condition.
type Condition struct {
	Comparison             string `json:"comparison"`
	ActualValueVariableID  string `json:"actual"`
	ExpectedValue          string `json:"value"`
}

// Trigger is a Matomo Tag Manager trigger within a container version.
type Trigger struct {
	IDTrigger  string            `json:"idtrigger"`
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	Parameters map[string]string `json:"parameters"`
	Conditions []Condition       `json:"conditions"`
}

// TriggerParams holds the fields accepted by
// addContainerTrigger/updateContainerTrigger.
type TriggerParams struct {
	Type       string
	Name       string
	Parameters map[string]string
	Conditions []Condition
}

func triggerParamsToValues(idSite int, idContainer, idContainerVersion string, p TriggerParams) (url.Values, error) {
	v := url.Values{
		"idSite":             {strconv.Itoa(idSite)},
		"idContainer":        {idContainer},
		"idContainerVersion": {idContainerVersion},
		"type":               {p.Type},
		"name":               {p.Name},
	}
	params := p.Parameters
	if params == nil {
		params = map[string]string{}
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	v.Set("parameters", string(paramsJSON))

	conditions := p.Conditions
	if conditions == nil {
		conditions = []Condition{}
	}
	conditionsJSON, err := json.Marshal(conditions)
	if err != nil {
		return nil, err
	}
	v.Set("conditions", string(conditionsJSON))

	return v, nil
}

// AddContainerTrigger creates a trigger in a container's version and
// returns its ID.
func (c *Client) AddContainerTrigger(ctx context.Context, idSite int, idContainer, idContainerVersion string, p TriggerParams) (string, error) {
	v, err := triggerParamsToValues(idSite, idContainer, idContainerVersion, p)
	if err != nil {
		return "", err
	}
	var out struct {
		IDTrigger string `json:"idtrigger"`
	}
	if err := c.call(ctx, "TagManager.addContainerTrigger", v, &out); err != nil {
		return "", err
	}
	return out.IDTrigger, nil
}

// UpdateContainerTrigger updates an existing trigger.
func (c *Client) UpdateContainerTrigger(ctx context.Context, idSite int, idContainer, idContainerVersion, idTrigger string, p TriggerParams) error {
	v, err := triggerParamsToValues(idSite, idContainer, idContainerVersion, p)
	if err != nil {
		return err
	}
	v.Set("idTrigger", idTrigger)
	return c.call(ctx, "TagManager.updateContainerTrigger", v, nil)
}

// DeleteContainerTrigger deletes a trigger from a container's version.
func (c *Client) DeleteContainerTrigger(ctx context.Context, idSite int, idContainer, idContainerVersion, idTrigger string) error {
	v := url.Values{
		"idSite":             {strconv.Itoa(idSite)},
		"idContainer":        {idContainer},
		"idContainerVersion": {idContainerVersion},
		"idTrigger":          {idTrigger},
	}
	return c.call(ctx, "TagManager.deleteContainerTrigger", v, nil)
}

// GetContainerTrigger returns a single trigger's configuration.
func (c *Client) GetContainerTrigger(ctx context.Context, idSite int, idContainer, idContainerVersion, idTrigger string) (*Trigger, error) {
	v := url.Values{
		"idSite":             {strconv.Itoa(idSite)},
		"idContainer":        {idContainer},
		"idContainerVersion": {idContainerVersion},
		"idTrigger":          {idTrigger},
	}
	var trig Trigger
	if err := c.call(ctx, "TagManager.getContainerTrigger", v, &trig); err != nil {
		return nil, err
	}
	return &trig, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/matomo/... -run TestClient_.*ContainerTrigger -v`
Expected: all four tests `PASS`.

- [ ] **Step 5: Commit**

```bash
git add internal/matomo/tagmanager_triggers.go internal/matomo/tagmanager_triggers_test.go
git commit -m "feat(matomo): add Tag Manager trigger API client methods"
```

---

### Task 9: Tag Manager variable client methods

**Files:**
- Create: `internal/matomo/tagmanager_variables.go`
- Test: `internal/matomo/tagmanager_variables_test.go`

**Interfaces:**
- Consumes: `Client.call` (Task 2).
- Produces:
  - `type Variable struct { IDVariable string; Name string; Type string; Parameters map[string]string; DefaultValue string }`
  - `type VariableParams struct { Type, Name string; Parameters map[string]string; DefaultValue *string }`
  - `func (c *Client) AddContainerVariable(ctx context.Context, idSite int, idContainer, idContainerVersion string, p VariableParams) (string, error)`
  - `func (c *Client) UpdateContainerVariable(ctx context.Context, idSite int, idContainer, idContainerVersion, idVariable string, p VariableParams) error`
  - `func (c *Client) DeleteContainerVariable(ctx context.Context, idSite int, idContainer, idContainerVersion, idVariable string) error`
  - `func (c *Client) GetContainerVariable(ctx context.Context, idSite int, idContainer, idContainerVersion, idVariable string) (*Variable, error)`

- [ ] **Step 1: Write the failing tests**

`internal/matomo/tagmanager_variables_test.go`:
```go
package matomo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_AddContainerVariable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "TagManager.addContainerVariable" {
			t.Errorf("method = %q, want TagManager.addContainerVariable", got)
		}
		if got := r.URL.Query().Get("defaultValue"); got != "n/a" {
			t.Errorf("defaultValue = %q, want n/a", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"idvariable": "9"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	defaultValue := "n/a"
	id, err := c.AddContainerVariable(context.Background(), 3, "abc123", "1", VariableParams{
		Type:         "Constant",
		Name:         "My var",
		DefaultValue: &defaultValue,
	})
	if err != nil {
		t.Fatalf("AddContainerVariable() error = %v", err)
	}
	if id != "9" {
		t.Errorf("id = %q, want 9", id)
	}
}

func TestClient_UpdateContainerVariable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "TagManager.updateContainerVariable" {
			t.Errorf("method = %q, want TagManager.updateContainerVariable", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	err := c.UpdateContainerVariable(context.Background(), 3, "abc123", "1", "9", VariableParams{Type: "Constant", Name: "Renamed"})
	if err != nil {
		t.Fatalf("UpdateContainerVariable() error = %v", err)
	}
}

func TestClient_DeleteContainerVariable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "TagManager.deleteContainerVariable" {
			t.Errorf("method = %q, want TagManager.deleteContainerVariable", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	if err := c.DeleteContainerVariable(context.Background(), 3, "abc123", "1", "9"); err != nil {
		t.Fatalf("DeleteContainerVariable() error = %v", err)
	}
}

func TestClient_GetContainerVariable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("method"); got != "TagManager.getContainerVariable" {
			t.Errorf("method = %q, want TagManager.getContainerVariable", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"idvariable": "9", "name": "My var", "type": "Constant",
			"parameters": map[string]any{}, "defaultValue": "n/a",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", srv.Client())
	v, err := c.GetContainerVariable(context.Background(), 3, "abc123", "1", "9")
	if err != nil {
		t.Fatalf("GetContainerVariable() error = %v", err)
	}
	if v.IDVariable != "9" || v.DefaultValue != "n/a" {
		t.Errorf("variable = %+v, want IDVariable=9 DefaultValue=n/a", v)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/matomo/... -run TestClient_.*ContainerVariable -v`
Expected: FAIL — compile error, `tagmanager_variables.go` does not exist yet.

- [ ] **Step 3: Write the implementation**

`internal/matomo/tagmanager_variables.go`:
```go
package matomo

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
)

// Variable is a Matomo Tag Manager variable within a container version.
type Variable struct {
	IDVariable   string            `json:"idvariable"`
	Name         string            `json:"name"`
	Type         string            `json:"type"`
	Parameters   map[string]string `json:"parameters"`
	DefaultValue string            `json:"defaultValue"`
}

// VariableParams holds the fields accepted by
// addContainerVariable/updateContainerVariable.
type VariableParams struct {
	Type         string
	Name         string
	Parameters   map[string]string
	DefaultValue *string
}

func variableParamsToValues(idSite int, idContainer, idContainerVersion string, p VariableParams) (url.Values, error) {
	v := url.Values{
		"idSite":             {strconv.Itoa(idSite)},
		"idContainer":        {idContainer},
		"idContainerVersion": {idContainerVersion},
		"type":               {p.Type},
		"name":               {p.Name},
	}
	params := p.Parameters
	if params == nil {
		params = map[string]string{}
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	v.Set("parameters", string(paramsJSON))

	if p.DefaultValue != nil {
		v.Set("defaultValue", *p.DefaultValue)
	}

	return v, nil
}

// AddContainerVariable creates a variable in a container's version and
// returns its ID.
func (c *Client) AddContainerVariable(ctx context.Context, idSite int, idContainer, idContainerVersion string, p VariableParams) (string, error) {
	v, err := variableParamsToValues(idSite, idContainer, idContainerVersion, p)
	if err != nil {
		return "", err
	}
	var out struct {
		IDVariable string `json:"idvariable"`
	}
	if err := c.call(ctx, "TagManager.addContainerVariable", v, &out); err != nil {
		return "", err
	}
	return out.IDVariable, nil
}

// UpdateContainerVariable updates an existing variable.
func (c *Client) UpdateContainerVariable(ctx context.Context, idSite int, idContainer, idContainerVersion, idVariable string, p VariableParams) error {
	v, err := variableParamsToValues(idSite, idContainer, idContainerVersion, p)
	if err != nil {
		return err
	}
	v.Set("idVariable", idVariable)
	return c.call(ctx, "TagManager.updateContainerVariable", v, nil)
}

// DeleteContainerVariable deletes a variable from a container's version.
func (c *Client) DeleteContainerVariable(ctx context.Context, idSite int, idContainer, idContainerVersion, idVariable string) error {
	v := url.Values{
		"idSite":             {strconv.Itoa(idSite)},
		"idContainer":        {idContainer},
		"idContainerVersion": {idContainerVersion},
		"idVariable":         {idVariable},
	}
	return c.call(ctx, "TagManager.deleteContainerVariable", v, nil)
}

// GetContainerVariable returns a single variable's configuration.
func (c *Client) GetContainerVariable(ctx context.Context, idSite int, idContainer, idContainerVersion, idVariable string) (*Variable, error) {
	v := url.Values{
		"idSite":             {strconv.Itoa(idSite)},
		"idContainer":        {idContainer},
		"idContainerVersion": {idContainerVersion},
		"idVariable":         {idVariable},
	}
	var variable Variable
	if err := c.call(ctx, "TagManager.getContainerVariable", v, &variable); err != nil {
		return nil, err
	}
	return &variable, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/matomo/... -run TestClient_.*ContainerVariable -v`
Expected: all four tests `PASS`.

- [ ] **Step 5: Run the full client package test suite**

Run: `go test ./internal/matomo/... -v`
Expected: every test in `internal/matomo` `PASS` (Tasks 2-9 combined).

- [ ] **Step 6: Commit**

```bash
git add internal/matomo/tagmanager_variables.go internal/matomo/tagmanager_variables_test.go
git commit -m "feat(matomo): add Tag Manager variable API client methods"
```

---

### Task 10: Composite resource ID helpers

Implements spec §4's `site_id`/`container_id`/`entity_id` composite ID
scheme. Every later resource task (11, 13, 14, 16-18) uses these instead of
hand-rolling `strings.Split`.

**Files:**
- Create: `internal/provider/ids.go`
- Test: `internal/provider/ids_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `func buildContainerID(siteID int, idContainer string) string` → `"{siteID}/{idContainer}"`
  - `func parseContainerID(id string) (siteID int, idContainer string, err error)`
  - `func buildDimensionID(siteID int, index int) string` → `"{siteID}/{index}"`
  - `func parseDimensionID(id string) (siteID int, index int, err error)`
  - `func buildEntityID(siteID int, idContainer, entityID string) string` → `"{siteID}/{idContainer}/{entityID}"`
  - `func parseEntityID(id string) (siteID int, idContainer, entityID string, err error)`

- [ ] **Step 1: Write the failing tests**

`internal/provider/ids_test.go`:
```go
package provider

import "testing"

func TestBuildParseContainerID(t *testing.T) {
	id := buildContainerID(3, "abc123")
	if id != "3/abc123" {
		t.Fatalf("buildContainerID() = %q, want 3/abc123", id)
	}
	siteID, idContainer, err := parseContainerID(id)
	if err != nil {
		t.Fatalf("parseContainerID() error = %v", err)
	}
	if siteID != 3 || idContainer != "abc123" {
		t.Errorf("parseContainerID() = (%d, %q), want (3, abc123)", siteID, idContainer)
	}
}

func TestParseContainerID_invalid(t *testing.T) {
	cases := []string{"", "3", "3/abc/extra", "notanumber/abc123"}
	for _, c := range cases {
		if _, _, err := parseContainerID(c); err == nil {
			t.Errorf("parseContainerID(%q) error = nil, want error", c)
		}
	}
}

func TestBuildParseDimensionID(t *testing.T) {
	id := buildDimensionID(3, 1)
	if id != "3/1" {
		t.Fatalf("buildDimensionID() = %q, want 3/1", id)
	}
	siteID, index, err := parseDimensionID(id)
	if err != nil {
		t.Fatalf("parseDimensionID() error = %v", err)
	}
	if siteID != 3 || index != 1 {
		t.Errorf("parseDimensionID() = (%d, %d), want (3, 1)", siteID, index)
	}
}

func TestBuildParseEntityID(t *testing.T) {
	id := buildEntityID(3, "abc123", "5")
	if id != "3/abc123/5" {
		t.Fatalf("buildEntityID() = %q, want 3/abc123/5", id)
	}
	siteID, idContainer, entityID, err := parseEntityID(id)
	if err != nil {
		t.Fatalf("parseEntityID() error = %v", err)
	}
	if siteID != 3 || idContainer != "abc123" || entityID != "5" {
		t.Errorf("parseEntityID() = (%d, %q, %q), want (3, abc123, 5)", siteID, idContainer, entityID)
	}
}

func TestParseEntityID_invalid(t *testing.T) {
	cases := []string{"", "3/abc123", "3/abc123/5/extra"}
	for _, c := range cases {
		if _, _, _, err := parseEntityID(c); err == nil {
			t.Errorf("parseEntityID(%q) error = nil, want error", c)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/provider/... -run 'TestBuildParse|TestParse' -v`
Expected: FAIL — compile error, `ids.go` does not exist yet.

- [ ] **Step 3: Write the implementation**

`internal/provider/ids.go`:
```go
package provider

import (
	"fmt"
	"strconv"
	"strings"
)

func buildContainerID(siteID int, idContainer string) string {
	return fmt.Sprintf("%d/%s", siteID, idContainer)
}

func parseContainerID(id string) (siteID int, idContainer string, err error) {
	parts := strings.Split(id, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return 0, "", fmt.Errorf("invalid container id %q, expected format \"site_id/container_id\"", id)
	}
	siteID, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", fmt.Errorf("invalid container id %q: site_id segment is not numeric: %w", id, err)
	}
	return siteID, parts[1], nil
}

func buildDimensionID(siteID int, index int) string {
	return fmt.Sprintf("%d/%d", siteID, index)
}

func parseDimensionID(id string) (siteID int, index int, err error) {
	parts := strings.Split(id, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return 0, 0, fmt.Errorf("invalid custom dimension id %q, expected format \"site_id/index\"", id)
	}
	siteID, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid custom dimension id %q: site_id segment is not numeric: %w", id, err)
	}
	index, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid custom dimension id %q: index segment is not numeric: %w", id, err)
	}
	return siteID, index, nil
}

func buildEntityID(siteID int, idContainer, entityID string) string {
	return fmt.Sprintf("%d/%s/%s", siteID, idContainer, entityID)
}

func parseEntityID(id string) (siteID int, idContainer, entityID string, err error) {
	parts := strings.Split(id, "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return 0, "", "", fmt.Errorf("invalid id %q, expected format \"site_id/container_id/entity_id\"", id)
	}
	siteID, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", "", fmt.Errorf("invalid id %q: site_id segment is not numeric: %w", id, err)
	}
	return siteID, parts[1], parts[2], nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/provider/... -run 'TestBuildParse|TestParse' -v`
Expected: all six tests `PASS`.

- [ ] **Step 5: Commit**

```bash
git add internal/provider/ids.go internal/provider/ids_test.go
git commit -m "feat(provider): add composite resource ID helpers"
```

---

### Task 11: Provider `Configure` (builds the shared client) + `matomo_site` resource

**Files:**
- Modify: `internal/provider/provider.go` (fill in `Configure`, register the resource in `Resources()`)
- Modify: `internal/provider/provider_test.go` (add a `Configure` test)
- Create: `internal/provider/resource_site.go`
- Test: `internal/provider/resource_site_test.go`

**Interfaces:**
- Consumes: `matomo.NewClient`, `matomo.Client.{AddSite,UpdateSite,DeleteSite,GetSiteFromID}` (Task 3); `providerserver.NewProtocol6WithError` pattern from `terraform-plugin-testing`.
- Produces:
  - `func (p *MatomoProvider) Configure(...)` populates `resp.ResourceData` / `resp.DataSourceData` with a `*matomo.Client`.
  - `func NewSiteResource() resource.Resource` — registered in `Resources()`.
  - Pattern every later resource task (13, 14, 16-18) follows: a `Resource` struct holding `client *matomo.Client`, with `Configure(ctx, req, resp)` doing `r.client = req.ProviderData.(*matomo.Client)`.

- [ ] **Step 1: Write the failing `Configure` test**

Append to `internal/provider/provider_test.go`:
```go
func TestMatomoProvider_Configure(t *testing.T) {
	t.Setenv("MATOMO_BASE_URL", "")
	t.Setenv("MATOMO_API_TOKEN", "")

	p := New("test")().(*MatomoProvider)
	req := provider.ConfigureRequest{
		Config: tfsdk.Config{
			Raw: tftypes.NewValue(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"base_url":              tftypes.String,
					"api_token":             tftypes.String,
					"insecure_skip_verify":  tftypes.Bool,
				},
			}, map[string]tftypes.Value{
				"base_url":             tftypes.NewValue(tftypes.String, "https://matomo.example.com"),
				"api_token":            tftypes.NewValue(tftypes.String, "test-token"),
				"insecure_skip_verify": tftypes.NewValue(tftypes.Bool, nil),
			}),
			Schema: func() schema.Schema {
				resp := &provider.SchemaResponse{}
				p.Schema(context.Background(), provider.SchemaRequest{}, resp)
				return resp.Schema
			}(),
		},
	}
	resp := &provider.ConfigureResponse{}
	p.Configure(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Configure() diagnostics = %v", resp.Diagnostics)
	}
	if resp.ResourceData == nil {
		t.Fatal("Configure() resp.ResourceData = nil, want *matomo.Client")
	}
	if _, ok := resp.ResourceData.(*matomo.Client); !ok {
		t.Fatalf("Configure() resp.ResourceData type = %T, want *matomo.Client", resp.ResourceData)
	}
}
```

Add the needed imports to `internal/provider/provider_test.go`: `"context"`, `"github.com/hashicorp/terraform-plugin-framework/provider/schema"`, `"github.com/hashicorp/terraform-plugin-framework/tfsdk"`, `"github.com/hashicorp/terraform-plugin-go/tftypes"`, `"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"`.

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/provider/... -run TestMatomoProvider_Configure -v`
Expected: FAIL — `resp.ResourceData` is `nil` (current `Configure` is a no-op).

- [ ] **Step 3: Implement `Configure`**

Replace the `Configure` method and add a model type in `internal/provider/provider.go`:
```go
type matomoProviderModel struct {
	BaseURL             types.String `tfsdk:"base_url"`
	APIToken            types.String `tfsdk:"api_token"`
	InsecureSkipVerify  types.Bool   `tfsdk:"insecure_skip_verify"`
}

func (p *MatomoProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config matomoProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	baseURL := config.BaseURL.ValueString()
	if baseURL == "" {
		baseURL = os.Getenv("MATOMO_BASE_URL")
	}
	if baseURL == "" {
		resp.Diagnostics.AddError(
			"Missing Matomo base URL",
			"Set base_url in the provider configuration or the MATOMO_BASE_URL environment variable.",
		)
	}

	apiToken := config.APIToken.ValueString()
	if apiToken == "" {
		apiToken = os.Getenv("MATOMO_API_TOKEN")
	}
	if apiToken == "" {
		resp.Diagnostics.AddError(
			"Missing Matomo API token",
			"Set api_token in the provider configuration or the MATOMO_API_TOKEN environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	httpClient := &http.Client{}
	if config.InsecureSkipVerify.ValueBool() {
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // explicit opt-in via provider config
		}
	}

	client := matomo.NewClient(baseURL, apiToken, httpClient)
	resp.ResourceData = client
	resp.DataSourceData = client
}
```

Add imports to `internal/provider/provider.go`: `"crypto/tls"`, `"net/http"`, `"os"`, `"github.com/hashicorp/terraform-plugin-framework/types"`, `"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"`.

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/provider/... -run TestMatomoProvider_Configure -v`
Expected: `PASS`.

- [ ] **Step 5: Write the failing `matomo_site` resource test**

`internal/provider/resource_site_test.go` uses `terraform-plugin-testing`'s `resource.UnitTest` against an `httptest` Matomo stand-in, going through the real provider wiring end-to-end:
```go
package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func newTestProviderFactories(srv *httptest.Server) map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"matomo": providerserver.NewProtocol6WithError(New("test")()),
	}
}

func testAccPreCheckConfig(srv *httptest.Server) string {
	return `
provider "matomo" {
  base_url  = "` + srv.URL + `"
  api_token = "test-token"
}
`
}

func TestAccSiteResource_basic(t *testing.T) {
	sites := map[string]map[string]any{}
	nextID := 1

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		method := r.URL.Query().Get("method")
		switch method {
		case "SitesManager.addSite":
			id := nextID
			nextID++
			idStr := strconv.Itoa(id)
			sites[idStr] = map[string]any{
				"idsite": idStr, "name": r.URL.Query().Get("siteName"),
				"timezone": "UTC", "currency": "USD",
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"value": idStr})
		case "SitesManager.updateSite":
			id := r.URL.Query().Get("idSite")
			sites[id]["name"] = r.URL.Query().Get("siteName")
			_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
		case "SitesManager.deleteSite":
			delete(sites, r.URL.Query().Get("idSite"))
			_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
		case "SitesManager.getSiteFromId":
			id := r.URL.Query().Get("idSite")
			site, ok := sites[id]
			if !ok {
				_ = json.NewEncoder(w).Encode(map[string]any{"result": "error", "message": "Website id Not found"})
				return
			}
			_ = json.NewEncoder(w).Encode(site)
		default:
			t.Fatalf("unexpected method %q", method)
		}
	}))
	defer srv.Close()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: newTestProviderFactories(srv),
		Steps: []resource.TestStep{
			{
				Config: testAccPreCheckConfig(srv) + `
resource "matomo_site" "test" {
  name = "Example"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("matomo_site.test", "name", "Example"),
					resource.TestCheckResourceAttrSet("matomo_site.test", "id"),
				),
			},
			{
				Config: testAccPreCheckConfig(srv) + `
resource "matomo_site" "test" {
  name = "Renamed"
}
`,
				Check: resource.TestCheckResourceAttr("matomo_site.test", "name", "Renamed"),
			},
		},
	})
}

```

- [ ] **Step 6: Run the test to verify it fails**

Run: `go test ./internal/provider/... -run TestAccSiteResource_basic -v`
Expected: FAIL — compile error, `resource_site.go` does not exist yet (no `matomo_site` resource registered).

- [ ] **Step 7: Implement the `matomo_site` resource**

`internal/provider/resource_site.go`:
```go
package provider

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

var (
	_ resource.Resource                = &siteResource{}
	_ resource.ResourceWithConfigure   = &siteResource{}
	_ resource.ResourceWithImportState = &siteResource{}
)

func NewSiteResource() resource.Resource {
	return &siteResource{}
}

type siteResource struct {
	client *matomo.Client
}

type siteResourceModel struct {
	ID       types.String `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	Timezone types.String `tfsdk:"timezone"`
	Currency types.String `tfsdk:"currency"`
}

func (r *siteResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site"
}

func (r *siteResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The site's numeric ID, assigned by Matomo on creation.",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The site's name.",
			},
			"timezone": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The site's timezone, e.g. \"UTC\" or \"America/New_York\".",
			},
			"currency": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The site's currency code, e.g. \"USD\".",
			},
		},
	}
}

func (r *siteResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*matomo.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", "expected *matomo.Client")
		return
	}
	r.client = client
}

func (r *siteResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan siteResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := matomo.AddSiteParams{Name: plan.Name.ValueString()}
	if !plan.Timezone.IsUnknown() && !plan.Timezone.IsNull() {
		tz := plan.Timezone.ValueString()
		params.Timezone = &tz
	}
	if !plan.Currency.IsUnknown() && !plan.Currency.IsNull() {
		cur := plan.Currency.ValueString()
		params.Currency = &cur
	}

	idSite, err := r.client.AddSite(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError("Error creating Matomo site", err.Error())
		return
	}

	r.readIntoModel(ctx, idSite, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *siteResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state siteResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	idSite, err := strconv.Atoi(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid site id in state", err.Error())
		return
	}

	site, err := r.client.GetSiteFromID(ctx, idSite)
	if err != nil {
		if apiErr, ok := err.(*matomo.APIError); ok && apiErr.Message == "Website id Not found" {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Matomo site", err.Error())
		return
	}

	state.ID = types.StringValue(strconv.Itoa(site.IDSite))
	state.Name = types.StringValue(site.Name)
	state.Timezone = types.StringValue(site.Timezone)
	state.Currency = types.StringValue(site.Currency)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *siteResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan siteResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state siteResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	idSite, err := strconv.Atoi(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid site id in state", err.Error())
		return
	}

	params := matomo.AddSiteParams{Name: plan.Name.ValueString()}
	if !plan.Timezone.IsUnknown() && !plan.Timezone.IsNull() {
		tz := plan.Timezone.ValueString()
		params.Timezone = &tz
	}
	if !plan.Currency.IsUnknown() && !plan.Currency.IsNull() {
		cur := plan.Currency.ValueString()
		params.Currency = &cur
	}

	if err := r.client.UpdateSite(ctx, idSite, matomo.UpdateSiteParams{AddSiteParams: params}); err != nil {
		resp.Diagnostics.AddError("Error updating Matomo site", err.Error())
		return
	}

	r.readIntoModel(ctx, idSite, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *siteResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state siteResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	idSite, err := strconv.Atoi(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid site id in state", err.Error())
		return
	}

	if err := r.client.DeleteSite(ctx, idSite); err != nil {
		resp.Diagnostics.AddError("Error deleting Matomo site", err.Error())
	}
}

func (r *siteResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// readIntoModel fetches the site by ID and copies its fields into model,
// preserving model.ID. Used by Create/Update to refresh computed fields
// after a write.
func (r *siteResource) readIntoModel(ctx context.Context, idSite int, model *siteResourceModel, diags *diag.Diagnostics) {
	site, err := r.client.GetSiteFromID(ctx, idSite)
	if err != nil {
		diags.AddError("Error reading Matomo site after write", err.Error())
		return
	}
	model.ID = types.StringValue(strconv.Itoa(site.IDSite))
	model.Name = types.StringValue(site.Name)
	model.Timezone = types.StringValue(site.Timezone)
	model.Currency = types.StringValue(site.Currency)
}
```

- [ ] **Step 8: Register the resource in the provider**

In `internal/provider/provider.go`, change `Resources`:
```go
func (p *MatomoProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewSiteResource,
	}
}
```

- [ ] **Step 9: Run the test to verify it passes**

Run: `go test ./internal/provider/... -run TestAccSiteResource_basic -v`
Expected: `PASS` (both steps — create and update — succeed).

- [ ] **Step 10: Run the full provider package test suite**

Run: `go test ./internal/provider/... -v`
Expected: every test `PASS`.

- [ ] **Step 11: Commit**

```bash
git add internal/provider/provider.go internal/provider/provider_test.go internal/provider/resource_site.go internal/provider/resource_site_test.go
git commit -m "feat(provider): implement provider Configure and matomo_site resource"
```

---

### Task 12: `data "matomo_site"`

Looks up a site Terraform doesn't manage, by `id` or by `name` (exactly one
required).

**Files:**
- Create: `internal/provider/data_source_site.go`
- Test: `internal/provider/data_source_site_test.go`
- Modify: `internal/provider/provider.go` (register in `DataSources()`)

**Interfaces:**
- Consumes: `matomo.Client.{GetSiteFromID,GetAllSites}` (Task 3).
- Produces: `func NewSiteDataSource() datasource.DataSource`.

- [ ] **Step 1: Write the failing test**

`internal/provider/data_source_site_test.go`:
```go
package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccSiteDataSource_byName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("method") != "SitesManager.getAllSites" {
			t.Fatalf("unexpected method %q", r.URL.Query().Get("method"))
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"idsite": "1", "name": "Other", "timezone": "UTC", "currency": "USD"},
			{"idsite": "2", "name": "Example", "timezone": "UTC", "currency": "USD"},
		})
	}))
	defer srv.Close()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: newTestProviderFactories(srv),
		Steps: []resource.TestStep{
			{
				Config: testAccPreCheckConfig(srv) + `
data "matomo_site" "test" {
  name = "Example"
}
`,
				Check: resource.TestCheckResourceAttr("data.matomo_site.test", "id", "2"),
			},
		},
	})
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/provider/... -run TestAccSiteDataSource_byName -v`
Expected: FAIL — compile error, `data_source_site.go` does not exist yet.

- [ ] **Step 3: Implement the data source**

`internal/provider/data_source_site.go`:
```go
package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

var (
	_ datasource.DataSource              = &siteDataSource{}
	_ datasource.DataSourceWithConfigure = &siteDataSource{}
)

func NewSiteDataSource() datasource.DataSource {
	return &siteDataSource{}
}

type siteDataSource struct {
	client *matomo.Client
}

type siteDataSourceModel struct {
	ID       types.String `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	Timezone types.String `tfsdk:"timezone"`
	Currency types.String `tfsdk:"currency"`
}

func (d *siteDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site"
}

func (d *siteDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The site's numeric ID. Exactly one of id or name is required.",
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(
						stringAttrPath("id"), stringAttrPath("name"),
					),
				},
			},
			"name": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The site's name. Exactly one of id or name is required.",
			},
			"timezone": schema.StringAttribute{Computed: true},
			"currency": schema.StringAttribute{Computed: true},
		},
	}
}

func (d *siteDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *siteDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config siteDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var site *matomo.Site

	if !config.ID.IsNull() {
		idSite, err := strconv.Atoi(config.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid id", err.Error())
			return
		}
		s, err := d.client.GetSiteFromID(ctx, idSite)
		if err != nil {
			resp.Diagnostics.AddError("Error reading Matomo site", err.Error())
			return
		}
		site = s
	} else {
		sites, err := d.client.GetAllSites(ctx)
		if err != nil {
			resp.Diagnostics.AddError("Error listing Matomo sites", err.Error())
			return
		}
		for i := range sites {
			if sites[i].Name == config.Name.ValueString() {
				site = &sites[i]
				break
			}
		}
		if site == nil {
			resp.Diagnostics.AddError("Site not found", fmt.Sprintf("no site named %q", config.Name.ValueString()))
			return
		}
	}

	config.ID = types.StringValue(strconv.Itoa(site.IDSite))
	config.Name = types.StringValue(site.Name)
	config.Timezone = types.StringValue(site.Timezone)
	config.Currency = types.StringValue(site.Currency)
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
```

`stringAttrPath` is a small helper for building `path.Expression` values for
validators; define it once in `internal/provider/ids.go` (it is not ID
logic, but it has no better home yet and avoids a one-function file):
```go
func stringAttrPath(name string) path.Expression {
	return path.MatchRoot(name)
}
```
Add `"github.com/hashicorp/terraform-plugin-framework/path"` to
`internal/provider/ids.go`'s imports.

- [ ] **Step 4: Register the data source in the provider**

In `internal/provider/provider.go`, change `DataSources`:
```go
func (p *MatomoProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewSiteDataSource,
	}
}
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `go test ./internal/provider/... -run TestAccSiteDataSource_byName -v`
Expected: `PASS`.

- [ ] **Step 6: Commit**

```bash
git add internal/provider/provider.go internal/provider/ids.go internal/provider/data_source_site.go internal/provider/data_source_site_test.go
git commit -m "feat(provider): add matomo_site data source"
```

---

### Task 13: `matomo_custom_dimension` resource

Implements spec §6.2's adopt-or-create-and-verify Create logic.
`extractions` (per-extraction regex rules) is left out of this resource for
now — not required by anything else in this plan's scope (YAGNI); add it as
a follow-up task when a consumer needs it.

**Files:**
- Create: `internal/provider/resource_custom_dimension.go`
- Test: `internal/provider/resource_custom_dimension_test.go`
- Modify: `internal/provider/provider.go` (register in `Resources()`)

**Interfaces:**
- Consumes: `matomo.Client.{ConfigureNewCustomDimension,ConfigureExistingCustomDimension,GetConfiguredCustomDimensions}` (Task 4); `buildDimensionID`/`parseDimensionID` (Task 10).
- Produces: `func NewCustomDimensionResource() resource.Resource`.

- [ ] **Step 1: Write the failing tests**

`internal/provider/resource_custom_dimension_test.go`:
```go
package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

type fakeDimension struct {
	ID, Index             int
	Scope, Name           string
	Active, CaseSensitive bool
}

func newCustomDimensionTestServer(t *testing.T, dims map[int]*fakeDimension) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("method") {
		case "CustomDimensions.getConfiguredCustomDimensions":
			out := []map[string]any{}
			for _, d := range dims {
				out = append(out, map[string]any{
					"id": strconv.Itoa(d.ID), "index": strconv.Itoa(d.Index),
					"scope": d.Scope, "name": d.Name, "active": d.Active, "case_sensitive": d.CaseSensitive,
				})
			}
			_ = json.NewEncoder(w).Encode(out)
		case "CustomDimensions.configureNewCustomDimension":
			id := len(dims) + 1
			dims[id] = &fakeDimension{
				ID: id, Index: id, Scope: r.URL.Query().Get("scope"),
				Name: r.URL.Query().Get("name"), Active: r.URL.Query().Get("active") == "1",
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": strconv.Itoa(id)})
		case "CustomDimensions.configureExistingCustomDimension":
			id, _ := strconv.Atoi(r.URL.Query().Get("idDimension"))
			d, ok := dims[id]
			if !ok {
				_ = json.NewEncoder(w).Encode(map[string]any{"result": "error", "message": "dimension not found"})
				return
			}
			d.Name = r.URL.Query().Get("name")
			d.Active = r.URL.Query().Get("active") == "1"
			_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
		default:
			t.Fatalf("unexpected method %q", r.URL.Query().Get("method"))
		}
	}))
}

func TestAccCustomDimensionResource_createsNewSlot(t *testing.T) {
	dims := map[int]*fakeDimension{}
	srv := newCustomDimensionTestServer(t, dims)
	defer srv.Close()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: newTestProviderFactories(srv),
		Steps: []resource.TestStep{
			{
				Config: testAccPreCheckConfig(srv) + `
resource "matomo_custom_dimension" "test" {
  site_id = "3"
  index   = 1
  scope   = "visit"
  name    = "Test Dimension"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("matomo_custom_dimension.test", "id", "3/1"),
					resource.TestCheckResourceAttr("matomo_custom_dimension.test", "active", "true"),
				),
			},
		},
	})
}

func TestAccCustomDimensionResource_adoptsExistingSlot(t *testing.T) {
	dims := map[int]*fakeDimension{
		1: {ID: 1, Index: 1, Scope: "visit", Name: "Pre-existing", Active: true},
	}
	srv := newCustomDimensionTestServer(t, dims)
	defer srv.Close()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: newTestProviderFactories(srv),
		Steps: []resource.TestStep{
			{
				Config: testAccPreCheckConfig(srv) + `
resource "matomo_custom_dimension" "test" {
  site_id = "3"
  index   = 1
  scope   = "visit"
  name    = "Adopted Name"
}
`,
				Check: resource.TestCheckResourceAttr("matomo_custom_dimension.test", "id", "3/1"),
			},
		},
	})

	if dims[1].Name != "Adopted Name" {
		t.Errorf("dims[1].Name = %q, want Adopted Name", dims[1].Name)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/provider/... -run TestAccCustomDimensionResource -v`
Expected: FAIL — compile error, `resource_custom_dimension.go` does not exist yet.

- [ ] **Step 3: Implement the resource**

`internal/provider/resource_custom_dimension.go`:
```go
package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

var (
	_ resource.Resource                = &customDimensionResource{}
	_ resource.ResourceWithConfigure   = &customDimensionResource{}
	_ resource.ResourceWithImportState = &customDimensionResource{}
)

func NewCustomDimensionResource() resource.Resource {
	return &customDimensionResource{}
}

type customDimensionResource struct {
	client *matomo.Client
}

type customDimensionResourceModel struct {
	ID     types.String `tfsdk:"id"`
	SiteID types.String `tfsdk:"site_id"`
	Index  types.Int64  `tfsdk:"index"`
	Scope  types.String `tfsdk:"scope"`
	Name   types.String `tfsdk:"name"`
	Active types.Bool   `tfsdk:"active"`
}

func (r *customDimensionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_custom_dimension"
}

func (r *customDimensionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Composite \"site_id/index\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"site_id": schema.StringAttribute{
				Required:    true,
				Description: "The owning site's id (matomo_site.x.id).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"index": schema.Int64Attribute{
				Required:    true,
				Description: "The dimension's slot number within its scope. You choose this; Matomo does not support picking a slot on creation, so Create verifies the slot it assigns matches this value.",
			},
			"scope": schema.StringAttribute{
				Required:    true,
				Description: "\"visit\" or \"action\".",
				Validators: []validator.String{
					stringvalidator.OneOf("visit", "action"),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The dimension's display name.",
			},
			"active": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the dimension is active. Matomo has no delete API for custom dimensions; destroying this resource sets active=false rather than removing the slot.",
			},
		},
	}
}

func (r *customDimensionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*matomo.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", "expected *matomo.Client")
		return
	}
	r.client = client
}

func (r *customDimensionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan customDimensionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, err := strconv.Atoi(plan.SiteID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid site_id", err.Error())
		return
	}
	declaredIndex := int(plan.Index.ValueInt64())
	scope := plan.Scope.ValueString()
	name := plan.Name.ValueString()
	active := true
	if !plan.Active.IsUnknown() && !plan.Active.IsNull() {
		active = plan.Active.ValueBool()
	}

	existing, err := r.client.GetConfiguredCustomDimensions(ctx, siteID)
	if err != nil {
		resp.Diagnostics.AddError("Error listing Matomo custom dimensions", err.Error())
		return
	}

	var match *matomo.CustomDimension
	for i := range existing {
		if existing[i].Index == declaredIndex && existing[i].Scope == scope {
			match = &existing[i]
			break
		}
	}

	if match != nil {
		if err := r.client.ConfigureExistingCustomDimension(ctx, match.ID, siteID, name, active); err != nil {
			resp.Diagnostics.AddError("Error adopting existing Matomo custom dimension", err.Error())
			return
		}
	} else {
		newIndex, err := r.client.ConfigureNewCustomDimension(ctx, siteID, name, scope, active)
		if err != nil {
			resp.Diagnostics.AddError("Error creating Matomo custom dimension", err.Error())
			return
		}
		if newIndex != declaredIndex {
			resp.Diagnostics.AddError(
				"Custom dimension slot mismatch",
				fmt.Sprintf(
					"Declared index %d for scope %q, but Matomo assigned slot %d instead (the next free slot was not %d — likely because a lower slot was consumed outside Terraform). "+
						"Slot %d has already been created in Matomo and cannot be deleted via its API; either declare index = %d for this resource, or bring slot %d under management with its own matomo_custom_dimension resource.",
					declaredIndex, scope, newIndex, declaredIndex, newIndex, newIndex, newIndex,
				),
			)
			return
		}
	}

	plan.ID = types.StringValue(buildDimensionID(siteID, declaredIndex))
	plan.Active = types.BoolValue(active)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *customDimensionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state customDimensionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, index, err := parseDimensionID(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id in state", err.Error())
		return
	}

	dims, err := r.client.GetConfiguredCustomDimensions(ctx, siteID)
	if err != nil {
		resp.Diagnostics.AddError("Error reading Matomo custom dimensions", err.Error())
		return
	}

	var found *matomo.CustomDimension
	for i := range dims {
		if dims[i].Index == index {
			found = &dims[i]
			break
		}
	}
	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	state.SiteID = types.StringValue(strconv.Itoa(siteID))
	state.Index = types.Int64Value(int64(found.Index))
	state.Scope = types.StringValue(found.Scope)
	state.Name = types.StringValue(found.Name)
	state.Active = types.BoolValue(found.Active)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *customDimensionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan customDimensionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, index, err := parseDimensionID(plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id in state", err.Error())
		return
	}

	active := true
	if !plan.Active.IsUnknown() && !plan.Active.IsNull() {
		active = plan.Active.ValueBool()
	}

	if err := r.client.ConfigureExistingCustomDimension(ctx, index, siteID, plan.Name.ValueString(), active); err != nil {
		resp.Diagnostics.AddError("Error updating Matomo custom dimension", err.Error())
		return
	}

	plan.Active = types.BoolValue(active)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *customDimensionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state customDimensionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, index, err := parseDimensionID(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id in state", err.Error())
		return
	}

	if err := r.client.ConfigureExistingCustomDimension(ctx, index, siteID, state.Name.ValueString(), false); err != nil {
		resp.Diagnostics.AddError("Error deactivating Matomo custom dimension", err.Error())
	}
}

func (r *customDimensionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
```

- [ ] **Step 4: Register the resource in the provider**

In `internal/provider/provider.go`, add `NewCustomDimensionResource` to the
slice returned by `Resources()`.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/provider/... -run TestAccCustomDimensionResource -v`
Expected: both tests `PASS`.

- [ ] **Step 6: Commit**

```bash
git add internal/provider/provider.go internal/provider/resource_custom_dimension.go internal/provider/resource_custom_dimension_test.go
git commit -m "feat(provider): add matomo_custom_dimension resource"
```

---

### Task 14: `matomo_tagmanager_container` resource

**Files:**
- Create: `internal/provider/resource_tagmanager_container.go`
- Test: `internal/provider/resource_tagmanager_container_test.go`
- Modify: `internal/provider/provider.go` (register in `Resources()`)

**Interfaces:**
- Consumes: `matomo.Client.{AddContainer,UpdateContainer,DeleteContainer,GetContainer}` (Task 5); `buildContainerID`/`parseContainerID` (Task 10).
- Produces: `func NewTagManagerContainerResource() resource.Resource`.

- [ ] **Step 1: Write the failing test**

`internal/provider/resource_tagmanager_container_test.go`:
```go
package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTagManagerContainerResource_basic(t *testing.T) {
	containers := map[string]map[string]any{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("method") {
		case "TagManager.addContainer":
			id := "abc123"
			containers[id] = map[string]any{
				"idcontainer": id, "idsite": r.URL.Query().Get("idSite"),
				"context": r.URL.Query().Get("context"), "name": r.URL.Query().Get("name"),
				"description": r.URL.Query().Get("description"),
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"idcontainer": id})
		case "TagManager.updateContainer":
			id := r.URL.Query().Get("idContainer")
			containers[id]["name"] = r.URL.Query().Get("name")
			containers[id]["description"] = r.URL.Query().Get("description")
			_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
		case "TagManager.deleteContainer":
			delete(containers, r.URL.Query().Get("idContainer"))
			_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
		case "TagManager.getContainer":
			id := r.URL.Query().Get("idContainer")
			ct, ok := containers[id]
			if !ok {
				_ = json.NewEncoder(w).Encode(map[string]any{"result": "error", "message": "Container does not exist"})
				return
			}
			_ = json.NewEncoder(w).Encode(ct)
		default:
			t.Fatalf("unexpected method %q", r.URL.Query().Get("method"))
		}
	}))
	defer srv.Close()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: newTestProviderFactories(srv),
		Steps: []resource.TestStep{
			{
				Config: testAccPreCheckConfig(srv) + `
resource "matomo_tagmanager_container" "test" {
  site_id = "3"
  context = "web"
  name    = "Main"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("matomo_tagmanager_container.test", "id", "3/abc123"),
					resource.TestCheckResourceAttr("matomo_tagmanager_container.test", "context", "web"),
				),
			},
			{
				Config: testAccPreCheckConfig(srv) + `
resource "matomo_tagmanager_container" "test" {
  site_id = "3"
  context = "web"
  name    = "Renamed"
}
`,
				Check: resource.TestCheckResourceAttr("matomo_tagmanager_container.test", "name", "Renamed"),
			},
		},
	})
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/provider/... -run TestAccTagManagerContainerResource_basic -v`
Expected: FAIL — compile error, `resource_tagmanager_container.go` does not exist yet.

- [ ] **Step 3: Implement the resource**

`internal/provider/resource_tagmanager_container.go`:
```go
package provider

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

var (
	_ resource.Resource                = &tagManagerContainerResource{}
	_ resource.ResourceWithConfigure   = &tagManagerContainerResource{}
	_ resource.ResourceWithImportState = &tagManagerContainerResource{}
)

func NewTagManagerContainerResource() resource.Resource {
	return &tagManagerContainerResource{}
}

type tagManagerContainerResource struct {
	client *matomo.Client
}

type tagManagerContainerResourceModel struct {
	ID          types.String `tfsdk:"id"`
	SiteID      types.String `tfsdk:"site_id"`
	Context     types.String `tfsdk:"context"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
}

func (r *tagManagerContainerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tagmanager_container"
}

func (r *tagManagerContainerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Composite \"site_id/container_id\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"site_id": schema.StringAttribute{
				Required:    true,
				Description: "The owning site's id (matomo_site.x.id).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"context": schema.StringAttribute{
				Required:    true,
				Description: "\"web\", \"android\", or \"ios\".",
				Validators: []validator.String{
					stringvalidator.OneOf("web", "android", "ios"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The container's name.",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The container's description.",
			},
		},
	}
}

func (r *tagManagerContainerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*matomo.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", "expected *matomo.Client")
		return
	}
	r.client = client
}

func (r *tagManagerContainerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan tagManagerContainerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, err := strconv.Atoi(plan.SiteID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid site_id", err.Error())
		return
	}
	description := ""
	if !plan.Description.IsUnknown() && !plan.Description.IsNull() {
		description = plan.Description.ValueString()
	}

	idContainer, err := r.client.AddContainer(ctx, siteID, plan.Context.ValueString(), plan.Name.ValueString(), description)
	if err != nil {
		resp.Diagnostics.AddError("Error creating Matomo Tag Manager container", err.Error())
		return
	}

	plan.ID = types.StringValue(buildContainerID(siteID, idContainer))
	plan.Description = types.StringValue(description)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *tagManagerContainerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state tagManagerContainerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, err := parseContainerID(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id in state", err.Error())
		return
	}

	ct, err := r.client.GetContainer(ctx, siteID, idContainer)
	if err != nil {
		if apiErr, ok := err.(*matomo.APIError); ok && apiErr.Message == "Container does not exist" {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Matomo Tag Manager container", err.Error())
		return
	}

	state.SiteID = types.StringValue(strconv.Itoa(siteID))
	state.Context = types.StringValue(ct.Context)
	state.Name = types.StringValue(ct.Name)
	state.Description = types.StringValue(ct.Description)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *tagManagerContainerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan tagManagerContainerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, err := parseContainerID(plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id in state", err.Error())
		return
	}

	description := ""
	if !plan.Description.IsUnknown() && !plan.Description.IsNull() {
		description = plan.Description.ValueString()
	}

	if err := r.client.UpdateContainer(ctx, siteID, idContainer, plan.Name.ValueString(), description); err != nil {
		resp.Diagnostics.AddError("Error updating Matomo Tag Manager container", err.Error())
		return
	}

	plan.Description = types.StringValue(description)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *tagManagerContainerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state tagManagerContainerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, err := parseContainerID(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id in state", err.Error())
		return
	}

	if err := r.client.DeleteContainer(ctx, siteID, idContainer); err != nil {
		resp.Diagnostics.AddError("Error deleting Matomo Tag Manager container", err.Error())
	}
}

func (r *tagManagerContainerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
```

- [ ] **Step 4: Register the resource in the provider**

In `internal/provider/provider.go`, add `NewTagManagerContainerResource` to
the slice returned by `Resources()`.

- [ ] **Step 5: Run the test to verify it passes**

Run: `go test ./internal/provider/... -run TestAccTagManagerContainerResource_basic -v`
Expected: `PASS`.

- [ ] **Step 6: Commit**

```bash
git add internal/provider/provider.go internal/provider/resource_tagmanager_container.go internal/provider/resource_tagmanager_container_test.go
git commit -m "feat(provider): add matomo_tagmanager_container resource"
```

---

### Task 15: Draft version resolver

Implements spec §6.4's rule that tag/trigger/variable resources always
target the container's draft version, never exposing a version ID.

**Files:**
- Create: `internal/provider/draft_version.go`
- Test: `internal/provider/draft_version_test.go`

**Interfaces:**
- Consumes: `matomo.Client.GetContainerVersions` (Task 6).
- Produces: `func resolveDraftVersionID(ctx context.Context, client *matomo.Client, siteID int, idContainer string) (string, error)` — used by Tasks 16-18.

- [ ] **Step 1: Write the failing tests**

`internal/provider/draft_version_test.go`:
```go
package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

func TestResolveDraftVersionID_found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"idcontainerversion": "1", "name": "Draft", "isDraft": true},
			{"idcontainerversion": "2", "name": "v1", "isDraft": false},
		})
	}))
	defer srv.Close()

	client := matomo.NewClient(srv.URL, "test-token", srv.Client())
	id, err := resolveDraftVersionID(context.Background(), client, 3, "abc123")
	if err != nil {
		t.Fatalf("resolveDraftVersionID() error = %v", err)
	}
	if id != "1" {
		t.Errorf("id = %q, want 1", id)
	}
}

func TestResolveDraftVersionID_notFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"idcontainerversion": "2", "name": "v1", "isDraft": false},
		})
	}))
	defer srv.Close()

	client := matomo.NewClient(srv.URL, "test-token", srv.Client())
	_, err := resolveDraftVersionID(context.Background(), client, 3, "abc123")
	if err == nil {
		t.Fatal("resolveDraftVersionID() error = nil, want error (no draft version found)")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/provider/... -run TestResolveDraftVersionID -v`
Expected: FAIL — compile error, `draft_version.go` does not exist yet.

- [ ] **Step 3: Write the implementation**

`internal/provider/draft_version.go`:
```go
package provider

import (
	"context"
	"fmt"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

// resolveDraftVersionID returns the id of a container's mutable draft
// version. matomo_tagmanager_tag/_trigger/_variable resources always write
// to the draft; users never see or set a version id directly.
func resolveDraftVersionID(ctx context.Context, client *matomo.Client, siteID int, idContainer string) (string, error) {
	versions, err := client.GetContainerVersions(ctx, siteID, idContainer)
	if err != nil {
		return "", fmt.Errorf("listing container versions: %w", err)
	}
	for _, v := range versions {
		if v.IsDraft {
			return v.IDContainerVersion, nil
		}
	}
	return "", fmt.Errorf("no draft version found for container %q (site %d) — every Tag Manager container should have one; this likely indicates the container was deleted out of band", idContainer, siteID)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/provider/... -run TestResolveDraftVersionID -v`
Expected: both tests `PASS`.

- [ ] **Step 5: Commit**

```bash
git add internal/provider/draft_version.go internal/provider/draft_version_test.go
git commit -m "feat(provider): add draft container version resolver"
```

---

### Task 16: `matomo_tagmanager_tag` generic resource

The largest resource in this plan: generic `type` + `parameter{}` form from
spec §6.4, draft-version targeting (Task 15), composite trigger-ID
references resolved against the same container, and `status` driving
pause/resume.

**Files:**
- Create: `internal/provider/resource_tagmanager_tag.go`
- Test: `internal/provider/resource_tagmanager_tag_test.go`
- Modify: `internal/provider/provider.go` (register in `Resources()`)

**Interfaces:**
- Consumes: `matomo.Client.{AddContainerTag,UpdateContainerTag,DeleteContainerTag,GetContainerTag,PauseContainerTag,ResumeContainerTag}` (Task 7); `resolveDraftVersionID` (Task 15); `buildEntityID`/`parseEntityID`, `parseContainerID` (Task 10).
- Produces: `func NewTagManagerTagResource() resource.Resource`; the same `entityIDsFromComposite`/`compositeIDsFromEntity` helper pair this task adds to `internal/provider/ids.go` is reused unmodified by Tasks 17-18 for trigger/variable cross-references.

- [ ] **Step 1: Write the failing test for the new ID helpers**

These helpers convert between a tag's `fire_trigger_ids`/`block_trigger_ids`
(each a full composite trigger `.id`, e.g. `"3/abc123/7"`) and the bare IDs
(`"7"`) Matomo's API expects, validating every referenced entity belongs to
the same container as the tag itself.

Append to `internal/provider/ids_test.go`:
```go
func TestBareCompositeEntityIDs_roundTrip(t *testing.T) {
	composite := []string{buildEntityID(3, "abc123", "7"), buildEntityID(3, "abc123", "8")}
	bare, err := bareEntityIDs(3, "abc123", composite)
	if err != nil {
		t.Fatalf("bareEntityIDs() error = %v", err)
	}
	if len(bare) != 2 || bare[0] != "7" || bare[1] != "8" {
		t.Errorf("bare = %v, want [7 8]", bare)
	}
	roundTripped := compositeEntityIDs(3, "abc123", bare)
	if roundTripped[0] != composite[0] || roundTripped[1] != composite[1] {
		t.Errorf("roundTripped = %v, want %v", roundTripped, composite)
	}
}

func TestBareEntityIDs_wrongContainer(t *testing.T) {
	_, err := bareEntityIDs(3, "abc123", []string{buildEntityID(3, "other-container", "7")})
	if err == nil {
		t.Fatal("bareEntityIDs() error = nil, want error (cross-container reference)")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/provider/... -run 'TestBareCompositeEntityIDs|TestBareEntityIDs' -v`
Expected: FAIL — compile error, `bareEntityIDs`/`compositeEntityIDs` do not exist yet.

- [ ] **Step 3: Implement the ID helpers**

Append to `internal/provider/ids.go`:
```go
// bareEntityIDs converts composite entity ids (site/container/entity) to
// bare entity ids, verifying every one belongs to the given container.
func bareEntityIDs(siteID int, idContainer string, compositeIDs []string) ([]string, error) {
	bare := make([]string, 0, len(compositeIDs))
	for _, composite := range compositeIDs {
		entitySiteID, entityContainer, entityID, err := parseEntityID(composite)
		if err != nil {
			return nil, fmt.Errorf("invalid reference %q: %w", composite, err)
		}
		if entitySiteID != siteID || entityContainer != idContainer {
			return nil, fmt.Errorf("reference %q belongs to a different container than site %d / container %q", composite, siteID, idContainer)
		}
		bare = append(bare, entityID)
	}
	return bare, nil
}

// compositeEntityIDs is the inverse of bareEntityIDs, used when reading
// Matomo's response back into state.
func compositeEntityIDs(siteID int, idContainer string, bareIDs []string) []string {
	composite := make([]string, 0, len(bareIDs))
	for _, id := range bareIDs {
		composite = append(composite, buildEntityID(siteID, idContainer, id))
	}
	return composite
}
```
Add `"fmt"` to `internal/provider/ids.go`'s imports if not already present
(it is, from Task 10).

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/provider/... -run 'TestBareCompositeEntityIDs|TestBareEntityIDs' -v`
Expected: both `PASS`.

- [ ] **Step 5: Write the failing resource test**

`internal/provider/resource_tagmanager_tag_test.go`:
```go
package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTagManagerTagResource_basic(t *testing.T) {
	tags := map[string]map[string]any{}
	nextID := 1

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("method") {
		case "TagManager.getContainerVersions":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"idcontainerversion": "1", "name": "Draft", "isDraft": true},
			})
		case "TagManager.addContainerTag":
			id := nextID
			nextID++
			idStr := strconv.Itoa(id)
			tags[idStr] = map[string]any{
				"idtag": idStr, "name": r.URL.Query().Get("name"), "type": r.URL.Query().Get("type"),
				"status": "active", "parameters": map[string]any{"customHtml": "<script></script>"},
				"fireTriggerIds": []string{}, "blockTriggerIds": []string{},
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"idtag": idStr})
		case "TagManager.updateContainerTag":
			id := r.URL.Query().Get("idTag")
			tags[id]["name"] = r.URL.Query().Get("name")
			_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
		case "TagManager.pauseContainerTag":
			tags[r.URL.Query().Get("idTag")]["status"] = "paused"
			_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
		case "TagManager.resumeContainerTag":
			tags[r.URL.Query().Get("idTag")]["status"] = "active"
			_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
		case "TagManager.deleteContainerTag":
			delete(tags, r.URL.Query().Get("idTag"))
			_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
		case "TagManager.getContainerTag":
			id := r.URL.Query().Get("idTag")
			tag, ok := tags[id]
			if !ok {
				_ = json.NewEncoder(w).Encode(map[string]any{"result": "error", "message": "Tag does not exist"})
				return
			}
			_ = json.NewEncoder(w).Encode(tag)
		default:
			t.Fatalf("unexpected method %q", r.URL.Query().Get("method"))
		}
	}))
	defer srv.Close()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: newTestProviderFactories(srv),
		Steps: []resource.TestStep{
			{
				Config: testAccPreCheckConfig(srv) + `
resource "matomo_tagmanager_tag" "test" {
  container_id = "3/abc123"
  type         = "CustomHtml"
  name         = "My tag"
  parameter {
    name  = "customHtml"
    value = "<script></script>"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("matomo_tagmanager_tag.test", "id", "3/abc123/1"),
					resource.TestCheckResourceAttr("matomo_tagmanager_tag.test", "status", "active"),
				),
			},
			{
				Config: testAccPreCheckConfig(srv) + `
resource "matomo_tagmanager_tag" "test" {
  container_id = "3/abc123"
  type         = "CustomHtml"
  name         = "My tag"
  status       = "paused"
  parameter {
    name  = "customHtml"
    value = "<script></script>"
  }
}
`,
				Check: resource.TestCheckResourceAttr("matomo_tagmanager_tag.test", "status", "paused"),
			},
		},
	})
}

```

- [ ] **Step 6: Run the test to verify it fails**

Run: `go test ./internal/provider/... -run TestAccTagManagerTagResource_basic -v`
Expected: FAIL — compile error, `resource_tagmanager_tag.go` does not exist yet.

- [ ] **Step 7: Implement the resource**

`internal/provider/resource_tagmanager_tag.go`:
```go
package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

var (
	_ resource.Resource                = &tagManagerTagResource{}
	_ resource.ResourceWithConfigure   = &tagManagerTagResource{}
	_ resource.ResourceWithImportState = &tagManagerTagResource{}
)

func NewTagManagerTagResource() resource.Resource {
	return &tagManagerTagResource{}
}

type tagManagerTagResource struct {
	client *matomo.Client
}

type tagParameterModel struct {
	Name  types.String `tfsdk:"name"`
	Value types.String `tfsdk:"value"`
}

type tagManagerTagResourceModel struct {
	ID              types.String        `tfsdk:"id"`
	ContainerID     types.String        `tfsdk:"container_id"`
	Type            types.String        `tfsdk:"type"`
	Name            types.String        `tfsdk:"name"`
	Status          types.String        `tfsdk:"status"`
	FireTriggerIDs  []types.String      `tfsdk:"fire_trigger_ids"`
	BlockTriggerIDs []types.String      `tfsdk:"block_trigger_ids"`
	Parameter       []tagParameterModel `tfsdk:"parameter"`
}

func (r *tagManagerTagResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tagmanager_tag"
}

func (r *tagManagerTagResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Composite \"site_id/container_id/tag_id\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"container_id": schema.StringAttribute{
				Required:    true,
				Description: "The owning container's id (matomo_tagmanager_container.x.id).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"type": schema.StringAttribute{
				Required:    true,
				Description: "The Matomo tag type, e.g. \"CustomHtml\". See the matomo_tagmanager_tag_types data source for valid values.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The tag's display name.",
			},
			"status": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "\"active\" or \"paused\". Changing this edits the draft version only — like every other field on this resource, it has no effect on a live container until a new version is created and published.",
				Validators: []validator.String{
					stringvalidator.OneOf("active", "paused"),
				},
			},
			"fire_trigger_ids": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Trigger ids (matomo_tagmanager_trigger.x.id) that fire this tag.",
			},
			"block_trigger_ids": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Trigger ids (matomo_tagmanager_trigger.x.id) that block this tag from firing.",
			},
		},
		Blocks: map[string]schema.Block{
			"parameter": schema.ListNestedBlock{
				Description: "Type-specific configuration as name/value pairs.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name":  schema.StringAttribute{Required: true},
						"value": schema.StringAttribute{Required: true},
					},
				},
			},
		},
	}
}

func (r *tagManagerTagResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*matomo.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", "expected *matomo.Client")
		return
	}
	r.client = client
}

func parametersToMap(params []tagParameterModel) map[string]string {
	m := make(map[string]string, len(params))
	for _, p := range params {
		m[p.Name.ValueString()] = p.Value.ValueString()
	}
	return m
}

func stringSliceFromModel(in []types.String) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = v.ValueString()
	}
	return out
}

func stringModelFromSlice(in []string) []types.String {
	out := make([]types.String, len(in))
	for i, v := range in {
		out[i] = types.StringValue(v)
	}
	return out
}

func (r *tagManagerTagResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan tagManagerTagResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, err := parseContainerID(plan.ContainerID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid container_id", err.Error())
		return
	}

	versionID, err := resolveDraftVersionID(ctx, r.client, siteID, idContainer)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving draft container version", err.Error())
		return
	}

	fireIDs, err := bareEntityIDs(siteID, idContainer, stringSliceFromModel(plan.FireTriggerIDs))
	if err != nil {
		resp.Diagnostics.AddError("Invalid fire_trigger_ids", err.Error())
		return
	}
	blockIDs, err := bareEntityIDs(siteID, idContainer, stringSliceFromModel(plan.BlockTriggerIDs))
	if err != nil {
		resp.Diagnostics.AddError("Invalid block_trigger_ids", err.Error())
		return
	}

	idTag, err := r.client.AddContainerTag(ctx, siteID, idContainer, versionID, matomo.TagParams{
		Type:            plan.Type.ValueString(),
		Name:            plan.Name.ValueString(),
		Parameters:      parametersToMap(plan.Parameter),
		FireTriggerIDs:  fireIDs,
		BlockTriggerIDs: blockIDs,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating Matomo Tag Manager tag", err.Error())
		return
	}

	status := "active"
	if !plan.Status.IsUnknown() && !plan.Status.IsNull() {
		status = plan.Status.ValueString()
	}
	if status == "paused" {
		if err := r.client.PauseContainerTag(ctx, siteID, idContainer, versionID, idTag); err != nil {
			resp.Diagnostics.AddError("Error pausing Matomo Tag Manager tag", err.Error())
			return
		}
	}

	plan.ID = types.StringValue(buildEntityID(siteID, idContainer, idTag))
	plan.Status = types.StringValue(status)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *tagManagerTagResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state tagManagerTagResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, idTag, err := parseEntityID(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id in state", err.Error())
		return
	}

	versionID, err := resolveDraftVersionID(ctx, r.client, siteID, idContainer)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving draft container version", err.Error())
		return
	}

	tag, err := r.client.GetContainerTag(ctx, siteID, idContainer, versionID, idTag)
	if err != nil {
		if apiErr, ok := err.(*matomo.APIError); ok && apiErr.Message == "Tag does not exist" {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Matomo Tag Manager tag", err.Error())
		return
	}

	state.ContainerID = types.StringValue(buildContainerID(siteID, idContainer))
	state.Type = types.StringValue(tag.Type)
	state.Name = types.StringValue(tag.Name)
	state.Status = types.StringValue(tag.Status)
	state.FireTriggerIDs = stringModelFromSlice(compositeEntityIDs(siteID, idContainer, tag.FireTriggerIDs))
	state.BlockTriggerIDs = stringModelFromSlice(compositeEntityIDs(siteID, idContainer, tag.BlockTriggerIDs))

	params := make([]tagParameterModel, 0, len(tag.Parameters))
	for name, value := range tag.Parameters {
		params = append(params, tagParameterModel{Name: types.StringValue(name), Value: types.StringValue(value)})
	}
	state.Parameter = params

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *tagManagerTagResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan tagManagerTagResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, idTag, err := parseEntityID(plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id in state", err.Error())
		return
	}

	versionID, err := resolveDraftVersionID(ctx, r.client, siteID, idContainer)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving draft container version", err.Error())
		return
	}

	fireIDs, err := bareEntityIDs(siteID, idContainer, stringSliceFromModel(plan.FireTriggerIDs))
	if err != nil {
		resp.Diagnostics.AddError("Invalid fire_trigger_ids", err.Error())
		return
	}
	blockIDs, err := bareEntityIDs(siteID, idContainer, stringSliceFromModel(plan.BlockTriggerIDs))
	if err != nil {
		resp.Diagnostics.AddError("Invalid block_trigger_ids", err.Error())
		return
	}

	if err := r.client.UpdateContainerTag(ctx, siteID, idContainer, versionID, idTag, matomo.TagParams{
		Type:            plan.Type.ValueString(),
		Name:            plan.Name.ValueString(),
		Parameters:      parametersToMap(plan.Parameter),
		FireTriggerIDs:  fireIDs,
		BlockTriggerIDs: blockIDs,
	}); err != nil {
		resp.Diagnostics.AddError("Error updating Matomo Tag Manager tag", err.Error())
		return
	}

	status := "active"
	if !plan.Status.IsUnknown() && !plan.Status.IsNull() {
		status = plan.Status.ValueString()
	}
	if status == "paused" {
		err = r.client.PauseContainerTag(ctx, siteID, idContainer, versionID, idTag)
	} else {
		err = r.client.ResumeContainerTag(ctx, siteID, idContainer, versionID, idTag)
	}
	if err != nil {
		resp.Diagnostics.AddError("Error updating Matomo Tag Manager tag status", err.Error())
		return
	}

	plan.Status = types.StringValue(status)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *tagManagerTagResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state tagManagerTagResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, idTag, err := parseEntityID(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id in state", err.Error())
		return
	}

	versionID, err := resolveDraftVersionID(ctx, r.client, siteID, idContainer)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving draft container version", err.Error())
		return
	}

	if err := r.client.DeleteContainerTag(ctx, siteID, idContainer, versionID, idTag); err != nil {
		resp.Diagnostics.AddError("Error deleting Matomo Tag Manager tag", err.Error())
	}
}

func (r *tagManagerTagResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
```

- [ ] **Step 8: Register the resource in the provider**

In `internal/provider/provider.go`, add `NewTagManagerTagResource` to the
slice returned by `Resources()`.

- [ ] **Step 9: Run the test to verify it passes**

Run: `go test ./internal/provider/... -run TestAccTagManagerTagResource_basic -v`
Expected: `PASS` (both steps — create with status=active, update to status=paused).

- [ ] **Step 10: Commit**

```bash
git add internal/provider/ids.go internal/provider/ids_test.go internal/provider/provider.go internal/provider/resource_tagmanager_tag.go internal/provider/resource_tagmanager_tag_test.go
git commit -m "feat(provider): add matomo_tagmanager_tag generic resource"
```

---

### Task 17: `matomo_tagmanager_trigger` generic resource

Trigger `conditions.actual` is a Matomo "actual value" identifier (e.g.
`"url_path"`) or a variable macro reference (e.g. `"{{My Variable}}"`) — a
free-form string, not a numeric variable ID — so unlike Task 16's
`fire_trigger_ids`, conditions need no composite-ID translation.

**Files:**
- Create: `internal/provider/resource_tagmanager_trigger.go`
- Test: `internal/provider/resource_tagmanager_trigger_test.go`
- Modify: `internal/provider/provider.go` (register in `Resources()`)

**Interfaces:**
- Consumes: `matomo.Client.{AddContainerTrigger,UpdateContainerTrigger,DeleteContainerTrigger,GetContainerTrigger}` (Task 8); `resolveDraftVersionID` (Task 15); `buildEntityID`/`parseEntityID`, `buildContainerID`/`parseContainerID` (Task 10).
- Produces: `func NewTagManagerTriggerResource() resource.Resource`.

- [ ] **Step 1: Write the failing test**

`internal/provider/resource_tagmanager_trigger_test.go`:
```go
package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTagManagerTriggerResource_basic(t *testing.T) {
	triggers := map[string]map[string]any{}
	nextID := 1

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("method") {
		case "TagManager.getContainerVersions":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"idcontainerversion": "1", "name": "Draft", "isDraft": true},
			})
		case "TagManager.addContainerTrigger":
			id := nextID
			nextID++
			idStr := strconv.Itoa(id)
			triggers[idStr] = map[string]any{
				"idtrigger": idStr, "name": r.URL.Query().Get("name"), "type": r.URL.Query().Get("type"),
				"parameters": map[string]any{},
				"conditions": []map[string]any{{"comparison": "equals", "actual": "url_path", "value": "/checkout"}},
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"idtrigger": idStr})
		case "TagManager.updateContainerTrigger":
			id := r.URL.Query().Get("idTrigger")
			triggers[id]["name"] = r.URL.Query().Get("name")
			_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
		case "TagManager.deleteContainerTrigger":
			delete(triggers, r.URL.Query().Get("idTrigger"))
			_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
		case "TagManager.getContainerTrigger":
			id := r.URL.Query().Get("idTrigger")
			trig, ok := triggers[id]
			if !ok {
				_ = json.NewEncoder(w).Encode(map[string]any{"result": "error", "message": "Trigger does not exist"})
				return
			}
			_ = json.NewEncoder(w).Encode(trig)
		default:
			t.Fatalf("unexpected method %q", r.URL.Query().Get("method"))
		}
	}))
	defer srv.Close()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: newTestProviderFactories(srv),
		Steps: []resource.TestStep{
			{
				Config: testAccPreCheckConfig(srv) + `
resource "matomo_tagmanager_trigger" "test" {
  container_id = "3/abc123"
  type         = "PageView"
  name         = "Checkout page"
  condition {
    comparison = "equals"
    actual     = "url_path"
    value      = "/checkout"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("matomo_tagmanager_trigger.test", "id", "3/abc123/1"),
					resource.TestCheckResourceAttr("matomo_tagmanager_trigger.test", "condition.0.comparison", "equals"),
				),
			},
		},
	})
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/provider/... -run TestAccTagManagerTriggerResource_basic -v`
Expected: FAIL — compile error, `resource_tagmanager_trigger.go` does not exist yet.

- [ ] **Step 3: Implement the resource**

`internal/provider/resource_tagmanager_trigger.go`:
```go
package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

var (
	_ resource.Resource                = &tagManagerTriggerResource{}
	_ resource.ResourceWithConfigure   = &tagManagerTriggerResource{}
	_ resource.ResourceWithImportState = &tagManagerTriggerResource{}
)

func NewTagManagerTriggerResource() resource.Resource {
	return &tagManagerTriggerResource{}
}

type tagManagerTriggerResource struct {
	client *matomo.Client
}

type triggerConditionModel struct {
	Comparison types.String `tfsdk:"comparison"`
	Actual     types.String `tfsdk:"actual"`
	Value      types.String `tfsdk:"value"`
}

type tagManagerTriggerResourceModel struct {
	ID          types.String             `tfsdk:"id"`
	ContainerID types.String             `tfsdk:"container_id"`
	Type        types.String             `tfsdk:"type"`
	Name        types.String             `tfsdk:"name"`
	Parameter   []tagParameterModel      `tfsdk:"parameter"`
	Condition   []triggerConditionModel  `tfsdk:"condition"`
}

func (r *tagManagerTriggerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tagmanager_trigger"
}

func (r *tagManagerTriggerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Composite \"site_id/container_id/trigger_id\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"container_id": schema.StringAttribute{
				Required:    true,
				Description: "The owning container's id (matomo_tagmanager_container.x.id).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"type": schema.StringAttribute{
				Required:    true,
				Description: "The Matomo trigger type, e.g. \"PageView\". See the matomo_tagmanager_trigger_types data source for valid values.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The trigger's display name.",
			},
		},
		Blocks: map[string]schema.Block{
			"parameter": schema.ListNestedBlock{
				Description: "Type-specific configuration as name/value pairs.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name":  schema.StringAttribute{Required: true},
						"value": schema.StringAttribute{Required: true},
					},
				},
			},
			"condition": schema.ListNestedBlock{
				Description: "Conditions that must all match for this trigger to fire.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"comparison": schema.StringAttribute{Required: true},
						"actual":     schema.StringAttribute{Required: true, Description: "A Matomo \"actual value\" identifier (e.g. \"url_path\") or a variable macro reference (e.g. \"{{My Variable}}\")."},
						"value":      schema.StringAttribute{Required: true},
					},
				},
			},
		},
	}
}

func (r *tagManagerTriggerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*matomo.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", "expected *matomo.Client")
		return
	}
	r.client = client
}

func conditionsToParams(conditions []triggerConditionModel) []matomo.Condition {
	out := make([]matomo.Condition, len(conditions))
	for i, c := range conditions {
		out[i] = matomo.Condition{
			Comparison:            c.Comparison.ValueString(),
			ActualValueVariableID: c.Actual.ValueString(),
			ExpectedValue:         c.Value.ValueString(),
		}
	}
	return out
}

func conditionsFromAPI(conditions []matomo.Condition) []triggerConditionModel {
	out := make([]triggerConditionModel, len(conditions))
	for i, c := range conditions {
		out[i] = triggerConditionModel{
			Comparison: types.StringValue(c.Comparison),
			Actual:     types.StringValue(c.ActualValueVariableID),
			Value:      types.StringValue(c.ExpectedValue),
		}
	}
	return out
}

func (r *tagManagerTriggerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan tagManagerTriggerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, err := parseContainerID(plan.ContainerID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid container_id", err.Error())
		return
	}

	versionID, err := resolveDraftVersionID(ctx, r.client, siteID, idContainer)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving draft container version", err.Error())
		return
	}

	idTrigger, err := r.client.AddContainerTrigger(ctx, siteID, idContainer, versionID, matomo.TriggerParams{
		Type:       plan.Type.ValueString(),
		Name:       plan.Name.ValueString(),
		Parameters: parametersToMap(plan.Parameter),
		Conditions: conditionsToParams(plan.Condition),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating Matomo Tag Manager trigger", err.Error())
		return
	}

	plan.ID = types.StringValue(buildEntityID(siteID, idContainer, idTrigger))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *tagManagerTriggerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state tagManagerTriggerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, idTrigger, err := parseEntityID(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id in state", err.Error())
		return
	}

	versionID, err := resolveDraftVersionID(ctx, r.client, siteID, idContainer)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving draft container version", err.Error())
		return
	}

	trig, err := r.client.GetContainerTrigger(ctx, siteID, idContainer, versionID, idTrigger)
	if err != nil {
		if apiErr, ok := err.(*matomo.APIError); ok && apiErr.Message == "Trigger does not exist" {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Matomo Tag Manager trigger", err.Error())
		return
	}

	state.ContainerID = types.StringValue(buildContainerID(siteID, idContainer))
	state.Type = types.StringValue(trig.Type)
	state.Name = types.StringValue(trig.Name)
	state.Condition = conditionsFromAPI(trig.Conditions)

	params := make([]tagParameterModel, 0, len(trig.Parameters))
	for name, value := range trig.Parameters {
		params = append(params, tagParameterModel{Name: types.StringValue(name), Value: types.StringValue(value)})
	}
	state.Parameter = params

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *tagManagerTriggerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan tagManagerTriggerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, idTrigger, err := parseEntityID(plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id in state", err.Error())
		return
	}

	versionID, err := resolveDraftVersionID(ctx, r.client, siteID, idContainer)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving draft container version", err.Error())
		return
	}

	if err := r.client.UpdateContainerTrigger(ctx, siteID, idContainer, versionID, idTrigger, matomo.TriggerParams{
		Type:       plan.Type.ValueString(),
		Name:       plan.Name.ValueString(),
		Parameters: parametersToMap(plan.Parameter),
		Conditions: conditionsToParams(plan.Condition),
	}); err != nil {
		resp.Diagnostics.AddError("Error updating Matomo Tag Manager trigger", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *tagManagerTriggerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state tagManagerTriggerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, idTrigger, err := parseEntityID(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id in state", err.Error())
		return
	}

	versionID, err := resolveDraftVersionID(ctx, r.client, siteID, idContainer)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving draft container version", err.Error())
		return
	}

	if err := r.client.DeleteContainerTrigger(ctx, siteID, idContainer, versionID, idTrigger); err != nil {
		resp.Diagnostics.AddError("Error deleting Matomo Tag Manager trigger", err.Error())
	}
}

func (r *tagManagerTriggerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
```

- [ ] **Step 4: Register the resource in the provider**

In `internal/provider/provider.go`, add `NewTagManagerTriggerResource` to
the slice returned by `Resources()`.

- [ ] **Step 5: Run the test to verify it passes**

Run: `go test ./internal/provider/... -run TestAccTagManagerTriggerResource_basic -v`
Expected: `PASS`.

- [ ] **Step 6: Commit**

```bash
git add internal/provider/provider.go internal/provider/resource_tagmanager_trigger.go internal/provider/resource_tagmanager_trigger_test.go
git commit -m "feat(provider): add matomo_tagmanager_trigger generic resource"
```

---

### Task 18: `matomo_tagmanager_variable` generic resource

Same shape as Tasks 16-17, simpler: no trigger-ID or condition handling,
just `type` + `parameter{}` + an optional `default_value`.

**Files:**
- Create: `internal/provider/resource_tagmanager_variable.go`
- Test: `internal/provider/resource_tagmanager_variable_test.go`
- Modify: `internal/provider/provider.go` (register in `Resources()`)

**Interfaces:**
- Consumes: `matomo.Client.{AddContainerVariable,UpdateContainerVariable,DeleteContainerVariable,GetContainerVariable}` (Task 9); `resolveDraftVersionID` (Task 15); `buildEntityID`/`parseEntityID`, `buildContainerID`/`parseContainerID` (Task 10); `parametersToMap`/`tagParameterModel` (Task 16, reused as-is — same `parameter { name, value }` shape).
- Produces: `func NewTagManagerVariableResource() resource.Resource`.

- [ ] **Step 1: Write the failing test**

`internal/provider/resource_tagmanager_variable_test.go`:
```go
package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTagManagerVariableResource_basic(t *testing.T) {
	variables := map[string]map[string]any{}
	nextID := 1

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("method") {
		case "TagManager.getContainerVersions":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"idcontainerversion": "1", "name": "Draft", "isDraft": true},
			})
		case "TagManager.addContainerVariable":
			id := nextID
			nextID++
			idStr := strconv.Itoa(id)
			variables[idStr] = map[string]any{
				"idvariable": idStr, "name": r.URL.Query().Get("name"), "type": r.URL.Query().Get("type"),
				"parameters": map[string]any{}, "defaultValue": r.URL.Query().Get("defaultValue"),
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"idvariable": idStr})
		case "TagManager.updateContainerVariable":
			id := r.URL.Query().Get("idVariable")
			variables[id]["name"] = r.URL.Query().Get("name")
			_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
		case "TagManager.deleteContainerVariable":
			delete(variables, r.URL.Query().Get("idVariable"))
			_ = json.NewEncoder(w).Encode(map[string]any{"value": true})
		case "TagManager.getContainerVariable":
			id := r.URL.Query().Get("idVariable")
			v, ok := variables[id]
			if !ok {
				_ = json.NewEncoder(w).Encode(map[string]any{"result": "error", "message": "Variable does not exist"})
				return
			}
			_ = json.NewEncoder(w).Encode(v)
		default:
			t.Fatalf("unexpected method %q", r.URL.Query().Get("method"))
		}
	}))
	defer srv.Close()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: newTestProviderFactories(srv),
		Steps: []resource.TestStep{
			{
				Config: testAccPreCheckConfig(srv) + `
resource "matomo_tagmanager_variable" "test" {
  container_id  = "3/abc123"
  type          = "Constant"
  name          = "My var"
  default_value = "n/a"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("matomo_tagmanager_variable.test", "id", "3/abc123/1"),
					resource.TestCheckResourceAttr("matomo_tagmanager_variable.test", "default_value", "n/a"),
				),
			},
		},
	})
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/provider/... -run TestAccTagManagerVariableResource_basic -v`
Expected: FAIL — compile error, `resource_tagmanager_variable.go` does not exist yet.

- [ ] **Step 3: Implement the resource**

`internal/provider/resource_tagmanager_variable.go`:
```go
package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

var (
	_ resource.Resource                = &tagManagerVariableResource{}
	_ resource.ResourceWithConfigure   = &tagManagerVariableResource{}
	_ resource.ResourceWithImportState = &tagManagerVariableResource{}
)

func NewTagManagerVariableResource() resource.Resource {
	return &tagManagerVariableResource{}
}

type tagManagerVariableResource struct {
	client *matomo.Client
}

type tagManagerVariableResourceModel struct {
	ID           types.String        `tfsdk:"id"`
	ContainerID  types.String        `tfsdk:"container_id"`
	Type         types.String        `tfsdk:"type"`
	Name         types.String        `tfsdk:"name"`
	DefaultValue types.String        `tfsdk:"default_value"`
	Parameter    []tagParameterModel `tfsdk:"parameter"`
}

func (r *tagManagerVariableResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tagmanager_variable"
}

func (r *tagManagerVariableResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Composite \"site_id/container_id/variable_id\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"container_id": schema.StringAttribute{
				Required:    true,
				Description: "The owning container's id (matomo_tagmanager_container.x.id).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"type": schema.StringAttribute{
				Required:    true,
				Description: "The Matomo variable type, e.g. \"Constant\". See the matomo_tagmanager_variable_types data source for valid values.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The variable's display name.",
			},
			"default_value": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Value used when the variable cannot be resolved.",
			},
		},
		Blocks: map[string]schema.Block{
			"parameter": schema.ListNestedBlock{
				Description: "Type-specific configuration as name/value pairs.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name":  schema.StringAttribute{Required: true},
						"value": schema.StringAttribute{Required: true},
					},
				},
			},
		},
	}
}

func (r *tagManagerVariableResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*matomo.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", "expected *matomo.Client")
		return
	}
	r.client = client
}

func (r *tagManagerVariableResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan tagManagerVariableResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, err := parseContainerID(plan.ContainerID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid container_id", err.Error())
		return
	}

	versionID, err := resolveDraftVersionID(ctx, r.client, siteID, idContainer)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving draft container version", err.Error())
		return
	}

	var defaultValue *string
	if !plan.DefaultValue.IsUnknown() && !plan.DefaultValue.IsNull() {
		v := plan.DefaultValue.ValueString()
		defaultValue = &v
	}

	idVariable, err := r.client.AddContainerVariable(ctx, siteID, idContainer, versionID, matomo.VariableParams{
		Type:         plan.Type.ValueString(),
		Name:         plan.Name.ValueString(),
		Parameters:   parametersToMap(plan.Parameter),
		DefaultValue: defaultValue,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating Matomo Tag Manager variable", err.Error())
		return
	}

	plan.ID = types.StringValue(buildEntityID(siteID, idContainer, idVariable))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *tagManagerVariableResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state tagManagerVariableResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, idVariable, err := parseEntityID(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id in state", err.Error())
		return
	}

	versionID, err := resolveDraftVersionID(ctx, r.client, siteID, idContainer)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving draft container version", err.Error())
		return
	}

	v, err := r.client.GetContainerVariable(ctx, siteID, idContainer, versionID, idVariable)
	if err != nil {
		if apiErr, ok := err.(*matomo.APIError); ok && apiErr.Message == "Variable does not exist" {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Matomo Tag Manager variable", err.Error())
		return
	}

	state.ContainerID = types.StringValue(buildContainerID(siteID, idContainer))
	state.Type = types.StringValue(v.Type)
	state.Name = types.StringValue(v.Name)
	state.DefaultValue = types.StringValue(v.DefaultValue)

	params := make([]tagParameterModel, 0, len(v.Parameters))
	for name, value := range v.Parameters {
		params = append(params, tagParameterModel{Name: types.StringValue(name), Value: types.StringValue(value)})
	}
	state.Parameter = params

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *tagManagerVariableResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan tagManagerVariableResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, idVariable, err := parseEntityID(plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id in state", err.Error())
		return
	}

	versionID, err := resolveDraftVersionID(ctx, r.client, siteID, idContainer)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving draft container version", err.Error())
		return
	}

	var defaultValue *string
	if !plan.DefaultValue.IsUnknown() && !plan.DefaultValue.IsNull() {
		v := plan.DefaultValue.ValueString()
		defaultValue = &v
	}

	if err := r.client.UpdateContainerVariable(ctx, siteID, idContainer, versionID, idVariable, matomo.VariableParams{
		Type:         plan.Type.ValueString(),
		Name:         plan.Name.ValueString(),
		Parameters:   parametersToMap(plan.Parameter),
		DefaultValue: defaultValue,
	}); err != nil {
		resp.Diagnostics.AddError("Error updating Matomo Tag Manager variable", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *tagManagerVariableResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state tagManagerVariableResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, idVariable, err := parseEntityID(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id in state", err.Error())
		return
	}

	versionID, err := resolveDraftVersionID(ctx, r.client, siteID, idContainer)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving draft container version", err.Error())
		return
	}

	if err := r.client.DeleteContainerVariable(ctx, siteID, idContainer, versionID, idVariable); err != nil {
		resp.Diagnostics.AddError("Error deleting Matomo Tag Manager variable", err.Error())
	}
}

func (r *tagManagerVariableResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
```

- [ ] **Step 4: Register the resource in the provider**

In `internal/provider/provider.go`, add `NewTagManagerVariableResource` to
the slice returned by `Resources()`. The `Resources()` method should now
return all six: `NewSiteResource`, `NewCustomDimensionResource`,
`NewTagManagerContainerResource`, `NewTagManagerTagResource`,
`NewTagManagerTriggerResource`, `NewTagManagerVariableResource`. Likewise
`DataSources()` returns `NewSiteDataSource`.

- [ ] **Step 5: Run the test to verify it passes**

Run: `go test ./internal/provider/... -run TestAccTagManagerVariableResource_basic -v`
Expected: `PASS`.

- [ ] **Step 6: Commit**

```bash
git add internal/provider/provider.go internal/provider/resource_tagmanager_variable.go internal/provider/resource_tagmanager_variable_test.go
git commit -m "feat(provider): add matomo_tagmanager_variable generic resource"
```

---

### Task 19: Full verification pass

**Files:** none created/modified — this task only runs checks across
everything built in Tasks 1-18.

- [ ] **Step 1: Build**

Run: `go build -o /dev/null .`
Expected: exits 0, no output.

- [ ] **Step 2: Run the full test suite**

Run: `go test ./... -v -count=1`
Expected: every test across `internal/matomo` and `internal/provider`
`PASS`. This is the first point where the whole tree — client (Tasks 2-9),
ID helpers (Task 10), and all six resources + one data source (Tasks 11-18)
— is exercised together via the real provider wiring.

- [ ] **Step 3: Lint**

Run: `golangci-lint run ./...` (install via
`go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest` if
not already on `PATH`).
Expected: no issues. Fix anything it flags (most likely: unused imports
left over from the placeholder-fixing steps in Tasks 11 and 16) before
moving on.

- [ ] **Step 4: Push CI green**

Push the branch and confirm the `ci.yml` workflow from Task 1 (build, test,
lint) passes on GitHub Actions.

- [ ] **Step 5: Commit (if Step 3 required fixes)**

```bash
git add -A
git commit -m "chore: fix lint issues from foundation build-out"
```

If Step 3 found nothing to fix, skip this step — there is nothing to
commit.

---

## Out of scope for this plan

Per spec §12-13, the following are deliberately not part of this plan and
get their own plan(s) once this one ships:

- Typed per-type tag/trigger/variable resources and the `tools/gen`
  codegenerator (spec §6.4, phase 5).
- `matomo_tagmanager_{contexts,environments,tag_types,trigger_types,variable_types}`
  data sources (spec §8, phase 6).
- Actions: `create_container_version`, `publish_container_version`,
  `enable/disable_preview_mode` (spec §7, phase 7).
- `tfplugindocs` generation, `examples/`, `goreleaser`, registry submission
  (spec §10, phase 8).
- The `createContainerVersion`/`publishContainerVersion`/
  `enable/disablePreviewMode` Tag Manager client methods (only
  `GetContainerVersions` is needed by this plan; the rest belong with the
  actions plan above).
- `matomo_custom_dimension.extractions`.

