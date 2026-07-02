# Tag Manager actions — design spec

## 1. Goal and scope

Phase 7 of the original provider design (`docs/superpowers/specs/2026-06-30-matomo-tagmanager-provider-design.md`
§7, §12). Adds four unlinked Terraform actions wrapping Matomo Tag
Manager's version/publish/preview lifecycle operations:

- `matomo_tagmanager_create_container_version`
- `matomo_tagmanager_publish_container_version`
- `matomo_tagmanager_enable_preview_mode`
- `matomo_tagmanager_disable_preview_mode`

Out of scope: any change to the existing tag/trigger/variable resources
or the Phase 6 discovery data sources.

## 2. Platform constraint: actions cannot chain outputs

The original spec's §7 HCL example chains `create_container_version`'s
output into `publish_container_version`'s config:

```hcl
action "matomo_tagmanager_publish_container_version" "go_live" {
  config {
    container_version_id = action.matomo_tagmanager_create_container_version.release.output.container_version_id
  }
}
```

This is not possible with the actions API in this provider's installed
`terraform-plugin-framework` version (v1.19.0), confirmed by reading the
SDK source directly:

- `action.Schema` (`action/schema.go`) has only `Attributes`/`Blocks` — a
  single config-in schema, no separate output/result schema. Its own doc
  comment states: "An action currently cannot cause changes to resource
  state."
- `action.InvokeResponse` (`action/invoke.go`) has only `Diagnostics` and
  `SendProgress func(InvokeProgressEvent)` — no field for returning a
  result value practitioners could reference elsewhere.

Actions in this framework version are fire-and-forget side effects, not
value producers. This is a hard platform constraint, not a design
choice, and it reshapes `publish_container_version` (see §4.2).

## 3. Matomo API methods (confirmed against `matomo-org/tag-manager` `API.php`, branch `5.x-dev`)

- `createContainerVersion($idSite, $idContainer, $name, $description = '', $idContainerVersion = null)`
  — snapshots a version (by default, from the current draft) and returns
  its id. `$name` is required, validated to 1-50 characters by Matomo.
  Exact JSON wire shape of the returned id (bare int vs. `{"value": N}`)
  is unconfirmed from source reading alone; to be confirmed against live
  Matomo during acceptance testing, per this project's established
  practice of never guessing wire format. Every other Matomo "create X"
  endpoint already wrapped in this codebase (`AddContainer`,
  `AddContainerTag`, etc.) uses `{"value": N}`, so that's the working
  assumption pending live confirmation.
- `publishContainerVersion($idSite, $idContainer, $idContainerVersion, $environment)`
  — publishes an existing version snapshot to an environment. Requires a
  real, previously-created version id — confirmed you cannot publish the
  mutable draft directly ("The draft itself is mutable; you cannot
  publish draft changes directly — you must first create a version
  snapshot via createContainerVersion()"). Returns an array (release
  details); this provider discards the response body entirely (see §4.2)
  since there's no way to expose it back to Terraform anyway — only the
  error envelope matters.
- `enablePreviewMode($idSite, $idContainer, $idContainerVersion = null)` /
  `disablePreviewMode($idSite, $idContainer, $idContainerVersion = null)`
  — both default `$idContainerVersion` to the draft when omitted; no
  return value. Per the approved design decision, this provider always
  omits `$idContainerVersion` (always targets the draft).

## 4. New client methods

New file `internal/matomo/tagmanager_container_versions.go`:

```go
package matomo

import (
	"context"
	"net/url"
	"strconv"
)

// CreateContainerVersion snapshots a container's current draft into a
// new, named, immutable version and returns its id. name must be 1-50
// characters (a Matomo-side constraint).
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

(`CreateContainerVersion`'s `out.Value` decode shape is the working
assumption from §3; if live Matomo returns a bare int instead, this
changes to decoding directly into an `int` — verify via the acceptance
test's actual JSON response at implementation time.)

## 5. Provider wiring

`internal/provider/provider.go`:

```go
var _ provider.ProviderWithActions = &MatomoProvider{}

