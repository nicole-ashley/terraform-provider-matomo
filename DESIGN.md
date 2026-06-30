# terraform-provider-matomo — Design

Status: brainstorm / pre-implementation. This document captures the architecture
for a Terraform provider that orchestrates Matomo via its HTTP API. Phase one
scope is the **Tag Manager** module in full; the provider is structured so
other Matomo modules (Sites, Users, Goals, …) can be added later without
rework.

## 1. Why Tag Manager first

Matomo Tag Manager (containers → versions → tags/triggers/variables → releases)
is a self-contained CRUD surface with ~47 API methods, all under the
`TagManager.*` module. It's a good first slice: it exercises nested resources,
generic/typed parameter handling, and exactly the kind of imperative
"publish" operation the new Terraform **action** API was built for.

## 2. Tech stack

- Go, `terraform-plugin-framework` (current major; this is the only framework
  with action support, which is required here).
- `terraform-plugin-testing` for acceptance-style tests (`TF_ACC=1`).
- `tfplugindocs` to generate `docs/` from schema + `examples/` + `templates/`.
- `golangci-lint`, `goreleaser`, GitHub Actions CI.

## 3. Matomo API client (`internal/matomo`)

Matomo's API is a single HTTP endpoint (`/index.php?module=API&method=X`,
GET/POST, `format=JSON`, `token_auth=...`) that returns either the payload or
an error envelope `{"result":"error","message":"..."}`. The client is a thin
wrapper, not a generic RPC proxy:

- `matomo.Client` — holds base URL, token, `*http.Client`, does request
  building, auth, error-envelope decoding into a typed `*matomo.APIError`.
- `internal/matomo/tagmanager.go` — one Go method per `TagManager.*` API
  method (containers, versions, tags, triggers, variables, releases, preview
  mode, available-types lookups), each with request/response structs.
- All client methods take `context.Context` and return typed structs/errors —
  no `map[string]interface{}` leaking into provider code.
- Unit tested with `httptest.Server` fixtures covering success and Matomo
  error-envelope responses for every method (table-driven, no live Matomo
  needed).

## 4. Provider configuration

```hcl
provider "matomo" {
  base_url  = "https://analytics.example.com" # or MATOMO_BASE_URL
  api_token = var.matomo_api_token             # or MATOMO_API_TOKEN, sensitive
}
```

Optional: `insecure_skip_verify` for self-hosted instances with internal CA.
No default `site_id` — every resource is explicit about which site it
belongs to, since Matomo Tag Manager is inherently multi-site.

## 5. The draft/version problem (key design decision)

Tag Manager's mental model is GTM-like: a container has one mutable **draft**
version that tags/triggers/variables are written against, plus immutable
numbered **versions** created as snapshots of the draft, which are then
**published** to an **environment** (live/staging/dev) to produce a release.

Modeling versions as a CRUD resource doesn't fit Terraform well — "create a
version" is a point-in-time snapshot action, not a stable piece of desired
state, and "publish" is explicitly called out as an action-API use case in
the brief. So:

- `matomo_tagmanager_tag` / `_trigger` / `_variable` resources always target
  the container's **draft** version. The provider resolves the draft version
  ID internally (`getContainerVersions` → the one with `isDraft`) — users
  never pass a version ID for everyday authoring.
- Versions are **not** a CRUD resource. Snapshotting and publishing are
  **actions** (see §7), invoked explicitly from HCL/CLI when you want to cut
  a release, e.g. via `terraform apply -- -action=...` / action blocks in
  config, run after the draft resources converge.

This keeps "edit tags" (idempotent, plannable, Terraform-native) cleanly
separated from "cut a release" (imperative, ordered, side-effecting —
action-native).

## 6. Resources

| Resource | Matomo API methods | Notes |
|---|---|---|
| `matomo_tagmanager_container` | add/update/delete/getContainer, getContainers | `context` (web/android/ios) is ForceNew |
| `matomo_tagmanager_tag` | add/update/delete/getContainerTag, pause/resumeContainerTag | targets draft version; `status` (active/paused) drives pause/resume calls |
| `matomo_tagmanager_trigger` | add/update/delete/getContainerTrigger | targets draft version |
| `matomo_tagmanager_variable` | add/update/delete/getContainerVariable | targets draft version |

Tag/trigger/variable **type-specific parameters** are wildly heterogeneous
(30+ tag types, 15+ trigger types, custom JS variables, etc.) and Matomo
itself models them as `name => value` parameter arrays plus a `conditions`
list for triggers. Rather than generating one typed resource per type (huge
surface, fragile against Matomo plugin-contributed types), the schema uses:

