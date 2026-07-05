# Tag Manager Schema Completeness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `description` to tags/triggers/variables (typed and generic), `priority` to tags (typed and generic), and three boolean flags (`ignore_gtm_data_layer`, `actively_sync_gtm_data_layer`, `is_tag_fire_limit_allowed_in_preview_mode`) to `matomo_tagmanager_container` - all confirmed real, additive Matomo API fields this provider currently has no way to set.

**Architecture:** Client-layer struct/method changes in `internal/matomo`, then a shared-runtime change for the typed resources' common fields (`typedTagCommon`/`typedTriggerCommon`/`typedVariableCommon` in `internal/provider/typed_*_resource.go`) plus a `tools/gen` template change so every current and future generated `matomo_tagmanager_{tag,trigger,variable}_<type>` resource picks up the new fields automatically, then identical hand-wiring on the three generic fallback resources and the hand-written container resource. Because this sandbox cannot run `go run ./tools/gen` (no live Matomo/Docker - confirmed in a prior session), the 64 already-committed generated schema files and their 64 docs pages are updated via two small one-off Go scripts that insert the exact same text the updated template will emit for any future real regeneration, verified byte-identical via `gofmt`.

**Tech Stack:** Go, terraform-plugin-framework, Matomo's `TagManager` HTTP API.

## Global Constraints

- All three additions are purely additive - no existing field changes shape, name, or behavior.
- `description` (tags/triggers/variables): `Optional`, `Computed`, `string`, plan-modifier `stringplanmodifier.UseStateForUnknown()`. Matomo's own default is `""` - always send a value (never omit the key), defaulting to `""` when config is null.
- `priority` (tags only): `Optional`, `Computed`, `int64`, plan-modifier `int64planmodifier.UseStateForUnknown()`. Matomo's own default is `999` - always send a value, defaulting to `999` when config is null.
- Container flags (`ignore_gtm_data_layer`, `actively_sync_gtm_data_layer`, `is_tag_fire_limit_allowed_in_preview_mode`): `Optional`, `Computed`, `bool`, plan-modifier `boolplanmodifier.UseStateForUnknown()`. Matomo's own API-layer default is `false` for all three - always send a value, defaulting to `false` when config is null.
- Boolean wire encoding for the container flags reuses the existing `boolToIntString` helper (`internal/matomo/sites.go:100-105`) - do not write a new one.
- Every generated-file and docs-file change must be verified with `gofmt -l` (schema files) and the project's doc-generation drift check in mind (CI runs `make docs`; this sandbox cannot, so hand-reconstruction must match tfplugindocs' exact alphabetical-within-section, `(String)`/`(Number)` conventions already used throughout `docs/resources/*.md`).
- Full spec: `docs/superpowers/specs/2026-07-05-tagmanager-schema-completeness-design.md`.

---

## Task 1: Client layer - `description` (tag/trigger/variable) and `priority` (tag)

**Files:**
- Modify: `internal/matomo/tagmanager_tags.go`
- Modify: `internal/matomo/tagmanager_triggers.go`
- Modify: `internal/matomo/tagmanager_variables.go`
- Test: `internal/matomo/tagmanager_tags_test.go` (create)
- Test: `internal/matomo/tagmanager_triggers_test.go` (create)
- Test: `internal/matomo/tagmanager_variables_test.go` (create)

**Interfaces:**
- Produces: `Tag.Description string`, `Tag.Priority int`, `TagParams.Description string`, `TagParams.Priority int`; `Trigger.Description string`, `TriggerParams.Description string`; `Variable.Description string`, `VariableParams.Description string`. `AddContainerTag`/`UpdateContainerTag`/`AddContainerTrigger`/`UpdateContainerTrigger`/`AddContainerVariable`/`UpdateContainerVariable` all keep their existing signatures (they take a `*Params` struct, so no signature change) but now also submit `description`/`priority` form values.

- [ ] **Step 1: Write the failing test for tag description/priority encoding**

Create `internal/matomo/tagmanager_tags_test.go`:

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

func TestAddContainerTag_sendsDescriptionAndPriority(t *testing.T) {
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		gotForm = r.Form
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int{"value": 7})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", srv.Client())
	_, err := client.AddContainerTag(context.Background(), 1, "abc123", "1", TagParams{
		Type:        "CustomHtml",
		Name:        "My Tag",
		Description: "a tag description",
		Priority:    42,
		Parameters:  ParamsMap{},
	})
	if err != nil {
		t.Fatalf("AddContainerTag() error = %v", err)
	}
	if got := gotForm.Get("description"); got != "a tag description" {
		t.Errorf("description = %q, want %q", got, "a tag description")
	}
	if got := gotForm.Get("priority"); got != "42" {
		t.Errorf("priority = %q, want %q", got, "42")
	}
}

