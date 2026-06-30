# terraform-provider-matomo — design spec

Status: approved design, pre-implementation.

## 1. Goal and scope

A Terraform provider, published to the public Terraform Registry as
`registry.terraform.io/nicole-ashley/matomo`, that manages Matomo via its
HTTP API. v1 scope:

- **Tag Manager**, in full: containers, draft/version/release lifecycle,
  tags, triggers, variables, preview mode.
- **Sites** (`SitesManager` API) and **Custom Dimensions**
  (`CustomDimensions` API) — pulled in because Tag Manager resources need to
  reference sites and dimensions by name instead of bare numeric IDs (see
  §4).

Out of scope for v1, explicitly: any other Matomo module (Users, Goals,
Segments, ...). The provider is structured so those can be added later
(`internal/matomo/<module>.go` per module, following the same client
pattern), but this spec does not design them.

Target Matomo version: latest stable Matomo core + Tag Manager plugin only.
Older versions are unsupported; this is a deliberate simplification, not an
oversight — revisit if real usage demands otherwise.

## 2. Tech stack

- Go, `terraform-plugin-framework` (current major — the only framework with
  action support, required for §7).
- `terraform-plugin-testing` for unit tests (`resource.UnitTest`, no live
  Matomo) and acceptance tests (`resource.Test`, gated on `TF_ACC=1`).
- `tfplugindocs` for generated docs.
- `golangci-lint`, `goreleaser`, GitHub Actions CI.

## 3. Matomo API client (`internal/matomo`)

Matomo's API is a single HTTP endpoint
(`/index.php?module=API&method=X&format=JSON&token_auth=...`, GET/POST)
returning either the payload or an error envelope
`{"result":"error","message":"..."}`. The client is a thin, explicitly typed
wrapper, not a generic RPC proxy:

- `matomo.Client` — base URL, token, `*http.Client`; builds requests,
  attaches auth, decodes the error envelope into a typed `*matomo.APIError`.
- One file per Matomo module: `internal/matomo/tagmanager.go` (the 47
  `TagManager.*` methods — containers, versions, tags, triggers, variables,
  releases, preview mode, type metadata lookups), `internal/matomo/sites.go`
  (`addSite`, `updateSite`, `deleteSite`, `getSiteFromId`, `getAllSites`),
  `internal/matomo/customdimensions.go` (`configureNewCustomDimension`,
  `configureExistingCustomDimension`, `getConfiguredCustomDimensions`,
  `getAvailableScopes`).
- Every client method takes `context.Context`, returns typed
  request/response structs and errors — no `map[string]interface{}` leaking
  into provider code.
- Unit tested with `httptest.Server` fixtures covering success and
  Matomo-error-envelope responses for every method; no live Matomo required.

## 4. Resource tree and composite IDs

Matomo's Tag Manager and Custom Dimensions APIs both require an explicit
`idSite` on every call, and Tag Manager additionally requires `idContainer`
on every tag/trigger/variable call. Declaring those IDs redundantly on every
resource would be both noisy and a drift hazard (nothing stops a typo'd
`site_id` on a tag from silently targeting the wrong site). Instead, parent
identity is carried through **composite resource IDs**, and child resources
reference only their immediate parent:

```
matomo_site                      id = "{site_id}"                       (root)
├── matomo_custom_dimension      id = "{site_id}/{index}"
└── matomo_tagmanager_container  id = "{site_id}/{container_id}"
    ├── matomo_tagmanager_tag        id = "{site_id}/{container_id}/{tag_id}"
    ├── matomo_tagmanager_trigger    id = "{site_id}/{container_id}/{trigger_id}"
    └── matomo_tagmanager_variable   id = "{site_id}/{container_id}/{variable_id}"
```

Rules:

- `matomo_site` is the only resource with a plain numeric `id` (it has no
  parent in this tree). `site_id` is required only here.
- `matomo_custom_dimension` and `matomo_tagmanager_container` take a single
  `site_id` attribute set to `matomo_site.x.id`.