func (p *MatomoProvider) Actions(_ context.Context) []func() action.Action {
	return []func() action.Action{
		NewCreateContainerVersionAction,
		NewPublishContainerVersionAction,
		NewEnablePreviewModeAction,
		NewDisablePreviewModeAction,
	}
}
```

`Configure()` additionally sets `resp.ActionData = client` (parallel to
the existing `resp.ResourceData`/`resp.DataSourceData` assignments;
`provider.ConfigureResponse.ActionData any` already exists in the SDK for
exactly this purpose).

## 6. The four actions

Each lives in its own file, `internal/provider/action_tagmanager_*.go`,
implementing `action.Action` + `action.ActionWithConfigure` (schema →
`Configure` reads `req.ProviderData.(*matomo.Client)` → `Invoke` parses
`container_id` via the existing `parseContainerID` helper, calls the
client method(s), reports progress via `resp.SendProgress`, sets
`resp.Diagnostics` on error).

### 6.1 `matomo_tagmanager_create_container_version`

```hcl
action "matomo_tagmanager_create_container_version" "release" {
  config {
    container_id = matomo_tagmanager_container.main.id
    name         = "release-${formatdate("YYYY-MM-DD", timestamp())}"
    description  = "Weekly release"
  }
}
```

Schema: `container_id` (required string), `name` (required string, 1-50
chars — document the Matomo-side limit in the field description, no
provider-side length validator since Matomo's own error is authoritative
and the exact boundary behavior hasn't been independently verified),
`description` (optional string, defaults to `""`).

`Invoke`: parse `container_id` → `(siteID, idContainer)`, call
`client.CreateContainerVersion(ctx, siteID, idContainer, name, description)`,
`SendProgress` before the call ("Creating container version...") and
after success ("Created container version <id>"). The created version's
id is only ever visible via that progress message — it cannot be
exposed as a consumable output (§2).

### 6.2 `matomo_tagmanager_publish_container_version`

```hcl
action "matomo_tagmanager_publish_container_version" "go_live" {
  config {
    container_id = matomo_tagmanager_container.main.id
    environment  = "live"
  }
}
```

Schema: `container_id` (required string), `environment` (required
string — free text, no static `stringvalidator.OneOf(...)`, same
philosophy as the generic tag/trigger/variable resources' `type` field;
the field description points at `matomo_tagmanager_environments` for
discovering valid values on a given instance).

`Invoke`, in order:
1. `SendProgress("Snapshotting current draft...")`.
2. `client.CreateContainerVersion(ctx, siteID, idContainer, "terraform-release-"+time.Now().UTC().Format(time.RFC3339), "")` —
   auto-generated name, no user-facing `name`/`description` fields on
   this action (deliberately: this action's whole point is "publish
   whatever the draft currently holds" with zero extra input, per the
   approved design decision).
3. `SendProgress("Publishing version <id> to environment <environment>...")`.
4. `client.PublishContainerVersion(ctx, siteID, idContainer, newVersionID, environment)`.
5. `SendProgress("Published.")` on success.

If step 2 fails, step 4 never runs and the diagnostic reports the
snapshot failure. If step 4 fails, the diagnostic reports the publish
failure — the snapshot from step 2 still exists in Matomo (harmless;
matches Matomo's own semantics of versions being cheap, retained
snapshots).

### 6.3 `matomo_tagmanager_enable_preview_mode`

```hcl
action "matomo_tagmanager_enable_preview_mode" "preview" {
  config {
    container_id = matomo_tagmanager_container.main.id
  }
}
```

Schema: `container_id` (required string) only — no version field, per
the approved design decision (always targets the draft, matching every
other resource in this provider).

`Invoke`: parse `container_id`, `SendProgress("Enabling preview mode...")`,
call `client.EnablePreviewMode(ctx, siteID, idContainer)`,
`SendProgress("Preview mode enabled.")` on success.

### 6.4 `matomo_tagmanager_disable_preview_mode`

Identical shape to 6.3, calling `client.DisablePreviewMode` instead.

## 7. Testing

- **Unit tests** (`internal/matomo/tagmanager_container_versions_test.go`):
  one `httptest`-fixture test per new client method, following the
  existing `internal/matomo` package convention (e.g.
  `tagmanager_templates_test.go`, `tagmanager_discovery_test.go`).
- **Acceptance tests** (`TF_ACC=1`,
  `internal/provider/action_tagmanager_*_acc_test.go`): one per action
  against live Matomo. Since `resource.Test`'s `TestStep` model is
  designed around resources (plan/apply/refresh), actions are exercised
  via a `TestStep.Config` containing both a `matomo_tagmanager_container`
  resource and the action block, asserting the action's effect via a
  follow-up read (e.g. `create_container_version`'s test creates a
  container, invokes the action, then confirms a new version exists via
  a direct client call in a `TestCheckFunc`, since there's no
  `matomo_tagmanager_container_versions` data source to assert through
  declaratively — adding one is out of scope for this phase). This
  mirrors how the project already handles action-shaped verification
  gaps: check real side effects out-of-band via the client, not just
  "did apply succeed."
- No `internal/provider`-layer unit tests for the actions themselves,
  matching the established convention for resources/data sources in
  this codebase (no mock-client pattern exists anywhere in this
  provider).

## 8. Documentation

No `examples/`/`tfplugindocs` work in this phase — matches Phase 6's
scope decision; this repo has no docs-generation CI step yet (Phase 8).

## 9. Explicitly out of scope

- Output chaining between actions (§2 — not possible in this framework
  version).
- A `matomo_tagmanager_container_versions` data source (would let
  acceptance tests assert declaratively instead of via a client-call
  `TestCheckFunc`, but no other stated use case yet).
- `container_version_id` override fields on `enable_preview_mode`/
  `disable_preview_mode` (per the approved design decision).
- Any static `stringvalidator.OneOf(...)` validation of `environment`
  against `matomo_tagmanager_environments`' real values.