func TestGetContainerTag_decodesDescriptionAndPriority(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"idtag":             7,
			"name":              "My Tag",
			"type":              "CustomHtml",
			"status":            "active",
			"description":       "a tag description",
			"priority":          42,
			"parameters":        map[string]any{},
			"fire_trigger_ids":  []int{},
			"block_trigger_ids": []int{},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", srv.Client())
	tag, err := client.GetContainerTag(context.Background(), 1, "abc123", "1", "7")
	if err != nil {
		t.Fatalf("GetContainerTag() error = %v", err)
	}
	if tag.Description != "a tag description" {
		t.Errorf("tag.Description = %q, want %q", tag.Description, "a tag description")
	}
	if tag.Priority != 42 {
		t.Errorf("tag.Priority = %d, want 42", tag.Priority)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/matomo/ -run TestAddContainerTag_sendsDescriptionAndPriority -v`
Expected: FAIL - `gotForm.Get("description")` and `gotForm.Get("priority")` are both `""`/`"0"` because `TagParams` has no such fields yet (compile error: `unknown field Description in struct literal of type TagParams`).

- [ ] **Step 3: Add Description/Priority to Tag/TagParams and wire them through**

In `internal/matomo/tagmanager_tags.go`, modify the `Tag` and `TagParams` structs and `tagParamsToValues`:

```go
// Tag is a Matomo Tag Manager tag within a container version.
type Tag struct {
	IDTag       int       `json:"idtag"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Status      string    `json:"status"`
	Description string    `json:"description"`
	Priority    int       `json:"priority"`
	Parameters  ParamsMap `json:"parameters"`
	// Confirmed against Matomo's own TagTest.php fixture: the response keys
	// are fire_trigger_ids/block_trigger_ids (snake_case), unlike the
	// fireTriggerIds/blockTriggerIds (camelCase) request parameters used to
	// set them. Confirmed against a live instance: unlike most other
	// Matomo ids, these array elements are unquoted JSON numbers, not
	// strings.
	FireTriggerIDs  []int `json:"fire_trigger_ids"`
	BlockTriggerIDs []int `json:"block_trigger_ids"`
}

// TagParams holds the fields accepted by addContainerTag/updateContainerTag.
type TagParams struct {
	Type            string
	Name            string
	Description     string
	Priority        int
	Parameters      ParamsMap
	FireTriggerIDs  []string
	BlockTriggerIDs []string
}

func tagParamsToValues(idSite int, idContainer, idContainerVersion string, p TagParams) url.Values {
	v := url.Values{
		"idSite":             {strconv.Itoa(idSite)},
		"idContainer":        {idContainer},
		"idContainerVersion": {idContainerVersion},
		"type":               {p.Type},
		"name":               {p.Name},
		"description":        {p.Description},
		"priority":           {strconv.Itoa(p.Priority)},
	}
	addParamsMap(v, "parameters", p.Parameters)
	addArrayParam(v, "fireTriggerIds", p.FireTriggerIDs)
	addArrayParam(v, "blockTriggerIds", p.BlockTriggerIDs)

	return v
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/matomo/ -run 'TestAddContainerTag_sendsDescriptionAndPriority|TestGetContainerTag_decodesDescriptionAndPriority' -v`
Expected: both PASS

- [ ] **Step 5: Write the failing test for trigger description encoding**

Create `internal/matomo/tagmanager_triggers_test.go`:

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

func TestAddContainerTrigger_sendsDescription(t *testing.T) {
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		gotForm = r.Form
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int{"value": 9})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", srv.Client())
	_, err := client.AddContainerTrigger(context.Background(), 1, "abc123", "1", TriggerParams{
		Type:        "PageView",
		Name:        "My Trigger",
		Description: "a trigger description",
		Parameters:  ParamsMap{},
	})
	if err != nil {
		t.Fatalf("AddContainerTrigger() error = %v", err)
	}
	if got := gotForm.Get("description"); got != "a trigger description" {
		t.Errorf("description = %q, want %q", got, "a trigger description")
	}
}

func TestGetContainerTrigger_decodesDescription(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"idtrigger":   9,
			"name":        "My Trigger",
			"type":        "PageView",
			"description": "a trigger description",
			"parameters":  map[string]any{},
			"conditions":  []any{},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", srv.Client())
	trig, err := client.GetContainerTrigger(context.Background(), 1, "abc123", "1", "9")
	if err != nil {
		t.Fatalf("GetContainerTrigger() error = %v", err)
	}
	if trig.Description != "a trigger description" {
		t.Errorf("trig.Description = %q, want %q", trig.Description, "a trigger description")
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `go test ./internal/matomo/ -run TestAddContainerTrigger_sendsDescription -v`
Expected: FAIL (compile error: `unknown field Description in struct literal of type TriggerParams`)

- [ ] **Step 7: Add Description to Trigger/TriggerParams and wire it through**

In `internal/matomo/tagmanager_triggers.go`, modify `Trigger`, `TriggerParams`, and `triggerParamsToValues`:

```go
// Trigger is a Matomo Tag Manager trigger within a container version.
type Trigger struct {
	IDTrigger   int         `json:"idtrigger"`
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Parameters  ParamsMap   `json:"parameters"`
	Conditions  []Condition `json:"conditions"`
}

// TriggerParams holds the fields accepted by
// addContainerTrigger/updateContainerTrigger.
type TriggerParams struct {
	Type        string
	Name        string
	Description string
	Parameters  ParamsMap
	Conditions  []Condition
}

func triggerParamsToValues(idSite int, idContainer, idContainerVersion string, p TriggerParams) url.Values {
	v := url.Values{
		"idSite":             {strconv.Itoa(idSite)},
		"idContainer":        {idContainer},
		"idContainerVersion": {idContainerVersion},
		"type":               {p.Type},
		"name":               {p.Name},
		"description":        {p.Description},
	}
	addParamsMap(v, "parameters", p.Parameters)
	addConditionsParam(v, "conditions", p.Conditions)

	return v
}
```

- [ ] **Step 8: Run tests to verify they pass**

Run: `go test ./internal/matomo/ -run 'TestAddContainerTrigger_sendsDescription|TestGetContainerTrigger_decodesDescription' -v`
Expected: both PASS

- [ ] **Step 9: Write the failing test for variable description encoding**

Create `internal/matomo/tagmanager_variables_test.go`:

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

func TestAddContainerVariable_sendsDescription(t *testing.T) {
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		gotForm = r.Form
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int{"value": 3})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", srv.Client())
	_, err := client.AddContainerVariable(context.Background(), 1, "abc123", "1", VariableParams{
		Type:        "Constant",
		Name:        "My Variable",
		Description: "a variable description",
		Parameters:  ParamsMap{},
	})
	if err != nil {
		t.Fatalf("AddContainerVariable() error = %v", err)
	}
	if got := gotForm.Get("description"); got != "a variable description" {
		t.Errorf("description = %q, want %q", got, "a variable description")
	}
}

func TestGetContainerVariable_decodesDescription(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"idvariable":    3,
			"name":          "My Variable",
			"type":          "Constant",
			"description":   "a variable description",
			"default_value": "",
			"parameters":    map[string]any{},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", srv.Client())
	v, err := client.GetContainerVariable(context.Background(), 1, "abc123", "1", "3")
	if err != nil {
		t.Fatalf("GetContainerVariable() error = %v", err)
	}
	if v.Description != "a variable description" {
		t.Errorf("v.Description = %q, want %q", v.Description, "a variable description")
	}
}
```

- [ ] **Step 10: Run test to verify it fails**

Run: `go test ./internal/matomo/ -run TestAddContainerVariable_sendsDescription -v`
Expected: FAIL (compile error: `unknown field Description in struct literal of type VariableParams`)

- [ ] **Step 11: Add Description to Variable/VariableParams and wire it through**

In `internal/matomo/tagmanager_variables.go`, modify `Variable`, `VariableParams`, and `variableParamsToValues`:

```go
// Variable is a Matomo Tag Manager variable within a container version.
type Variable struct {
	IDVariable  int       `json:"idvariable"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Parameters  ParamsMap `json:"parameters"`
	// Confirmed against Matomo's own VariableTest.php fixture: the response
	// key is default_value (snake_case), unlike the defaultValue (camelCase)
	// request parameter used to set it.
	DefaultValue string `json:"default_value"`
}

// VariableParams holds the fields accepted by
// addContainerVariable/updateContainerVariable.
type VariableParams struct {
	Type         string
	Name         string
	Description  string
	Parameters   ParamsMap
	DefaultValue *string
}

func variableParamsToValues(idSite int, idContainer, idContainerVersion string, p VariableParams) url.Values {
	v := url.Values{
		"idSite":             {strconv.Itoa(idSite)},
		"idContainer":        {idContainer},
		"idContainerVersion": {idContainerVersion},
		"type":               {p.Type},
		"name":               {p.Name},
		"description":        {p.Description},
	}
	addParamsMap(v, "parameters", p.Parameters)

	if p.DefaultValue != nil {
		v.Set("defaultValue", *p.DefaultValue)
	}

	return v
}
```

- [ ] **Step 12: Run tests to verify they pass**

Run: `go test ./internal/matomo/ -run 'TestAddContainerVariable_sendsDescription|TestGetContainerVariable_decodesDescription' -v`
Expected: both PASS

- [ ] **Step 13: Run the full matomo package test suite**

Run: `go test ./internal/matomo/... -v -count=1`
Expected: PASS (existing tests must still pass - `TagParams{}`/`TriggerParams{}`/`VariableParams{}` zero values still work since the new fields default to `""`/`0`)

- [ ] **Step 14: Commit**

```bash
git add internal/matomo/tagmanager_tags.go internal/matomo/tagmanager_triggers.go internal/matomo/tagmanager_variables.go internal/matomo/tagmanager_tags_test.go internal/matomo/tagmanager_triggers_test.go internal/matomo/tagmanager_variables_test.go
git commit -m "feat: add description (tag/trigger/variable) and priority (tag) to matomo client"
```

---

## Task 2: Client layer - container flags

**Files:**
- Modify: `internal/matomo/tagmanager_containers.go`
- Test: `internal/matomo/tagmanager_containers_test.go` (create)

**Interfaces:**
- Consumes: `boolToIntString(b bool) string` (`internal/matomo/sites.go:100-105`, existing, unchanged).
- Produces: `Container.IgnoreGtmDataLayer`, `Container.ActivelySyncGtmDataLayer`, `Container.IsTagFireLimitAllowedInPreviewMode` (all `bool`). `AddContainer`/`UpdateContainer` gain three new trailing `bool` parameters: `ignoreGtmDataLayer, isTagFireLimitAllowedInPreviewMode, activelySyncGtmDataLayer bool` (this exact order and these exact names match Matomo's own `API.php` signature). **This is a breaking signature change** to both functions - Task 9 updates their only two call sites.

- [ ] **Step 1: Write the failing test**

Create `internal/matomo/tagmanager_containers_test.go`:

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

func TestAddContainer_sendsFlags(t *testing.T) {
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		gotForm = r.Form
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"value": "abc123"})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", srv.Client())
	_, err := client.AddContainer(context.Background(), 1, "web", "My Container", "a description", true, false, true)
	if err != nil {
		t.Fatalf("AddContainer() error = %v", err)
	}
	if got := gotForm.Get("ignoreGtmDataLayer"); got != "1" {
		t.Errorf("ignoreGtmDataLayer = %q, want 1", got)
	}
	if got := gotForm.Get("isTagFireLimitAllowedInPreviewMode"); got != "0" {
		t.Errorf("isTagFireLimitAllowedInPreviewMode = %q, want 0", got)
	}
	if got := gotForm.Get("activelySyncGtmDataLayer"); got != "1" {
		t.Errorf("activelySyncGtmDataLayer = %q, want 1", got)
	}
}

func TestGetContainer_decodesFlags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"idcontainer":                        "abc123",
			"idsite":                             1,
			"context":                            "web",
			"name":                               "My Container",
			"description":                        "a description",
			"ignoreGtmDataLayer":                 true,
			"isTagFireLimitAllowedInPreviewMode": false,
			"activelySyncGtmDataLayer":           true,
			"draft":                              map[string]any{"idcontainerversion": 1},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", srv.Client())
	ct, err := client.GetContainer(context.Background(), 1, "abc123")
	if err != nil {
		t.Fatalf("GetContainer() error = %v", err)
	}
	if !ct.IgnoreGtmDataLayer {
		t.Error("IgnoreGtmDataLayer = false, want true")
	}
	if ct.IsTagFireLimitAllowedInPreviewMode {
		t.Error("IsTagFireLimitAllowedInPreviewMode = true, want false")
	}
	if !ct.ActivelySyncGtmDataLayer {
		t.Error("ActivelySyncGtmDataLayer = false, want true")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/matomo/ -run 'TestAddContainer_sendsFlags|TestGetContainer_decodesFlags' -v`
Expected: FAIL (compile error: `too many arguments in call to client.AddContainer` and `ct.IgnoreGtmDataLayer undefined`)

- [ ] **Step 3: Add the three fields and parameters**

In `internal/matomo/tagmanager_containers.go`, modify `Container`, `AddContainer`, and `UpdateContainer`:

```go
// Container is a Matomo Tag Manager container.
type Container struct {
	IDContainer string `json:"idcontainer"`
	IDSite      int    `json:"idsite"`
	Context     string `json:"context"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// All three confirmed against Matomo's own Dao/ContainersDao.php and
	// API.php: TINYINT(1) columns, addContainer/updateContainer parameters
	// defaulting to false at the API layer.
	IgnoreGtmDataLayer                 bool `json:"ignoreGtmDataLayer"`
	IsTagFireLimitAllowedInPreviewMode bool `json:"isTagFireLimitAllowedInPreviewMode"`
	ActivelySyncGtmDataLayer           bool `json:"activelySyncGtmDataLayer"`
	// Draft is the container's mutable draft version, always present on a
	// real container (confirmed against Matomo's own TagManager source:
	// TagManager.getContainer's response nests it as draft.idcontainerversion
	// - there is no dedicated API method to fetch just the draft, and
	// getContainerVersions' entries have no boolean "isDraft" field to pick
	// it out from the version list). Unlike most other Matomo ids, this one
	// comes back as an unquoted JSON number, confirmed against a live
	// instance.
	Draft *struct {
		IDContainerVersion int `json:"idcontainerversion"`
	} `json:"draft"`
}

// AddContainer creates a new Tag Manager container and returns its ID.
func (c *Client) AddContainer(ctx context.Context, idSite int, tmContext, name, description string, ignoreGtmDataLayer, isTagFireLimitAllowedInPreviewMode, activelySyncGtmDataLayer bool) (string, error) {
	v := url.Values{
		"idSite":                             {strconv.Itoa(idSite)},
		"context":                            {tmContext},
		"name":                               {name},
		"description":                        {description},
		"ignoreGtmDataLayer":                 {boolToIntString(ignoreGtmDataLayer)},
		"isTagFireLimitAllowedInPreviewMode": {boolToIntString(isTagFireLimitAllowedInPreviewMode)},
		"activelySyncGtmDataLayer":           {boolToIntString(activelySyncGtmDataLayer)},
	}
	var out struct {
		Value string `json:"value"`
	}
	if err := c.call(ctx, "TagManager.addContainer", v, &out); err != nil {
		return "", err
	}
	return out.Value, nil
}

// UpdateContainer updates a container's name, description, and flags.
func (c *Client) UpdateContainer(ctx context.Context, idSite int, idContainer, name, description string, ignoreGtmDataLayer, isTagFireLimitAllowedInPreviewMode, activelySyncGtmDataLayer bool) error {
	v := url.Values{
		"idSite":                             {strconv.Itoa(idSite)},
		"idContainer":                        {idContainer},
		"name":                               {name},
		"description":                        {description},
		"ignoreGtmDataLayer":                 {boolToIntString(ignoreGtmDataLayer)},
		"isTagFireLimitAllowedInPreviewMode": {boolToIntString(isTagFireLimitAllowedInPreviewMode)},
		"activelySyncGtmDataLayer":           {boolToIntString(activelySyncGtmDataLayer)},
	}
	return c.call(ctx, "TagManager.updateContainer", v, nil)
}
```

- [ ] **Step 4: This breaks `resource_tagmanager_container.go` - fix its two call sites minimally so the package compiles**

In `internal/provider/resource_tagmanager_container.go`, temporarily pass `false, false, false` for the three new parameters (Task 9 replaces these with real config-driven values):

In `Create` (around line 115):
```go
	idContainer, err := r.client.AddContainer(ctx, siteID, plan.Context.ValueString(), plan.Name.ValueString(), description, false, false, false)
```

In `Update` (around line 174):
```go
	if err := r.client.UpdateContainer(ctx, siteID, idContainer, plan.Name.ValueString(), description, false, false, false); err != nil {
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/matomo/... ./internal/provider/... -run 'TestAddContainer_sendsFlags|TestGetContainer_decodesFlags' -v -count=1`
Expected: both PASS, and the whole `internal/provider` package still compiles

- [ ] **Step 6: Run the full matomo package test suite**

Run: `go test ./internal/matomo/... -v -count=1`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/matomo/tagmanager_containers.go internal/matomo/tagmanager_containers_test.go internal/provider/resource_tagmanager_container.go
git commit -m "feat: add ignore_gtm_data_layer/actively_sync_gtm_data_layer/is_tag_fire_limit_allowed_in_preview_mode to matomo client"
```

---

## Task 3: Shared typed-resource runtime - common struct fields and CRUD wiring

**Files:**
- Modify: `internal/provider/typed_tag_resource.go`
- Modify: `internal/provider/typed_trigger_resource.go`
- Modify: `internal/provider/typed_variable_resource.go`
- Test: `internal/provider/typed_tag_resource_test.go` (existing file - extend)

**Interfaces:**
- Consumes: `matomo.TagParams.Description/Priority`, `matomo.TriggerParams.Description`, `matomo.VariableParams.Description` (Task 1).
- Produces: `typedTagCommon.Description types.String`, `typedTagCommon.Priority types.Int64`; `typedTriggerCommon.Description types.String`; `typedVariableCommon.Description types.String`. These are consumed by Task 4's `tools/gen` template change (the generated models embed these structs, so no generated-file change is needed for the Go struct side - only the schema Attributes map needs the new entries, which Task 4/5 handle).

- [ ] **Step 1: Add Description/Priority to typedTagCommon and wire Create/Read/Update**

In `internal/provider/typed_tag_resource.go`, modify the struct:

```go
type typedTagCommon struct {
	ID              types.String   `tfsdk:"id"`
	ContainerID     types.String   `tfsdk:"container_id"`
	Name            types.String   `tfsdk:"name"`
	Status          types.String   `tfsdk:"status"`
	Description     types.String   `tfsdk:"description"`
	Priority        types.Int64    `tfsdk:"priority"`
	FireTriggerIDs  []types.String `tfsdk:"fire_trigger_ids"`
	BlockTriggerIDs []types.String `tfsdk:"block_trigger_ids"`
}
```

In `Create`, right before the `r.client.AddContainerTag` call, resolve defaults and pass them through:

```go
	description := ""
	if !common.Description.IsUnknown() && !common.Description.IsNull() {
		description = common.Description.ValueString()
	}
	priority := int64(999)
	if !common.Priority.IsUnknown() && !common.Priority.IsNull() {
		priority = common.Priority.ValueInt64()
	}

	idTag, err := r.client.AddContainerTag(ctx, siteID, idContainer, versionID, matomo.TagParams{
		Type:            model.Meta().TypeID,
		Name:            common.Name.ValueString(),
		Description:     description,
		Priority:        int(priority),
		Parameters:      model.ToParams(),
		FireTriggerIDs:  fireIDs,
		BlockTriggerIDs: blockIDs,
	})
```

Right after `common.Status = types.StringValue(tag.Status)` in `Create` (the block that reads the tag back), add:

```go
	common.Description = types.StringValue(tag.Description)
	common.Priority = types.Int64Value(int64(tag.Priority))
```

In `Read`, right after the identical `common.Status = types.StringValue(tag.Status)` line, add the same two lines:

```go
	common.Description = types.StringValue(tag.Description)
	common.Priority = types.Int64Value(int64(tag.Priority))
```

In `Update`, add `Description`/`Priority` to the `matomo.TagParams` literal passed to `UpdateContainerTag` (values are already known at this point thanks to `UseStateForUnknown`, so no default-resolution needed here - use them directly):

```go
	if err := r.client.UpdateContainerTag(ctx, siteID, idContainer, versionID, idTag, matomo.TagParams{
		Type:            model.Meta().TypeID,
		Name:            common.Name.ValueString(),
		Description:     common.Description.ValueString(),
		Priority:        int(common.Priority.ValueInt64()),
		Parameters:      model.ToParams(),
		FireTriggerIDs:  fireIDs,
		BlockTriggerIDs: blockIDs,
	}); err != nil {
```

- [ ] **Step 2: Add Description to typedTriggerCommon and wire Create/Read/Update**

In `internal/provider/typed_trigger_resource.go`, modify the struct:

```go
type typedTriggerCommon struct {
	ID          types.String            `tfsdk:"id"`
	ContainerID types.String            `tfsdk:"container_id"`
	Name        types.String            `tfsdk:"name"`
	Description types.String            `tfsdk:"description"`
	Condition   []triggerConditionModel `tfsdk:"condition"`
}
```

In `Create`, right before the `r.client.AddContainerTrigger` call:

```go
	description := ""
	if !common.Description.IsUnknown() && !common.Description.IsNull() {
		description = common.Description.ValueString()
	}

	idTrigger, err := r.client.AddContainerTrigger(ctx, siteID, idContainer, versionID, matomo.TriggerParams{
		Type:        model.Meta().TypeID,
		Name:        common.Name.ValueString(),
		Description: description,
		Parameters:  model.ToParams(),
		Conditions:  conditionsToParams(common.Condition),
	})
```

Right after `common.Name = types.StringValue(trig.Name)` in `Create`'s read-back block, add:

```go
	common.Description = types.StringValue(trig.Description)
```

In `Read`, right after the identical `common.Name = types.StringValue(trig.Name)` line, add the same line:

```go
	common.Description = types.StringValue(trig.Description)
```

In `Update`, add `Description` to the `matomo.TriggerParams` literal:

```go
	if err := r.client.UpdateContainerTrigger(ctx, siteID, idContainer, versionID, idTrigger, matomo.TriggerParams{
		Type:        model.Meta().TypeID,
		Name:        common.Name.ValueString(),
		Description: common.Description.ValueString(),
		Parameters:  model.ToParams(),
		Conditions:  conditionsToParams(common.Condition),
	}); err != nil {
```

- [ ] **Step 3: Add Description to typedVariableCommon and wire Create/Read/Update**

In `internal/provider/typed_variable_resource.go`, modify the struct:

```go
type typedVariableCommon struct {
	ID           types.String `tfsdk:"id"`
	ContainerID  types.String `tfsdk:"container_id"`
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	DefaultValue types.String `tfsdk:"default_value"`
}
```

In `Create`, right before the `r.client.AddContainerVariable` call:

```go
	description := ""
	if !common.Description.IsUnknown() && !common.Description.IsNull() {
		description = common.Description.ValueString()
	}

	idVariable, err := r.client.AddContainerVariable(ctx, siteID, idContainer, versionID, matomo.VariableParams{
		Type:         model.Meta().TypeID,
		Name:         common.Name.ValueString(),
		Description:  description,
		Parameters:   model.ToParams(),
		DefaultValue: defaultValue,
	})
```

Right after `common.Name = types.StringValue(v.Name)` in `Create`'s read-back block, add:

```go
	common.Description = types.StringValue(v.Description)
```

In `Read`, right after the identical `common.Name = types.StringValue(v.Name)` line, add the same line:

```go
	common.Description = types.StringValue(v.Description)
```

In `Update`, add `Description` to the `matomo.VariableParams` literal, and (since `Update` already does a `GetContainerVariable` read-back for `DefaultValue`) also set `common.Description` from that same read-back response, right next to the existing `common.DefaultValue = ...`-equivalent line (`plan.DefaultValue`/`common.DefaultValue` depending on file - this file uses `common.DefaultValue`):

```go
	if err := r.client.UpdateContainerVariable(ctx, siteID, idContainer, versionID, idVariable, matomo.VariableParams{
		Type:         model.Meta().TypeID,
		Name:         common.Name.ValueString(),
		Description:  common.Description.ValueString(),
		Parameters:   model.ToParams(),
		DefaultValue: defaultValue,
	}); err != nil {
		resp.Diagnostics.AddError("Error updating Matomo Tag Manager variable", err.Error())
		return
	}

	// See Create's identical read-back for why this is required whenever
	// default_value is left unconfigured. Reused here to also pick up the
	// server's real description in the same round trip.
	v, err := r.client.GetContainerVariable(ctx, siteID, idContainer, versionID, idVariable)
	if err != nil {
		resp.Diagnostics.AddError("Error reading back updated Matomo Tag Manager variable", err.Error())
		return
	}
	common.DefaultValue = variableDefaultValueFromAPI(v.DefaultValue)
	common.Description = types.StringValue(v.Description)
```

- [ ] **Step 4: Run the existing typed-resource test file to check for compile errors**

Run: `go build ./... 2>&1 | head -50`
Expected: build fails only in `tools/gen`-generated files (Task 5 has not run yet) with errors like `unknown field Description in struct literal` for every `generated_tag_*.go`/`generated_trigger_*.go`/`generated_variable_*.go` file that constructs `matomo.TagParams{}` etc. This is expected at this point in the plan - Task 4 and 5 fix it. Confirm the *hand-written* files in this task compile cleanly by building just them:

Run: `go vet ./internal/provider/ 2>&1 | grep -v "generated_"`
Expected: no output referencing `typed_tag_resource.go`, `typed_trigger_resource.go`, or `typed_variable_resource.go`

- [ ] **Step 5: Commit**

```bash
git add internal/provider/typed_tag_resource.go internal/provider/typed_trigger_resource.go internal/provider/typed_variable_resource.go
git commit -m "feat: wire description/priority through the shared typed-resource runtime"
```

---

## Task 4: `tools/gen` template - schema.go.tmpl and unit tests

**Files:**
- Modify: `tools/gen/templates/schema.go.tmpl`
- Modify: `tools/gen/emit_test.go`

**Interfaces:**
- Consumes: `.Kind` (already available in template context, values `"tag"`/`"trigger"`/`"variable"`).
- Produces: every future `go run ./tools/gen` regeneration emits `"description"` (all kinds) and `"priority"` (tag kind only) in the common Attributes block. Task 5's bulk script inserts byte-identical text into the 64 already-committed generated files, so this template becomes the source of truth for any future real regeneration.

- [ ] **Step 1: Write the failing unit tests**

In `tools/gen/emit_test.go`, add (after the existing `TestRenderSchema_conditionallyRequired` test, same file):

```go
func TestRenderSchema_commonDescriptionAndPriority(t *testing.T) {
	tagSpec := TypeSpec{
		Kind:         "tag",
		TypeID:       "CustomHtml",
		Slug:         "customhtml",
		ResourceName: "matomo_tagmanager_tag_customhtml",
		Description:  "Inject custom HTML",
		Params: []ParamSpec{
			{MatomoName: "customHtml", TFName: "custom_html", GoFieldName: "CustomHtml", GoType: "String", Required: true},
		},
	}
	src, err := RenderSchema(tagSpec)
	if err != nil {
		t.Fatalf("RenderSchema() error = %v", err)
	}
	got := string(src)
	for _, want := range []string{
		`"description": schema.StringAttribute{`,
		`"priority": schema.Int64Attribute{`,
		`int64planmodifier.UseStateForUnknown()`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("tag schema missing %q; full source:\n%s", want, got)
		}
	}

	triggerSpec := TypeSpec{
		Kind:         "trigger",
		TypeID:       "PageView",
		Slug:         "pageview",
		ResourceName: "matomo_tagmanager_trigger_pageview",
		Description:  "Triggered on page view",
	}
	src, err = RenderSchema(triggerSpec)
	if err != nil {
		t.Fatalf("RenderSchema() error = %v", err)
	}
	got = string(src)
	if !strings.Contains(got, `"description": schema.StringAttribute{`) {
		t.Errorf("trigger schema missing description attribute; full source:\n%s", got)
	}
	if strings.Contains(got, `"priority"`) {
		t.Errorf("trigger schema must not have a priority attribute; full source:\n%s", got)
	}

	variableSpec := TypeSpec{
		Kind:         "variable",
		TypeID:       "Constant",
		Slug:         "constant",
		ResourceName: "matomo_tagmanager_variable_constant",
		Description:  "A constant value",
	}
	src, err = RenderSchema(variableSpec)
	if err != nil {
		t.Fatalf("RenderSchema() error = %v", err)
	}
	got = string(src)
	if !strings.Contains(got, `"description": schema.StringAttribute{`) {
		t.Errorf("variable schema missing description attribute; full source:\n%s", got)
	}
	if strings.Contains(got, `"priority"`) {
		t.Errorf("variable schema must not have a priority attribute; full source:\n%s", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tools/gen/ -run TestRenderSchema_commonDescriptionAndPriority -v`
Expected: FAIL - none of the three `strings.Contains` checks for `"description"`/`"priority"` find a match yet.

- [ ] **Step 3: Modify schema.go.tmpl**

In `tools/gen/templates/schema.go.tmpl`, replace the block from `"name": schema.StringAttribute{` through the end of the `{{- if eq .Kind "variable"}}...{{- end}}` block (originally lines 75-116) with:

```
			"name": schema.StringAttribute{
				Required: true,
			},
			"description": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Description: "Optional free-text description, shown in Matomo's Tag Manager UI.",
			},
{{- if eq .Kind "tag"}}
			"status": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Validators: []validator.String{stringvalidator.OneOf("active", "paused")},
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"fire_trigger_ids": schema.ListAttribute{
				Required:    true,
				ElementType: types.StringType,
			},
			"block_trigger_ids": schema.ListAttribute{
				// Not Computed, unlike the other common/generated Optional
				// attributes: the generated Go field type for a List
				// attribute is a bare []types.String (see the Params
				// range below), which - unlike types.List - has no way to
				// represent "the whole list is unknown," so marking it
				// Computed makes terraform-plugin-framework fail outright
				// trying to decode plan's Unknown value into it
				// (confirmed against a real acceptance-test run: "Value
				// Conversion Error ... Received unknown value, however
				// the target type cannot handle unknown values"). A
				// List-typed field can still show a spurious
				// "refresh plan not empty" diff if Matomo defaults it to
				// a non-empty value server-side - fixing that for real
				// would mean switching the generated field type to
				// types.List, which is a larger change than this pass
				// covers.
				Optional:    true,
				ElementType: types.StringType,
			},
			"priority": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
				Description: "Execution priority - lower values fire earlier when multiple tags fire on the same trigger. Matomo defaults to 999 when unset.",
			},
{{- end}}
{{- if eq .Kind "variable"}}
			"default_value": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
{{- end}}
```

This introduces `int64planmodifier` as an unconditional import need for the `"tag"` kind. In the same file's import block (near the top), the import is already gated on `{{- if .NeedsInt64PlanModifierImport}}` - this flag is computed per-spec in `tools/gen/emit.go`'s `newTemplateData`, currently only from *type-specific* Int64 params. Since `priority` is now a *common* tag attribute (not a param), `newTemplateData` must also set it for tag kind.

- [ ] **Step 4: Update tools/gen/emit.go so tag kind always needs the int64planmodifier import**

In `tools/gen/emit.go`, in `newTemplateData` (around line 116), change:

```go
	var needsBoolPM, needsInt64PM, needsFloat64PM bool
	var hasListOfObjectsBlocks, needsListPM, needsAttrImport bool
	for _, p := range spec.Params {
```

to:

```go
	needsInt64PM := spec.Kind == "tag" // priority is always an Int64 common attribute for tags
	var needsBoolPM, needsFloat64PM bool
	var hasListOfObjectsBlocks, needsListPM, needsAttrImport bool
	for _, p := range spec.Params {
```

(The existing loop body below is unchanged - it can still additionally set `needsInt64PM = true` for any kind with an Int64-typed param; the new initializer just means tag kind starts `true` instead of `false`.)

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./tools/gen/ -run TestRenderSchema_commonDescriptionAndPriority -v`
Expected: PASS

- [ ] **Step 6: Run the full tools/gen test suite**

Run: `go test ./tools/gen/... -v -count=1`
Expected: PASS (existing `TestRenderSchema_parsesAsValidGo` etc. must still pass - they don't assert an *absence* of `description`/`priority`, only presence of their own fields, so they're unaffected)

- [ ] **Step 7: Commit**

```bash
git add tools/gen/templates/schema.go.tmpl tools/gen/emit.go tools/gen/emit_test.go
git commit -m "feat: emit description/priority common attributes from tools/gen"
```

---

## Task 5: Bulk-regenerate the 64 committed generated schema files

**Files:**
- Create (temporary, deleted at end of task): `tools/gen/tmp_bulk_insert.go` (a standalone `go run`-able script, built with a `//go:build ignore` tag so it's never part of the normal build)
- Modify: all 31 `internal/provider/generated_tag_*.go` files (excluding `_test.go`)
- Modify: all 17 `internal/provider/generated_trigger_*.go` files (excluding `_test.go`)
- Modify: all 16 `internal/provider/generated_variable_*.go` files (excluding `_test.go`)

**Interfaces:**
- Consumes: the exact literal text Task 4's template now emits for the common `description`/`priority` attributes (this task's script must produce byte-identical output, verified by `gofmt`).
- Produces: 64 modified generated files whose schema functions now declare `description` (all) and `priority` (tag only), matching what `go run ./tools/gen` would emit if run against a live Matomo instance (not available in this sandbox - this script is the documented workaround, confirmed viable in a prior session for the same reason).

This task cannot use TDD in the usual sense (there's no new behavior to unit-test beyond what Task 4 already covers) - its correctness is verified by the full build/vet/test pass at the end plus visual diff review.

- [ ] **Step 1: Confirm the anchors are unique before writing the script**

Run this one-off check (no file changes) to reconfirm the exact anchor text appears exactly once in every target file - if this ever prints anything, stop and investigate before proceeding, since the plan's insertion logic assumes exactly one match per file:

```bash
python3 - <<'EOF'
import glob
name_anchor = '\t\t\t"name": schema.StringAttribute{\n\t\t\t\tRequired: true,\n\t\t\t},\n'
bad = []
files = (glob.glob("internal/provider/generated_tag_*.go") +
         glob.glob("internal/provider/generated_trigger_*.go") +
         glob.glob("internal/provider/generated_variable_*.go"))
files = [f for f in files if not f.endswith("_test.go")]
for f in files:
    n = open(f).read().count(name_anchor)
    if n != 1:
        bad.append((f, n))
print("files with count != 1:", bad)
print("total files:", len(files))
EOF
```

Expected: `files with count != 1: []` and `total files: 64`

- [ ] **Step 2: Write the bulk-insertion script**

Create `tools/gen/tmp_bulk_insert.go`:

```go
//go:build ignore

// tmp_bulk_insert.go is a one-off script that mechanically inserts the
// "description" (all kinds) and "priority" (tag kind) common attributes
// into every already-committed generated_{tag,trigger,variable}_*.go
// schema file, producing byte-identical output to what tools/gen's
// updated template (schema.go.tmpl) would emit if run against a live
// Matomo instance - not available in this sandbox. Run once via
// `go run tools/gen/tmp_bulk_insert.go` from the repo root, then delete
// this file; it is not part of the normal build (see the ignore tag
// above) and is not meant to be run again.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const nameAnchor = "\t\t\t\"name\": schema.StringAttribute{\n\t\t\t\tRequired: true,\n\t\t\t},\n"

const descriptionInsert = "\t\t\t\"description\": schema.StringAttribute{\n" +
	"\t\t\t\tOptional: true,\n" +
	"\t\t\t\tComputed: true,\n" +
	"\t\t\t\tPlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},\n" +
	"\t\t\t\tDescription: \"Optional free-text description, shown in Matomo's Tag Manager UI.\",\n" +
	"\t\t\t},\n"

const blockTriggerIDsAnchor = "\t\t\t\"block_trigger_ids\": schema.ListAttribute{\n" +
	"\t\t\t\t// Not Computed, unlike the other common/generated Optional\n" +
	"\t\t\t\t// attributes: the generated Go field type for a List\n" +
	"\t\t\t\t// attribute is a bare []types.String (see the Params\n" +
	"\t\t\t\t// range below), which - unlike types.List - has no way to\n" +
	"\t\t\t\t// represent \"the whole list is unknown,\" so marking it\n" +
	"\t\t\t\t// Computed makes terraform-plugin-framework fail outright\n" +
	"\t\t\t\t// trying to decode plan's Unknown value into it\n" +
	"\t\t\t\t// (confirmed against a real acceptance-test run: \"Value\n" +
	"\t\t\t\t// Conversion Error ... Received unknown value, however\n" +
	"\t\t\t\t// the target type cannot handle unknown values\"). A\n" +
	"\t\t\t\t// List-typed field can still show a spurious\n" +
	"\t\t\t\t// \"refresh plan not empty\" diff if Matomo defaults it to\n" +
	"\t\t\t\t// a non-empty value server-side - fixing that for real\n" +
	"\t\t\t\t// would mean switching the generated field type to\n" +
	"\t\t\t\t// types.List, which is a larger change than this pass\n" +
	"\t\t\t\t// covers.\n" +
	"\t\t\t\tOptional:    true,\n" +
	"\t\t\t\tElementType: types.StringType,\n" +
	"\t\t\t},\n"

const priorityInsert = "\t\t\t\"priority\": schema.Int64Attribute{\n" +
	"\t\t\t\tOptional: true,\n" +
	"\t\t\t\tComputed: true,\n" +
	"\t\t\t\tPlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},\n" +
	"\t\t\t\tDescription: \"Execution priority - lower values fire earlier when multiple tags fire on the same trigger. Matomo defaults to 999 when unset.\",\n" +
	"\t\t\t},\n"

const stringplanmodifierImport = "\t\"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier\"\n"
const int64planmodifierImport = "\t\"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier\"\n"

func processFile(path string, isTag bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := string(data)

	if strings.Count(content, nameAnchor) != 1 {
		return fmt.Errorf("%s: expected exactly one name anchor, found %d", path, strings.Count(content, nameAnchor))
	}
	content = strings.Replace(content, nameAnchor, nameAnchor+descriptionInsert, 1)

	if isTag {
		if strings.Count(content, blockTriggerIDsAnchor) != 1 {
			return fmt.Errorf("%s: expected exactly one block_trigger_ids anchor, found %d", path, strings.Count(content, blockTriggerIDsAnchor))
		}
		content = strings.Replace(content, blockTriggerIDsAnchor, blockTriggerIDsAnchor+priorityInsert, 1)

		if !strings.Contains(content, int64planmodifierImport) {
			if strings.Count(content, stringplanmodifierImport) != 1 {
				return fmt.Errorf("%s: expected exactly one stringplanmodifier import, found %d", path, strings.Count(content, stringplanmodifierImport))
			}
			content = strings.Replace(content, stringplanmodifierImport, stringplanmodifierImport+int64planmodifierImport, 1)
		}
	}

	return os.WriteFile(path, []byte(content), 0644)
}

func main() {
	groups := []struct {
		glob  string
		isTag bool
	}{
		{"internal/provider/generated_tag_*.go", true},
		{"internal/provider/generated_trigger_*.go", false},
		{"internal/provider/generated_variable_*.go", false},
	}
	total := 0
	for _, g := range groups {
		matches, err := filepath.Glob(g.glob)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		for _, m := range matches {
			if strings.HasSuffix(m, "_test.go") {
				continue
			}
			if err := processFile(m, g.isTag); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			total++
		}
	}
	fmt.Printf("processed %d files\n", total)
}
```

- [ ] **Step 3: Run the script**

Run: `go run tools/gen/tmp_bulk_insert.go`
Expected: `processed 64 files`

- [ ] **Step 4: Format all touched files**

Run: `gofmt -w internal/provider/generated_tag_*.go internal/provider/generated_trigger_*.go internal/provider/generated_variable_*.go`
Expected: no output (gofmt writes in place silently on success)

- [ ] **Step 5: Spot-check one file per kind**

Run: `sed -n '1,60p' internal/provider/generated_tag_addthis.go`
Expected: see `"description"` right after `"name"`, and `"priority"` right after `"block_trigger_ids"`, plus the new `int64planmodifier` import, all gofmt-aligned.

Run: `sed -n '1,35p' internal/provider/generated_trigger_pageview.go`
Expected: see `"description"` right after `"name"`.

Run: `sed -n '1,40p' internal/provider/generated_variable_constant.go`
Expected: see `"description"` right after `"name"`, before `"default_value"`.

- [ ] **Step 6: Delete the throwaway script**

Run: `rm tools/gen/tmp_bulk_insert.go`

- [ ] **Step 7: Build and vet the whole repo**

Run: `go build -o /dev/null ./... 2>&1 | head -80`
Expected: no errors (the `matomo.TagParams{}`/`TriggerParams{}`/`VariableParams{}` "unknown field" errors from Task 3's step 4 are now gone, since every generated model's `ToParams()`/`FromParams()` is untouched and the shared runtime's new `Description:`/`Priority:` fields only reference `common.Description`/`common.Priority`, which now exist via the embedded, updated `typedTagCommon`/etc.)

Run: `go vet ./...`
Expected: no output

- [ ] **Step 8: Run the full unit test suite**

Run: `go test ./... -v -count=1 2>&1 | tail -60`
Expected: PASS (all packages)

- [ ] **Step 9: Commit**

```bash
git add internal/provider/generated_tag_*.go internal/provider/generated_trigger_*.go internal/provider/generated_variable_*.go
git commit -m "feat: regenerate typed tag/trigger/variable resources with description/priority"
```

---

## Task 6: Generic `matomo_tagmanager_tag` resource - description and priority

**Files:**
- Modify: `internal/provider/resource_tagmanager_tag.go`

**Interfaces:**
- Consumes: `matomo.TagParams.Description/Priority` (Task 1).
- Produces: `tagManagerTagResourceModel.Description types.String`, `tagManagerTagResourceModel.Priority types.Int64`.

- [ ] **Step 1: Add fields to the model struct**

```go
type tagManagerTagResourceModel struct {
	ID              types.String         `tfsdk:"id"`
	ContainerID     types.String         `tfsdk:"container_id"`
	Type            types.String         `tfsdk:"type"`
	Name            types.String         `tfsdk:"name"`
	Status          types.String         `tfsdk:"status"`
	Description     types.String         `tfsdk:"description"`
	Priority        types.Int64          `tfsdk:"priority"`
	FireTriggerIDs  []types.String       `tfsdk:"fire_trigger_ids"`
	BlockTriggerIDs []types.String       `tfsdk:"block_trigger_ids"`
	Parameter       []tagParameterModel  `tfsdk:"parameter"`
	ParameterList   []parameterListModel `tfsdk:"parameter_list"`
}
```

- [ ] **Step 2: Add schema attributes**

Add the `"int64planmodifier"` import (`"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"`) to the file's import block, then add these two attributes to the `Attributes` map, right after `"status"`:

```go
			"description": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Optional free-text description, shown in Matomo's Tag Manager UI.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"priority": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Execution priority - lower values fire earlier when multiple tags fire on the same trigger. Matomo defaults to 999 when unset.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
```

- [ ] **Step 3: Wire Create**

Right before the `params := parametersToMap(plan.Parameter)` line in `Create`, add:

```go
	description := ""
	if !plan.Description.IsUnknown() && !plan.Description.IsNull() {
		description = plan.Description.ValueString()
	}
	priority := int64(999)
	if !plan.Priority.IsUnknown() && !plan.Priority.IsNull() {
		priority = plan.Priority.ValueInt64()
	}
```

Add `Description`/`Priority` to the `matomo.TagParams` literal passed to `AddContainerTag`:

```go
	idTag, err := r.client.AddContainerTag(ctx, siteID, idContainer, versionID, matomo.TagParams{
		Type:            plan.Type.ValueString(),
		Name:            plan.Name.ValueString(),
		Description:     description,
		Priority:        int(priority),
		Parameters:      params,
		FireTriggerIDs:  fireIDs,
		BlockTriggerIDs: blockIDs,
	})
```

Right before `resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)` at the end of `Create`, add:

```go
	plan.Description = types.StringValue(description)
	plan.Priority = types.Int64Value(priority)
```

- [ ] **Step 4: Wire Read**

Right after `state.Status = types.StringValue(tag.Status)` in `Read`, add:

```go
	state.Description = types.StringValue(tag.Description)
	state.Priority = types.Int64Value(int64(tag.Priority))
```

- [ ] **Step 5: Wire Update**

Mirror Step 3's default-resolution right before `params := parametersToMap(plan.Parameter)` in `Update`:

```go
	description := ""
	if !plan.Description.IsUnknown() && !plan.Description.IsNull() {
		description = plan.Description.ValueString()
	}
	priority := int64(999)
	if !plan.Priority.IsUnknown() && !plan.Priority.IsNull() {
		priority = plan.Priority.ValueInt64()
	}
```

Add `Description`/`Priority` to the `matomo.TagParams` literal passed to `UpdateContainerTag`:

```go
	if err := r.client.UpdateContainerTag(ctx, siteID, idContainer, versionID, idTag, matomo.TagParams{
		Type:            plan.Type.ValueString(),
		Name:            plan.Name.ValueString(),
		Description:     description,
		Priority:        int(priority),
		Parameters:      params,
		FireTriggerIDs:  fireIDs,
		BlockTriggerIDs: blockIDs,
	}); err != nil {
```

Right before `resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)` at the end of `Update`, add:

```go
	plan.Description = types.StringValue(description)
	plan.Priority = types.Int64Value(priority)
```

- [ ] **Step 6: Build and vet**

Run: `go build -o /dev/null ./... && go vet ./internal/provider/...`
Expected: no errors

- [ ] **Step 7: Commit**

```bash
git add internal/provider/resource_tagmanager_tag.go
git commit -m "feat: add description and priority to matomo_tagmanager_tag"
```

---

## Task 7: Generic `matomo_tagmanager_trigger` resource - description

**Files:**
- Modify: `internal/provider/resource_tagmanager_trigger.go`

**Interfaces:**
- Consumes: `matomo.TriggerParams.Description` (Task 1).
- Produces: `tagManagerTriggerResourceModel.Description types.String`.

- [ ] **Step 1: Add field to the model struct**

```go
type tagManagerTriggerResourceModel struct {
	ID            types.String            `tfsdk:"id"`
	ContainerID   types.String            `tfsdk:"container_id"`
	Type          types.String            `tfsdk:"type"`
	Name          types.String            `tfsdk:"name"`
	Description   types.String            `tfsdk:"description"`
	Parameter     []tagParameterModel     `tfsdk:"parameter"`
	ParameterList []parameterListModel    `tfsdk:"parameter_list"`
	Condition     []triggerConditionModel `tfsdk:"condition"`
}
```

- [ ] **Step 2: Add schema attribute**

Add this attribute to the `Attributes` map, right after `"name"`:

```go
			"description": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Optional free-text description, shown in Matomo's Tag Manager UI.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
```

- [ ] **Step 3: Wire Create**

Right before `params := parametersToMap(plan.Parameter)` in `Create`, add:

```go
	description := ""
	if !plan.Description.IsUnknown() && !plan.Description.IsNull() {
		description = plan.Description.ValueString()
	}
```

Add `Description` to the `matomo.TriggerParams` literal passed to `AddContainerTrigger`:

```go
	idTrigger, err := r.client.AddContainerTrigger(ctx, siteID, idContainer, versionID, matomo.TriggerParams{
		Type:        plan.Type.ValueString(),
		Name:        plan.Name.ValueString(),
		Description: description,
		Parameters:  params,
		Conditions:  conditionsToParams(plan.Condition),
	})
```

Right before `resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)` at the end of `Create`, add:

```go
	plan.Description = types.StringValue(description)
```

- [ ] **Step 4: Wire Read**

Right after `state.Name = types.StringValue(trig.Name)` in `Read`, add:

```go
	state.Description = types.StringValue(trig.Description)
```

- [ ] **Step 5: Wire Update**

Mirror Step 3's default-resolution right before `params := parametersToMap(plan.Parameter)` in `Update`:

```go
	description := ""
	if !plan.Description.IsUnknown() && !plan.Description.IsNull() {
		description = plan.Description.ValueString()
	}
```

Add `Description` to the `matomo.TriggerParams` literal passed to `UpdateContainerTrigger`:

```go
	if err := r.client.UpdateContainerTrigger(ctx, siteID, idContainer, versionID, idTrigger, matomo.TriggerParams{
		Type:        plan.Type.ValueString(),
		Name:        plan.Name.ValueString(),
		Description: description,
		Parameters:  params,
		Conditions:  conditionsToParams(plan.Condition),
	}); err != nil {
```

Right before `resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)` at the end of `Update`, add:

```go
	plan.Description = types.StringValue(description)
```

- [ ] **Step 6: Build and vet**

Run: `go build -o /dev/null ./... && go vet ./internal/provider/...`
Expected: no errors

- [ ] **Step 7: Commit**

```bash
git add internal/provider/resource_tagmanager_trigger.go
git commit -m "feat: add description to matomo_tagmanager_trigger"
```

---

## Task 8: Generic `matomo_tagmanager_variable` resource - description

**Files:**
- Modify: `internal/provider/resource_tagmanager_variable.go`

**Interfaces:**
- Consumes: `matomo.VariableParams.Description` (Task 1).
- Produces: `tagManagerVariableResourceModel.Description types.String`.

- [ ] **Step 1: Add field to the model struct**

```go
type tagManagerVariableResourceModel struct {
	ID            types.String         `tfsdk:"id"`
	ContainerID   types.String         `tfsdk:"container_id"`
	Type          types.String         `tfsdk:"type"`
	Name          types.String         `tfsdk:"name"`
	Description   types.String         `tfsdk:"description"`
	DefaultValue  types.String         `tfsdk:"default_value"`
	Parameter     []tagParameterModel  `tfsdk:"parameter"`
	ParameterList []parameterListModel `tfsdk:"parameter_list"`
}
```

- [ ] **Step 2: Add schema attribute**

Add this attribute to the `Attributes` map, right after `"name"`:

```go
			"description": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Optional free-text description, shown in Matomo's Tag Manager UI.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
```

- [ ] **Step 3: Wire Create**

Right before `params := parametersToMap(plan.Parameter)` in `Create`, add:

```go
	description := ""
	if !plan.Description.IsUnknown() && !plan.Description.IsNull() {
		description = plan.Description.ValueString()
	}
```

Add `Description` to the `matomo.VariableParams` literal passed to `AddContainerVariable`:

```go
	idVariable, err := r.client.AddContainerVariable(ctx, siteID, idContainer, versionID, matomo.VariableParams{
		Type:         plan.Type.ValueString(),
		Name:         plan.Name.ValueString(),
		Description:  description,
		Parameters:   params,
		DefaultValue: defaultValue,
	})
```

`Create` already does a `GetContainerVariable` read-back for `default_value` - reuse it for `description` too. Change:

```go
	v, err := r.client.GetContainerVariable(ctx, siteID, idContainer, versionID, idVariable)
	if err != nil {
		resp.Diagnostics.AddError("Error reading back created Matomo Tag Manager variable", err.Error())
		return
	}
	plan.DefaultValue = types.StringValue(v.DefaultValue)
	plan.Description = types.StringValue(v.Description)
```

- [ ] **Step 4: Wire Read**

Right after `state.Name = types.StringValue(v.Name)` in `Read`, add:

```go
	state.Description = types.StringValue(v.Description)
```

- [ ] **Step 5: Wire Update**

Mirror Step 3's default-resolution right before `params := parametersToMap(plan.Parameter)` in `Update`:

```go
	description := ""
	if !plan.Description.IsUnknown() && !plan.Description.IsNull() {
		description = plan.Description.ValueString()
	}
```

Add `Description` to the `matomo.VariableParams` literal passed to `UpdateContainerVariable`:

```go
	if err := r.client.UpdateContainerVariable(ctx, siteID, idContainer, versionID, idVariable, matomo.VariableParams{
		Type:         plan.Type.ValueString(),
		Name:         plan.Name.ValueString(),
		Description:  description,
		Parameters:   params,
		DefaultValue: defaultValue,
	}); err != nil {
```

`Update` already does a `GetContainerVariable` read-back for `default_value` - reuse it for `description` too. Change:

```go
	v, err := r.client.GetContainerVariable(ctx, siteID, idContainer, versionID, idVariable)
	if err != nil {
		resp.Diagnostics.AddError("Error reading back updated Matomo Tag Manager variable", err.Error())
		return
	}
	plan.DefaultValue = types.StringValue(v.DefaultValue)
	plan.Description = types.StringValue(v.Description)
```

- [ ] **Step 6: Build and vet**

Run: `go build -o /dev/null ./... && go vet ./internal/provider/...`
Expected: no errors

- [ ] **Step 7: Commit**

```bash
git add internal/provider/resource_tagmanager_variable.go
git commit -m "feat: add description to matomo_tagmanager_variable"
```

---

## Task 9: `matomo_tagmanager_container` resource - three flags

**Files:**
- Modify: `internal/provider/resource_tagmanager_container.go`

**Interfaces:**
- Consumes: `client.AddContainer`/`client.UpdateContainer`'s new three trailing `bool` parameters (Task 2). Replaces the `false, false, false` placeholders Task 2 Step 4 introduced.

- [ ] **Step 1: Add fields to the model struct**

```go
type tagManagerContainerResourceModel struct {
	ID                                 types.String `tfsdk:"id"`
	SiteID                             types.String `tfsdk:"site_id"`
	Context                            types.String `tfsdk:"context"`
	Name                               types.String `tfsdk:"name"`
	Description                        types.String `tfsdk:"description"`
	IgnoreGtmDataLayer                 types.Bool   `tfsdk:"ignore_gtm_data_layer"`
	IsTagFireLimitAllowedInPreviewMode types.Bool   `tfsdk:"is_tag_fire_limit_allowed_in_preview_mode"`
	ActivelySyncGtmDataLayer           types.Bool   `tfsdk:"actively_sync_gtm_data_layer"`
}
```

- [ ] **Step 2: Add schema attributes and the boolplanmodifier import**

Add `"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"` to the import block. Add these three attributes to the `Attributes` map, right after `"description"`:

```go
			"ignore_gtm_data_layer": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "If true, this container ignores an existing GTM-style dataLayer on the page instead of reusing it.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"is_tag_fire_limit_allowed_in_preview_mode": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "If true, a tag's configured fire limit is also enforced while in preview/debug mode.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"actively_sync_gtm_data_layer": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "If true, this container actively keeps an existing GTM-style dataLayer in sync rather than only reading it once.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
```

- [ ] **Step 3: Wire Create**

Replace the whole `Create` function body from the `description :=` line through the `r.client.AddContainer(...)` call with:

```go
	description := ""
	if !plan.Description.IsUnknown() && !plan.Description.IsNull() {
		description = plan.Description.ValueString()
	}
	ignoreGtmDataLayer := false
	if !plan.IgnoreGtmDataLayer.IsUnknown() && !plan.IgnoreGtmDataLayer.IsNull() {
		ignoreGtmDataLayer = plan.IgnoreGtmDataLayer.ValueBool()
	}
	isTagFireLimitAllowedInPreviewMode := false
	if !plan.IsTagFireLimitAllowedInPreviewMode.IsUnknown() && !plan.IsTagFireLimitAllowedInPreviewMode.IsNull() {
		isTagFireLimitAllowedInPreviewMode = plan.IsTagFireLimitAllowedInPreviewMode.ValueBool()
	}
	activelySyncGtmDataLayer := false
	if !plan.ActivelySyncGtmDataLayer.IsUnknown() && !plan.ActivelySyncGtmDataLayer.IsNull() {
		activelySyncGtmDataLayer = plan.ActivelySyncGtmDataLayer.ValueBool()
	}

	idContainer, err := r.client.AddContainer(ctx, siteID, plan.Context.ValueString(), plan.Name.ValueString(), description, ignoreGtmDataLayer, isTagFireLimitAllowedInPreviewMode, activelySyncGtmDataLayer)
```

Right after `plan.Description = types.StringValue(description)` (already present) at the end of `Create`, add:

```go
	plan.IgnoreGtmDataLayer = types.BoolValue(ignoreGtmDataLayer)
	plan.IsTagFireLimitAllowedInPreviewMode = types.BoolValue(isTagFireLimitAllowedInPreviewMode)
	plan.ActivelySyncGtmDataLayer = types.BoolValue(activelySyncGtmDataLayer)
```

- [ ] **Step 4: Wire Read**

Right after `state.Description = types.StringValue(ct.Description)` in `Read`, add:

```go
	state.IgnoreGtmDataLayer = types.BoolValue(ct.IgnoreGtmDataLayer)
	state.IsTagFireLimitAllowedInPreviewMode = types.BoolValue(ct.IsTagFireLimitAllowedInPreviewMode)
	state.ActivelySyncGtmDataLayer = types.BoolValue(ct.ActivelySyncGtmDataLayer)
```

- [ ] **Step 5: Wire Update**

Replace the whole `Update` function body from the `description :=` line through the `r.client.UpdateContainer(...)` call with:

```go
	description := ""
	if !plan.Description.IsUnknown() && !plan.Description.IsNull() {
		description = plan.Description.ValueString()
	}
	ignoreGtmDataLayer := false
	if !plan.IgnoreGtmDataLayer.IsUnknown() && !plan.IgnoreGtmDataLayer.IsNull() {
		ignoreGtmDataLayer = plan.IgnoreGtmDataLayer.ValueBool()
	}
	isTagFireLimitAllowedInPreviewMode := false
	if !plan.IsTagFireLimitAllowedInPreviewMode.IsUnknown() && !plan.IsTagFireLimitAllowedInPreviewMode.IsNull() {
		isTagFireLimitAllowedInPreviewMode = plan.IsTagFireLimitAllowedInPreviewMode.ValueBool()
	}
	activelySyncGtmDataLayer := false
	if !plan.ActivelySyncGtmDataLayer.IsUnknown() && !plan.ActivelySyncGtmDataLayer.IsNull() {
		activelySyncGtmDataLayer = plan.ActivelySyncGtmDataLayer.ValueBool()
	}

	if err := r.client.UpdateContainer(ctx, siteID, idContainer, plan.Name.ValueString(), description, ignoreGtmDataLayer, isTagFireLimitAllowedInPreviewMode, activelySyncGtmDataLayer); err != nil {
```

Right after `plan.Description = types.StringValue(description)` (already present) at the end of `Update`, add:

```go
	plan.IgnoreGtmDataLayer = types.BoolValue(ignoreGtmDataLayer)
	plan.IsTagFireLimitAllowedInPreviewMode = types.BoolValue(isTagFireLimitAllowedInPreviewMode)
	plan.ActivelySyncGtmDataLayer = types.BoolValue(activelySyncGtmDataLayer)
```

- [ ] **Step 6: Build and vet**

Run: `go build -o /dev/null ./... && go vet ./internal/provider/...`
Expected: no errors

- [ ] **Step 7: Run the full unit test suite**

Run: `go test ./... -v -count=1 2>&1 | tail -60`
Expected: PASS (all packages)

- [ ] **Step 8: Commit**

```bash
git add internal/provider/resource_tagmanager_container.go
git commit -m "feat: add ignore_gtm_data_layer/actively_sync_gtm_data_layer/is_tag_fire_limit_allowed_in_preview_mode to matomo_tagmanager_container"
```

---

## Task 10: Documentation

**Files:**
- Modify: `docs/resources/tagmanager_container.md`
- Modify: `docs/resources/tagmanager_tag.md`
- Modify: `docs/resources/tagmanager_trigger.md`
- Modify: `docs/resources/tagmanager_variable.md`
- Modify: all 31 `docs/resources/tagmanager_tag_*.md` (excluding the four above)
- Modify: all 17 `docs/resources/tagmanager_trigger_*.md` (excluding the four above)
- Modify: all 16 `docs/resources/tagmanager_variable_*.md` (excluding the four above)
- Create (temporary, deleted at end of task): `tools/gen/tmp_bulk_docs.go`

This sandbox cannot run `make docs` (network-blocked, confirmed in a prior session) - CI's `make docs` drift check (`.github/workflows/ci.yml:21-25`) is the real verification; these hand-reconstructed docs must match what `tfplugindocs` would generate. tfplugindocs sorts each Required/Optional/Read-Only bullet list alphabetically by attribute name, quotes the attribute name in backticks, and labels the type as `(String)`, `(Number)` (for `Int64Attribute`), or `(Boolean)` (for `BoolAttribute`) followed by the attribute's `Description` text (already confirmed against every existing `docs/resources/*.md` file in this repo).

- [ ] **Step 1: Hand-edit the four generic/container docs**

In `docs/resources/tagmanager_container.md`, change the `### Optional` section from:

```markdown
### Optional

- `description` (String) The container's description.
```

to:

```markdown
### Optional

- `actively_sync_gtm_data_layer` (Boolean) If true, this container actively keeps an existing GTM-style dataLayer in sync rather than only reading it once.
- `description` (String) The container's description.
- `ignore_gtm_data_layer` (Boolean) If true, this container ignores an existing GTM-style dataLayer on the page instead of reusing it.
- `is_tag_fire_limit_allowed_in_preview_mode` (Boolean) If true, a tag's configured fire limit is also enforced while in preview/debug mode.
```

In `docs/resources/tagmanager_tag.md`, change the `### Optional` section from:

```markdown
### Optional

- `block_trigger_ids` (List of String) Trigger ids (matomo_tagmanager_trigger.x.id) that block this tag from firing. Note: writing an explicit empty list (`[]`) rather than omitting this attribute will produce a one-time diff to null on the first refresh after apply; this is harmless and converges after one plan/apply cycle.
- `parameter` (Block List) Type-specific configuration as name/value pairs. (see [below for nested schema](#nestedblock--parameter))
- `parameter_list` (Block List) A single named parameter whose value is a list of rows, each with arbitrary key/value items - for parameter types the generic parameter{} block cannot represent (e.g. Matomo's UI_CONTROL_MULTI_TUPLE fields, which need each row's fields sent as name[i][key]=value, not a flat list). Prefer a typed resource over this when one exists for your type - a typed resource's real nested block (e.g. custom_dimension{index,value}) is validated and self-documenting; this generic form is not. (see [below for nested schema](#nestedblock--parameter_list))
- `status` (String) "active" or "paused". Changing this edits the draft version only — like every other field on this resource, it has no effect on a live container until a new version is created and published.
```

to:

```markdown
### Optional

- `block_trigger_ids` (List of String) Trigger ids (matomo_tagmanager_trigger.x.id) that block this tag from firing. Note: writing an explicit empty list (`[]`) rather than omitting this attribute will produce a one-time diff to null on the first refresh after apply; this is harmless and converges after one plan/apply cycle.
- `description` (String) Optional free-text description, shown in Matomo's Tag Manager UI.
- `parameter` (Block List) Type-specific configuration as name/value pairs. (see [below for nested schema](#nestedblock--parameter))
- `parameter_list` (Block List) A single named parameter whose value is a list of rows, each with arbitrary key/value items - for parameter types the generic parameter{} block cannot represent (e.g. Matomo's UI_CONTROL_MULTI_TUPLE fields, which need each row's fields sent as name[i][key]=value, not a flat list). Prefer a typed resource over this when one exists for your type - a typed resource's real nested block (e.g. custom_dimension{index,value}) is validated and self-documenting; this generic form is not. (see [below for nested schema](#nestedblock--parameter_list))
- `priority` (Number) Execution priority - lower values fire earlier when multiple tags fire on the same trigger. Matomo defaults to 999 when unset.
- `status` (String) "active" or "paused". Changing this edits the draft version only — like every other field on this resource, it has no effect on a live container until a new version is created and published.
```

In `docs/resources/tagmanager_trigger.md`, change the `### Optional` section from:

```markdown
### Optional

- `condition` (Block List) Conditions that must all match for this trigger to fire. (see [below for nested schema](#nestedblock--condition))
- `parameter` (Block List) Type-specific configuration as name/value pairs. (see [below for nested schema](#nestedblock--parameter))
- `parameter_list` (Block List) A single named parameter whose value is a list of rows, each with arbitrary key/value items - for parameter types the generic parameter{} block cannot represent (e.g. Matomo's UI_CONTROL_MULTI_TUPLE fields, which need each row's fields sent as name[i][key]=value, not a flat list). Prefer a typed resource over this when one exists for your type - a typed resource's real nested block (e.g. custom_dimension{index,value}) is validated and self-documenting; this generic form is not. (see [below for nested schema](#nestedblock--parameter_list))
```

to:

```markdown
### Optional

- `condition` (Block List) Conditions that must all match for this trigger to fire. (see [below for nested schema](#nestedblock--condition))
- `description` (String) Optional free-text description, shown in Matomo's Tag Manager UI.
- `parameter` (Block List) Type-specific configuration as name/value pairs. (see [below for nested schema](#nestedblock--parameter))
- `parameter_list` (Block List) A single named parameter whose value is a list of rows, each with arbitrary key/value items - for parameter types the generic parameter{} block cannot represent (e.g. Matomo's UI_CONTROL_MULTI_TUPLE fields, which need each row's fields sent as name[i][key]=value, not a flat list). Prefer a typed resource over this when one exists for your type - a typed resource's real nested block (e.g. custom_dimension{index,value}) is validated and self-documenting; this generic form is not. (see [below for nested schema](#nestedblock--parameter_list))
```

In `docs/resources/tagmanager_variable.md`, change the `### Optional` section from:

```markdown
### Optional

- `default_value` (String) Value used when the variable cannot be resolved.
- `parameter` (Block List) Type-specific configuration as name/value pairs. (see [below for nested schema](#nestedblock--parameter))
- `parameter_list` (Block List) A single named parameter whose value is a list of rows, each with arbitrary key/value items - for parameter types the generic parameter{} block cannot represent (e.g. Matomo's UI_CONTROL_MULTI_TUPLE fields, which need each row's fields sent as name[i][key]=value, not a flat list). Prefer a typed resource over this when one exists for your type - a typed resource's real nested block (e.g. custom_dimension{index,value}) is validated and self-documenting; this generic form is not. (see [below for nested schema](#nestedblock--parameter_list))
```

to:

```markdown
### Optional

- `default_value` (String) Value used when the variable cannot be resolved.
- `description` (String) Optional free-text description, shown in Matomo's Tag Manager UI.
- `parameter` (Block List) Type-specific configuration as name/value pairs. (see [below for nested schema](#nestedblock--parameter))
- `parameter_list` (Block List) A single named parameter whose value is a list of rows, each with arbitrary key/value items - for parameter types the generic parameter{} block cannot represent (e.g. Matomo's UI_CONTROL_MULTI_TUPLE fields, which need each row's fields sent as name[i][key]=value, not a flat list). Prefer a typed resource over this when one exists for your type - a typed resource's real nested block (e.g. custom_dimension{index,value}) is validated and self-documenting; this generic form is not. (see [below for nested schema](#nestedblock--parameter_list))
```

- [ ] **Step 2: Write the bulk docs-insertion script**

Create `tools/gen/tmp_bulk_docs.go`:

```go
//go:build ignore

// tmp_bulk_docs.go inserts "- `description` (String) ..." (all kinds) and
// "- `priority` (Number) ..." (tag kind) into the "### Optional" bullet
// list of every typed resource's docs page, at the correct alphabetical
// position - matching what `tfplugindocs generate` (not runnable in this
// sandbox - network-blocked) would itself produce. Run once via
// `go run tools/gen/tmp_bulk_docs.go` from the repo root, then delete
// this file.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const descriptionLine = "- `description` (String) Optional free-text description, shown in Matomo's Tag Manager UI."
const priorityLine = "- `priority` (Number) Execution priority - lower values fire earlier when multiple tags fire on the same trigger. Matomo defaults to 999 when unset."

// bulletName extracts the backtick-quoted attribute name a doc bullet
// line starts with, e.g. "- `status` (String) ..." -> "status".
func bulletName(line string) string {
	start := strings.Index(line, "`")
	end := strings.Index(line[start+1:], "`")
	return line[start+1 : start+1+end]
}

func insertSorted(section []string, newLine string) []string {
	name := bulletName(newLine)
	i := sort.Search(len(section), func(i int) bool {
		return bulletName(section[i]) > name
	})
	out := make([]string, 0, len(section)+1)
	out = append(out, section[:i]...)
	out = append(out, newLine)
	out = append(out, section[i:]...)
	return out
}

func processFile(path string, isTag bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")

	optIdx := -1
	for i, l := range lines {
		if l == "### Optional" {
			optIdx = i
			break
		}
	}
	if optIdx == -1 {
		return fmt.Errorf("%s: no ### Optional section found", path)
	}

	// The Optional section's bullets run from optIdx+2 (skipping the
	// blank line after the heading) until the next blank line.
	start := optIdx + 2
	end := start
	for end < len(lines) && strings.HasPrefix(lines[end], "- ") {
		end++
	}
	section := append([]string{}, lines[start:end]...)

	section = insertSorted(section, descriptionLine)
	if isTag {
		section = insertSorted(section, priorityLine)
	}

	newLines := append([]string{}, lines[:start]...)
	newLines = append(newLines, section...)
	newLines = append(newLines, lines[end:]...)

	return os.WriteFile(path, []byte(strings.Join(newLines, "\n")), 0644)
}

func main() {
	groups := []struct {
		glob  string
		isTag bool
	}{
		{"docs/resources/tagmanager_tag_*.md", true},
		{"docs/resources/tagmanager_trigger_*.md", false},
		{"docs/resources/tagmanager_variable_*.md", false},
	}
	total := 0
	for _, g := range groups {
		matches, err := filepath.Glob(g.glob)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		for _, m := range matches {
			if err := processFile(m, g.isTag); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			total++
		}
	}
	fmt.Printf("processed %d docs files\n", total)
}
```

- [ ] **Step 3: Run the script**

Run: `go run tools/gen/tmp_bulk_docs.go`
Expected: `processed 64 docs files`

- [ ] **Step 4: Spot-check one file per kind**

Run: `sed -n '43,60p' docs/resources/tagmanager_tag_addthis.md`
Expected: `### Optional` list now includes `description` (alphabetically between `add_this_parent_selector` and `status`) and no `priority` change needed here (AddThis has no other optional attributes starting after "d" alphabetically before "priority" - confirm ordering reads correctly by eye).

Run: `cat docs/resources/tagmanager_trigger_pageview.md | sed -n '/### Optional/,/### Read-Only/p'`
Expected:
```markdown
### Optional

- `condition` (Block List) Conditions that must all match for this trigger to fire. (see [below for nested schema](#nestedblock--condition))
- `description` (String) Optional free-text description, shown in Matomo's Tag Manager UI.

### Read-Only
```

Run: `cat docs/resources/tagmanager_variable_constant.md | sed -n '/### Optional/,/### Read-Only/p'`
Expected `description` present, alphabetically before `default_value`... **check this by eye**: alphabetically `default_value` < `description` (compare "default" vs "descri": 'e'=='e', 'f' < 's', so "default_value" sorts before "description"). If the script placed them in the wrong order, fix `docs/resources/tagmanager_variable_constant.md` (and check whether the same reordering is needed in every variable file) by hand before proceeding - the `insertSorted` helper sorts correctly by construction, so this check should already pass; it exists to catch a mistaken assumption before it propagates to all 16 variable files.

- [ ] **Step 5: Delete the throwaway script**

Run: `rm tools/gen/tmp_bulk_docs.go`

- [ ] **Step 6: Commit**

```bash
git add docs/resources/tagmanager_container.md docs/resources/tagmanager_tag.md docs/resources/tagmanager_trigger.md docs/resources/tagmanager_variable.md docs/resources/tagmanager_tag_*.md docs/resources/tagmanager_trigger_*.md docs/resources/tagmanager_variable_*.md
git commit -m "docs: document description/priority/container flags"
```

---

## Task 11: Acceptance tests

**Files:**
- Modify: `internal/provider/resource_tagmanager_container_acc_test.go`
- Modify: `internal/provider/resource_tagmanager_tag_acc_test.go`
- Modify: `internal/provider/resource_tagmanager_trigger_acc_test.go`
- Modify: `internal/provider/resource_tagmanager_variable_acc_test.go`
- Modify: one typed tag acceptance test file, e.g. `internal/provider/generated_tag_customhtml_acc_test.go`
- Modify: one typed trigger acceptance test file, e.g. `internal/provider/generated_trigger_pageview_acc_test.go`
- Modify: one typed variable acceptance test file, e.g. `internal/provider/generated_variable_constant_acc_test.go`

These tests require a live Matomo instance (Docker) not available in this sandbox - they cannot be run locally here. They run in CI's `acceptance.yml` workflow. Extend each resource's existing `_basic` test's `Config` and `Check` rather than adding new test functions, per this project's established convention (confirmed in the design spec, section 6).

- [ ] **Step 1: Read the existing container acceptance test to find the `_basic` step to extend**

Run: `grep -n "func TestAccTagManagerContainerResource_basic" -A 40 internal/provider/resource_tagmanager_container_acc_test.go`

Add `ignore_gtm_data_layer = true` and `actively_sync_gtm_data_layer = true` to the first step's `Config` HCL (inside the `matomo_tagmanager_container.test` block, alongside the existing `description` line if present, or added fresh), and add these checks to the step's `Check`:

```go
						resource.TestCheckResourceAttr("matomo_tagmanager_container.test", "ignore_gtm_data_layer", "true"),
						resource.TestCheckResourceAttr("matomo_tagmanager_container.test", "actively_sync_gtm_data_layer", "true"),
						resource.TestCheckResourceAttr("matomo_tagmanager_container.test", "is_tag_fire_limit_allowed_in_preview_mode", "false"),
```

- [ ] **Step 2: Extend the generic tag acceptance test**

Run: `grep -n "func TestAccTagManagerTagResource_basic" -A 40 internal/provider/resource_tagmanager_tag_acc_test.go`

Add `description = "acceptance test tag description"` and `priority = 5` to the first step's `matomo_tagmanager_tag.test` config block, and add:

```go
						resource.TestCheckResourceAttr("matomo_tagmanager_tag.test", "description", "acceptance test tag description"),
						resource.TestCheckResourceAttr("matomo_tagmanager_tag.test", "priority", "5"),
```

- [ ] **Step 3: Extend the generic trigger acceptance test**

Run: `grep -n "func TestAccTagManagerTriggerResource_basic" -A 40 internal/provider/resource_tagmanager_trigger_acc_test.go`

Add `description = "acceptance test trigger description"` to the first step's `matomo_tagmanager_trigger.test` config block, and add:

```go
						resource.TestCheckResourceAttr("matomo_tagmanager_trigger.test", "description", "acceptance test trigger description"),
```

- [ ] **Step 4: Extend the generic variable acceptance test**

Run: `grep -n "func TestAccTagManagerVariableResource_basic" -A 40 internal/provider/resource_tagmanager_variable_acc_test.go`

Add `description = "acceptance test variable description"` to the first step's `matomo_tagmanager_variable.test` config block, and add:

```go
						resource.TestCheckResourceAttr("matomo_tagmanager_variable.test", "description", "acceptance test variable description"),
```

- [ ] **Step 5: Extend one typed tag acceptance test**

Run: `cat internal/provider/generated_tag_customhtml_acc_test.go`

Add `description = "acceptance test description"` and `priority = 3` to the `matomo_tagmanager_tag_customhtml.test` config block in its first step, and add:

```go
						resource.TestCheckResourceAttr("matomo_tagmanager_tag_customhtml.test", "description", "acceptance test description"),
						resource.TestCheckResourceAttr("matomo_tagmanager_tag_customhtml.test", "priority", "3"),
```

- [ ] **Step 6: Extend one typed trigger acceptance test**

Run: `cat internal/provider/generated_trigger_pageview_acc_test.go`

Add `description = "acceptance test description"` to the `matomo_tagmanager_trigger_pageview.test` config block in its first step, and add:

```go
						resource.TestCheckResourceAttr("matomo_tagmanager_trigger_pageview.test", "description", "acceptance test description"),
```

- [ ] **Step 7: Extend one typed variable acceptance test**

Run: `cat internal/provider/generated_variable_constant_acc_test.go`

Add `description = "acceptance test description"` to the `matomo_tagmanager_variable_constant.test` config block in its first step, and add:

```go
						resource.TestCheckResourceAttr("matomo_tagmanager_variable_constant.test", "description", "acceptance test description"),
```

- [ ] **Step 8: Build (acceptance tests are build-tagged/skip without a live server, but must compile)**

Run: `go build -o /dev/null ./... && go vet ./internal/provider/...`
Expected: no errors

- [ ] **Step 9: Commit**

```bash
git add internal/provider/resource_tagmanager_container_acc_test.go internal/provider/resource_tagmanager_tag_acc_test.go internal/provider/resource_tagmanager_trigger_acc_test.go internal/provider/resource_tagmanager_variable_acc_test.go internal/provider/generated_tag_customhtml_acc_test.go internal/provider/generated_trigger_pageview_acc_test.go internal/provider/generated_variable_constant_acc_test.go
git commit -m "test: cover description/priority/container flags in acceptance tests"
```

---

## Task 12: Full verification pass

**Files:** none (verification only)

- [ ] **Step 1: Full build**

Run: `go build -o /dev/null ./...`
Expected: no errors

- [ ] **Step 2: Full vet**

Run: `go vet ./...`
Expected: no output

- [ ] **Step 3: Full unit test suite**

Run: `go test ./... -v -count=1 2>&1 | tail -100`
Expected: all packages PASS

- [ ] **Step 4: gofmt check (matches CI's expectations)**

Run: `gofmt -l . | grep -v '^$'`
Expected: no output (empty - everything is gofmt-clean)

- [ ] **Step 5: Confirm no throwaway files remain**

Run: `git status --short`
Expected: `tools/gen/tmp_bulk_insert.go` and `tools/gen/tmp_bulk_docs.go` do not appear (both deleted in Tasks 5 and 10); only intentional changes remain staged/committed.

- [ ] **Step 6: Push and let CI confirm what this sandbox cannot check locally**

This is a reminder, not a command to run here: `go build`, `go test ./...`, and `gofmt` all pass locally per Steps 1-4, but CI's `make docs` drift check and the full acceptance-test suite (`acceptance.yml`, live Matomo via Docker) can only be confirmed once pushed - watch CI after pushing this branch and fix any diff it reports (most likely: a docs-formatting mismatch from Task 10's hand-reconstruction, or a live-API surprise in the container flags' bool wire encoding per `tagmanager_containers.go`'s new fields, consistent with this project's established iterate-via-live-CI pattern for exactly this class of uncertainty).
