# terraform-provider-matomo — real Matomo acceptance testing design

Status: approved design, pre-implementation.

## 1. Goal and scope

The foundation plan (client + 6 resources + 1 data source for Sites, Custom
Dimensions, and Tag Manager) is merged, but several of its assumptions were
never checked against a real Matomo instance:

- Five hardcoded drift-detection error strings ("Website id Not found",
  "Container does not exist", "Tag does not exist", "Trigger does not
  exist", "Variable does not exist") used to detect out-of-band deletion.
- The JSON-encoded wire format for Tag Manager's `parameters`,
  `fireTriggerIds`, `blockTriggerIds`, and `conditions` query values.
- Whether Matomo's custom dimension `id` and `index` actually diverge the
  way `internal/matomo/customdimensions.go`'s source-verified comments
  claim (verified against Matomo's PHP source, but never against a running
  instance).

This slice builds the infrastructure and test suite to verify all of the
above: a Docker Compose Matomo + Tag Manager fixture, a non-interactive
bootstrap script, a separate CI workflow that provisions the fixture and
runs a real acceptance test suite, and full CRUD + import + "disappears"
acceptance tests for all six existing resources and the one data source.

Out of scope: anything not already built in the foundation (typed
per-type resources, codegen, actions, additional Matomo modules). This
slice only adds *tests and test infrastructure* for what already exists —
no new provider functionality.

## 2. Environment constraint

This development environment cannot run Docker (no daemon available), so
the fixture cannot be exercised directly in an interactive session. The
fixture and tests are written and statically validated here; the actual
green/red signal comes from the new CI workflow's run in GitHub Actions,
where Docker is available. Iteration on acceptance-test failures happens by
reading Actions logs, not by running the suite locally in this environment.

## 3. Docker Compose fixture

`docker-compose.yml` at the repo root:

```yaml
services:
  db:
    image: mariadb:11
    environment:
      MARIADB_DATABASE: matomo
      MARIADB_USER: matomo
      MARIADB_PASSWORD: matomo
      MARIADB_ROOT_PASSWORD: root
    healthcheck:
      test: ["CMD", "healthcheck.sh", "--connect", "--innodb_initialized"]
      interval: 5s
      retries: 10

  matomo:
    image: matomo:latest
    depends_on:
      db:
        condition: service_healthy
    environment:
      MATOMO_DATABASE_HOST: db
      MATOMO_DATABASE_USERNAME: matomo
      MATOMO_DATABASE_PASSWORD: matomo
      MATOMO_DATABASE_DBNAME: matomo
    ports:
      - "8080:80"
```

`scripts/bootstrap-matomo.sh`: waits for the Matomo container to accept
connections, then runs Matomo's `console` CLI (already present in the
official image) non-interactively:

1. `console core:install` with DB connection flags and a fixed superuser
   (login/password/email from script-local constants — this is a
   throwaway, network-isolated CI fixture, not a credential to protect) to
   fully provision Matomo without the browser install wizard.
2. `console plugin:activate TagManager` to enable Tag Manager.
3. Generate a superuser API token via `console` (Matomo's console includes
   a token-generation command; if the installed version lacks one, fall
   back to a single authenticated `UsersManager.createAppSpecificTokenAuth`
   or equivalent API call using the superuser's session cookie from step
   1). Print `MATOMO_BASE_URL=http://localhost:8080` and
   `MATOMO_API_TOKEN=<token>` in a format the CI workflow can capture into
   `$GITHUB_ENV`.

This script is annotated with `# implementer: verify exact console
subcommand names/flags against the Matomo image version pinned above —
Matomo's CLI surface has changed across versions and this is the first
time it's been exercised in this project` since the plan author (this
brainstorming session) has not run it against a live Matomo instance
either — only the target commands are well-documented, not their exact
current flags.

## 4. CI workflow

New `.github/workflows/acceptance.yml`, deliberately separate from
`ci.yml`:

```yaml
name: Acceptance

on:
  workflow_dispatch:
  schedule:
    - cron: "0 6 * * *"

jobs:
  acceptance:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - uses: hashicorp/setup-terraform@v3
        with:
          terraform_wrapper: false
      - run: docker compose up -d
      - run: ./scripts/bootstrap-matomo.sh >> "$GITHUB_ENV"
      - run: TF_ACC=1 go test ./... -run TestAcc -v -count=1 -timeout 20m
      - if: always()
        run: docker compose down -v
```

Never triggers on `push`/`pull_request` — matches the spec's existing
"nightly/on-demand, not blocking PRs" phasing for acceptance tests. `ci.yml`
is untouched.