- `matomo_tagmanager_{tag,trigger,variable}` take a single `container_id`
  attribute set to `matomo_tagmanager_container.x.id` (the composite
  `"{site_id}/{container_id}"` string) — no `site_id` field exists on these
  resources at all.
- The provider parses composite IDs internally wherever it needs the
  individual path segments for an API call, and reconstitutes the composite
  form when writing `id` back to state.
- Cross-references *within* a container — `fire_trigger_ids`,
  `block_trigger_ids`, and (for the generic fallback resources) condition
  blocks that reference a variable — consume another resource's `.id`
  directly (e.g. `fire_trigger_ids = [matomo_tagmanager_trigger.pageview.id]`).
  The provider strips the composite ID down to the bare numeric ID Matomo's
  API expects before sending the request, and re-expands it when reading
  state back, validating that the referenced entity belongs to the same
  container (plan-time error otherwise).
- Every resource's **import ID matches its composite `.id` exactly**, so
  `terraform import` arguments are copy-pasteable from `terraform state
  show`/`-raw` output.

## 5. Provider configuration

```hcl
provider "matomo" {
  base_url  = "https://analytics.example.com" # or MATOMO_BASE_URL
  api_token = var.matomo_api_token             # or MATOMO_API_TOKEN, sensitive
}
```

Optional `insecure_skip_verify` for self-hosted instances with internal CAs.
No default `site_id` — every resource's site association is explicit via the
tree in §4. `addSite`/`deleteSite` require a superuser token; this is a
documented operational requirement, not something the provider enforces or
works around.

## 6. Resources

### 6.1 `matomo_site`

Full CRUD via `SitesManager.{addSite,updateSite,deleteSite,getSiteFromId}`.
Schema mirrors `addSite`'s parameters (`name`, `urls`, `timezone`,
`currency`, `ecommerce`, `excluded_ips`, etc.) Plain numeric `id`.

### 6.2 `matomo_custom_dimension`

- `site_id` (required) = `matomo_site.x.id`.
- `index` and `scope` (`visit` | `action`) — **required, `RequiresReplace`**.
  The user picks the slot explicitly; Matomo does not support choosing a
  slot index on creation, so:
  - **Create**: call `getConfiguredCustomDimensions(site_id)`. If a
    dimension already exists at the declared `index`+`scope`, adopt it via
    `configureExistingCustomDimension` (claims and updates it in place). If
    none exists, call `configureNewCustomDimension` and compare the slot
    Matomo assigned against the declared `index`; mismatch is a hard error
    (surfaces when slots were consumed outside Terraform, or out of order),
    not a silent misconfiguration.
  - **Update**: `configureExistingCustomDimension` for `name`, `active`,
    `extractions`, `case_sensitive` (everything except `index`/`scope`,
    which force replacement).
  - **Delete**: `configureExistingCustomDimension` with `active = false`.
    Matomo has no delete API for custom dimensions — the slot stays
    permanently reserved at that index regardless of what Terraform does.
    This is documented as a Matomo platform constraint on the resource's
    registry docs page, not glossed over.
- No separate computed ID field: `index` *is* the dimension's real Matomo ID
  by construction (Create verifies this, see above), so other resources
  reference it directly — e.g.
  `index = matomo_custom_dimension.product_category.index` in a Tag Manager
  variable's config — instead of a hardcoded literal.

### 6.3 `matomo_tagmanager_container`

CRUD via `add/update/delete/getContainer`. `site_id` and `context`
(`web`/`android`/`ios`) are `RequiresReplace`.

### 6.4 Tags, triggers, variables — draft-version model

Tag Manager's mental model: a container has one mutable **draft** version
that tags/triggers/variables are authored against, plus immutable numbered
**versions** snapshotted from the draft and **published** to an
**environment** (live/staging/dev). `matomo_tagmanager_tag` / `_trigger` /
`_variable` resources **always target the draft version** — the provider
resolves the draft version ID internally (`getContainerVersions` → the entry
with `isDraft`); users never see or set a version ID for everyday authoring.