```hcl
resource "matomo_tagmanager_tag" "ga4" {
  site_id      = matomo_tagmanager_container.main.site_id
  container_id = matomo_tagmanager_container.main.id
  type         = "Ga4Tag"
  name         = "GA4 page view"
  parameter {
    name  = "measurementId"
    value = "G-XXXX"
  }
  fire_trigger_ids = [matomo_tagmanager_trigger.pageview.id]
  status           = "active"
}
```

`type` is validated at plan time against `getAvailableTagTypesInContext` data
(see §8), so typos are caught without per-type schemas. This is flagged in
§10 as a place we may later add a handful of strongly-typed convenience
resources (e.g. `matomo_tagmanager_tag_custom_html`) layered on top of the
generic one, once real usage shows which types are worth it.

Import IDs throughout: `site_id/container_id[/entity_id]`.

## 7. Actions (experimental action API)

```hcl
action "matomo_tagmanager_create_container_version" "release" {
  config {
    site_id      = matomo_tagmanager_container.main.site_id
    container_id = matomo_tagmanager_container.main.id
    name         = "release-${formatdate("YYYY-MM-DD", timestamp())}"
  }
}

action "matomo_tagmanager_publish_container_version" "go_live" {
  config {
    site_id              = matomo_tagmanager_container.main.site_id
    container_id         = matomo_tagmanager_container.main.id
    container_version_id = action.matomo_tagmanager_create_container_version.release.output.container_version_id
    environment           = "live"
  }
}
```

- `matomo_tagmanager_create_container_version` — snapshots the current draft
  into an immutable version (`createContainerVersion`), emits its ID.
- `matomo_tagmanager_publish_container_version` — publishes a version to an
  environment (`publishContainerVersion`).
- `matomo_tagmanager_enable_preview_mode` / `..._disable_preview_mode` —
  thin wrappers, useful for CI smoke-testing a draft before release.

These are unlinked actions (no resource state to mutate), implementing
`action.ActionWithConfigure` to receive the shared `matomo.Client`, with
`ProgressEvent` calls around the HTTP round trip for visibility on slow
Matomo instances.

## 8. Data sources

Read-only enumerations needed for validation and discovery, all cheap
wrappers over the corresponding `getAvailable*` methods:

`matomo_tagmanager_contexts`, `..._environments`, `..._tag_types`,
`..._trigger_types`, `..._variable_types`, plus `matomo_tagmanager_container`
and `..._container_version` for read-only lookups (e.g. resolving an
existing container by name before import).

## 9. Testing & hygiene

- **Unit tests**: every `internal/matomo` client method (success + error
  envelope) via `httptest`; every resource's CRUD + error-path behavior via
  `terraform-plugin-testing`'s `resource.UnitTest` against the same
  `httptest` fixtures (no live Matomo required, runs in CI on every PR).
- **Acceptance tests**: `TF_ACC=1` suite against a real Matomo + Tag Manager
  plugin (docker-compose fixture), run nightly/on-demand, not blocking PRs.
- `golangci-lint` (errcheck, govet, staticcheck, gofmt) in CI.
- `tfplugindocs generate` checked into `docs/`, verified in CI (`tfplugindocs
  validate` / git-diff check).
- `examples/` per resource/data source/action (required by tfplugindocs and
  doubles as integration fixtures).
- `goreleaser` for signed multi-platform releases + Terraform Registry
  manifest.
- Conventional commits + `CHANGELOG.md`.

## 10. Repo layout

```
main.go
internal/
  matomo/            # API client, one file per Matomo module (tagmanager.go first)
  provider/          # provider.go, resource_*.go, data_source_*.go, action_*.go
examples/
docs/                # generated
templates/           # tfplugindocs templates
.github/workflows/   # ci.yml, release.yml
.golangci.yml
.goreleaser.yml
GNUmakefile
```

## 11. Phasing

1. Client package (`internal/matomo`) + unit tests, provider scaffold + CI.
2. `matomo_tagmanager_container` resource end-to-end (proves the pattern).
3. `tag` / `trigger` / `variable` resources (generic parameter model).
4. Data sources (types/contexts/environments).
5. Actions (create version, publish, preview mode).
6. Docs, examples, acceptance test suite, release pipeline.
7. (Future) typed convenience resources for common tag/trigger types;
   expand beyond Tag Manager to other Matomo modules (Sites, Users, Goals).

## 12. Open questions for the user

- Any specific Matomo version / Tag Manager plugin version floor to target?
- Preferred registry namespace (`registry.terraform.io/<org>/matomo`)?
- Should `matomo_tagmanager_tag.status` drive pause/resume automatically, or
  should pausing be a separate action (closer to Matomo's own UX, where
  pause/resume are one-off ops, not part of normal config)?