## 5. Test structure

**Rename** (mechanical, own commit, no logic changes): every existing
`TestAccXxx` in `internal/provider/*_test.go` (httptest-fixture-backed,
using `resource.UnitTest`) becomes `TestUnitXxx`. These already run in
`ci.yml` on every PR and continue to; only the name changes, to stop
colliding with the Terraform ecosystem's `TestAcc*` convention for real
acceptance tests.

**New**, one `<resource>_acc_test.go` per resource/data source, using
`resource.Test` (not `UnitTest`) against the real provider config
(`MATOMO_BASE_URL`/`MATOMO_API_TOKEN` from the environment) with a
`testAccPreCheck(t)` helper that `t.Skip()`s if `TF_ACC`,
`MATOMO_BASE_URL`, or `MATOMO_API_TOKEN` aren't set — so these files are
inert (compiled, never executed) in the fast `ci.yml` job and only run in
`acceptance.yml`.

Per resource, three test functions:

- **`TestAccXxx_basic`** — create, read-back via refresh, update, confirm
  via `TestCheckResourceAttr` against real Matomo responses.
- **`TestAccXxx_import`** — `resource.TestStep{ImportState: true,
  ImportStateVerify: true}`, confirming the composite import ID round-trips
  against a real Matomo-backed resource.
- **`TestAccXxx_disappears`** — create via Terraform, then delete the
  underlying Matomo object directly via a `matomo.Client` constructed
  inline in the test (bypassing Terraform entirely), then run
  `terraform plan` and assert it detects the resource is gone (non-empty
  plan proposing recreation, or for import-style checks,
  `resource.TestCheckNoResourceAttr`-style assertions on the refreshed
  state). This is the test that actually exercises each resource's
  hardcoded not-found error string against Matomo's real response, per §1.

`matomo_custom_dimension` additionally gets `TestAccCustomDimensionResource_idIndexDivergence`:
creates a visit-scope and then an action-scope dimension on the same real
site, confirms Matomo's real `id`/`index` values, and asserts they diverge
as `internal/matomo/customdimensions.go`'s comments (added during the
foundation plan's Task 13 fix, verified only against Matomo's PHP source)
claim.

Tag/trigger/variable acceptance tests exercise multi-parameter blocks
(mirroring the foundation plan's flaky-ordering regression tests, but
against real Matomo's actual JSON responses this time) to confirm the
`parameters`/`fireTriggerIds`/`conditions` wire format assumptions from
`internal/matomo/tagmanager_{tags,triggers,variables}.go`.

## 6. Outcome handling

If a real-Matomo test reveals one of the "unverified" assumptions was
wrong (a different not-found message, a different wire format), the
acceptance test failure is the signal — the fix is a normal implementation
task against the discrepancy, not part of this design. This spec builds
the harness to find such problems; it does not pre-guess what they'll be.

Once the full suite is green against real Matomo, the five error-string
code comments added during the foundation plan (marking them "unverified")
get updated to note they're now confirmed against a live instance — or
corrected if real Matomo's strings differ.

## 7. Phasing

1. `docker-compose.yml` + `scripts/bootstrap-matomo.sh` +
   `.github/workflows/acceptance.yml`, with a single trivial smoke test
   (e.g. `data.matomo_site` lookup against a manually-created site) to
   prove the fixture boots and the workflow reaches a real Matomo instance
   end-to-end.
2. Rename existing tests (`TestAcc*` → `TestUnit*`).
3. `matomo_site` + `data.matomo_site` acceptance tests (basic/import/disappears).
4. `matomo_custom_dimension` acceptance tests, including the id/index
   divergence test.
5. `matomo_tagmanager_container` acceptance tests.
6. `matomo_tagmanager_tag` acceptance tests, including multi-parameter
   wire-format verification.
7. `matomo_tagmanager_trigger` acceptance tests, including conditions
   wire-format verification.
8. `matomo_tagmanager_variable` acceptance tests.
9. Final verification: full suite green in `acceptance.yml`; update or fix
   the five error-string comments based on real results.

## 8. Explicitly out of scope for this slice

- Any new provider resource, data source, or action.
- `tools/gen` codegen against the fixture (a separate future plan; this
  fixture is built for acceptance tests, and reused by that plan later,
  not built by it).
- Running the acceptance suite in this development session (blocked by the
  Docker daemon constraint in §2) — verification happens via CI logs.
- Matomo versions other than whatever `matomo:latest` resolves to at
  fixture-build time (matches the foundation spec's "latest stable only"
  constraint).