Versions are **not** a CRUD resource. Snapshotting and publishing are
**actions** (§7) — "edit tags" (idempotent, plannable) is intentionally kept
separate from "cut a release" (imperative, ordered, side-effecting).

`status` (`active` | `paused`) is a plain attribute, driving
`pause/resumeContainerTag`. Verified against Tag Manager's source: pause and
resume both go through `assertUserCanEditContainerVersion` — the same guard
as every other tag edit — and only update the *preview* release, never a
published one. So `status` behaves exactly like every other field on these
resources: changing it edits the draft, visible in preview mode, and only
reaches a live container after a `create_container_version` +
`publish_container_version` action run. This is called out explicitly in
each resource's docs so `terraform apply` pausing a tag isn't mistaken for
an immediate live effect.

**Two resource forms per entity kind** (tag, trigger, variable):

- **Typed**, one per built-in Matomo type (`matomo_tagmanager_tag_ga4`,
  `matomo_tagmanager_trigger_page_view`, `matomo_tagmanager_variable_matomo_configuration`,
  etc.) — schema fields match that type's real parameters, giving static
  validation, accurate docs, and (where relevant, e.g. the Matomo
  Configuration variable's `custom_dimensions { index, value }` blocks)
  natural points to plug in a reference like
  `index = matomo_custom_dimension.x.index` instead of a literal.
- **Generic fallback** — `matomo_tagmanager_tag` / `_trigger` / `_variable`,
  taking `type` (string) plus repeated `parameter { name, value }` blocks.
  Covers third-party-plugin-contributed types that don't exist in the
  codegenerated typed set. `type` is validated at plan time against
  `getAvailableTagTypesInContext` (etc.) data, so typos are caught without a
  typed schema for that specific type.

**Generating the typed resources**: hand-transcribing ~50 schemas from
Matomo's PHP template source is both a large one-time effort and stale the
moment Matomo ships a new type. Instead, `tools/gen` is a Go program that:

1. Authenticates against a reference Matomo + Tag Manager instance (a
   docker-compose fixture, same one used for acceptance tests).
2. Calls `getAvailableTagTypesInContext` / `getAvailableTriggerTypesInContext`
   / `getAvailableVariableTypesInContext`, which expose each template's
   declared parameter list (name, data type, default, validation) — this is
   the same metadata Matomo's own UI uses to render its config forms.
3. Emits one `resource_tagmanager_{tag,trigger,variable}_<type>.go` per type
   from a Go template, mapping Matomo's parameter datatypes to Terraform
   schema types.
4. Is re-run (manually, not on every CI build) to pick up newly-released
   Matomo types; output is committed, not generated at build time.

Generated schemas are a starting point, not gospel — `getAvailable*` exposes
validation as Matomo's own `Setting` metadata, which doesn't always capture
everything a hand-written schema would (cross-field conflicts, semantic
references like the custom-dimension case). Tightening individual generated
files by hand is expected and fine; the generator's job is to make the
common case (a plain parameter list) free, not to be perfectly hands-off
forever.

### 6.5 Import

Every resource's import ID is its composite `.id` from §4
(`site_id/container_id/entity_id`, etc.).

## 7. Actions (experimental action API)

The action API itself is experimental and its HCL invocation/chaining syntax
may still change before stabilizing; the block shapes below illustrate the
intended action schemas (config in, output out), not a syntax guarantee.

```hcl
action "matomo_tagmanager_create_container_version" "release" {
  config {
    container_id = matomo_tagmanager_container.main.id
    name         = "release-${formatdate("YYYY-MM-DD", timestamp())}"
  }
}

action "matomo_tagmanager_publish_container_version" "go_live" {
  config {
    container_id          = matomo_tagmanager_container.main.id
    container_version_id  = action.matomo_tagmanager_create_container_version.release.output.container_version_id
    environment            = "live"
  }
}
```

- `matomo_tagmanager_create_container_version` — snapshots the draft into an
  immutable version (`createContainerVersion`), outputs its ID.
- `matomo_tagmanager_publish_container_version` — publishes a version to an
  environment (`publishContainerVersion`).
- `matomo_tagmanager_enable_preview_mode` / `..._disable_preview_mode` —
  thin wrappers (`enable/disablePreviewMode`), useful for smoke-testing a
  draft before cutting a release.

All four are unlinked actions implementing `action.ActionWithConfigure` to
receive the shared `matomo.Client`, with `ProgressEvent` calls around the
HTTP round trip.

## 8. Data sources

- `matomo_site` — lookup by ID or name, for sites that exist in Matomo but
  aren't (or shouldn't be) managed by this provider.
- `matomo_tagmanager_contexts`, `..._environments`, `..._tag_types`,
  `..._trigger_types`, `..._variable_types` — thin wrappers over the
  corresponding `getAvailable*` methods, used for discovery and for the
  generic fallback resources' `type` validation.

## 9. Testing

- **Unit tests**: every `internal/matomo` client method (success + error
  envelope) via `httptest`; every resource/data source/action's CRUD and
  error-path behavior via `terraform-plugin-testing`'s `resource.UnitTest`
  against the same fixtures — including the custom-dimension
  adopt-vs-create-and-verify-slot branch and the status→pause/resume path.
  Runs in CI on every PR, no live Matomo required.
- **Acceptance tests**: `TF_ACC=1` suite against a real Matomo + Tag Manager
  plugin (docker-compose fixture, the same one `tools/gen` uses), covering
  full create/update/import/destroy cycles per resource. Nightly/on-demand,
  not blocking PRs.
- **Generator**: `tools/gen` output is itself checked by running it against
  the fixture instance in CI and diffing against committed output — a
  drift-check, not a regeneration-on-every-build step.

## 10. Hygiene

- `golangci-lint` (errcheck, govet, staticcheck, gofmt) in CI.
- `tfplugindocs generate`, checked into `docs/`, verified in CI.
- `examples/` per resource/data source/action (required by tfplugindocs,
  doubles as integration fixtures).
- `goreleaser` for signed multi-platform releases + Terraform Registry
  manifest, namespace `nicole-ashley/matomo`.
- Conventional commits + `CHANGELOG.md`.

## 11. Repo layout

```
main.go
internal/
  matomo/              # API client: tagmanager.go, sites.go, customdimensions.go
  provider/            # provider.go, resource_*.go, data_source_*.go, action_*.go
    generated/         # output of tools/gen, committed
tools/
  gen/                 # codegen program (§6.4)
examples/
docs/                  # tfplugindocs output
templates/             # tfplugindocs templates
.github/workflows/     # ci.yml (unit tests, lint, docs check), acceptance.yml (nightly), release.yml
.golangci.yml
.goreleaser.yml
GNUmakefile
docker-compose.yml     # Matomo + Tag Manager fixture, shared by tools/gen and acceptance tests
```

## 12. Phasing

1. Client package (`internal/matomo`: tagmanager.go, sites.go,
   customdimensions.go) + unit tests, provider scaffold, docker-compose
   fixture, CI.
2. `matomo_site` resource + data source, `matomo_custom_dimension` resource
   — proves the composite-ID tree pattern on the simplest cases.
3. `matomo_tagmanager_container` resource.
4. Generic `matomo_tagmanager_{tag,trigger,variable}` fallback resources +
   draft-version resolution logic + `status`/pause-resume.
5. `tools/gen` + generated typed tag/trigger/variable resources.
6. Data sources (`contexts`/`environments`/`tag_types`/`trigger_types`/`variable_types`).
7. Actions (create version, publish, preview mode).
8. Docs, examples, acceptance test suite, release pipeline.

## 13. Explicitly out of scope for v1

- Any Matomo module beyond Sites, Custom Dimensions, and Tag Manager (Users,
  Goals, Segments, ...).
- Auto-regenerating `tools/gen` output as part of every CI build — it's a
  manual, committed-output step.
- Matomo versions older than current stable.
