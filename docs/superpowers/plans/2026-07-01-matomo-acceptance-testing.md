# Matomo Acceptance Testing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Docker Compose Matomo + Tag Manager fixture and a real-Matomo acceptance test suite (separate `acceptance.yml` CI workflow) that verifies the foundation plan's unverified assumptions: five hardcoded drift-detection error strings, the Tag Manager JSON wire format, and the custom-dimension `id`/`index` divergence.

**Architecture:** `docker-compose.yml` (Matomo + MariaDB) + a bootstrap script that non-interactively installs Matomo and activates Tag Manager via its `console` CLI, wired into a new GitHub Actions workflow that never blocks PRs. Existing httptest-backed tests are renamed off the `TestAcc*` prefix; new `*_acc_test.go` files per resource use `resource.Test` against the real provisioned Matomo instance, skip-guarded when no real Matomo is configured.

**Tech Stack:** Go, `terraform-plugin-testing` (`resource.Test`, not `UnitTest`), Docker Compose, GitHub Actions, Matomo's `console` CLI.

## Global Constraints

- Spec: `docs/superpowers/specs/2026-07-01-matomo-acceptance-testing-design.md` — every task below implements a section of it.
- **This development environment cannot run Docker** (no daemon available). No task in this plan can be verified end-to-end locally against a real Matomo instance. Every task's implementer must still run what CAN be verified locally (Go compilation, `gofmt`, `go vet`, and confirming a test correctly `t.Skip()`s when `TF_ACC`/`MATOMO_BASE_URL`/`MATOMO_API_TOKEN` are unset) — but the actual PASS against real Matomo is confirmed later, by the controller, via a GitHub Actions `workflow_dispatch` run of `acceptance.yml` and its logs. Do not claim a real-Matomo test "passes" without that evidence; report what you could and couldn't verify locally.
- `ci.yml` (existing, fast, httptest-backed) is untouched except for the test-rename task. `acceptance.yml` is new, separate, triggers only on `workflow_dispatch` and a nightly `schedule` cron — never on `push`/`pull_request`.
- New acceptance test files use `resource.Test`, never `resource.UnitTest` (that's reserved for the existing httptest-backed suite).
- Every new acceptance test file needs a `testAccPreCheck(t *testing.T)` helper (defined once, reused) that skips the test unless `TF_ACC`, `MATOMO_BASE_URL`, and `MATOMO_API_TOKEN` are all set.
- Existing composite ID / client / resource code (from the foundation plan) is not modified by this plan except where a real-Matomo test reveals a genuine discrepancy — and even then, only as an explicit, separately-reviewed fix, not folded silently into a test-writing task.

---

## File Structure

```
docker-compose.yml
scripts/
  bootstrap-matomo.sh
.github/workflows/
  acceptance.yml           # new
internal/provider/
  acc_test_helpers.go      # testAccPreCheck, shared real-provider factory
  resource_site_acc_test.go
  data_source_site_acc_test.go
  resource_custom_dimension_acc_test.go
  resource_tagmanager_container_acc_test.go
  resource_tagmanager_tag_acc_test.go
  resource_tagmanager_trigger_acc_test.go
  resource_tagmanager_variable_acc_test.go
  # existing *_test.go files: TestAccXxx -> TestUnitXxx (rename only)
internal/matomo/customdimensions.go   # comment update only, in the final task
internal/provider/resource_*.go       # comment update only, in the final task (error-string NOTE comments)
```

---

### Task 1: Docker Compose fixture, bootstrap script, acceptance CI workflow, smoke test

**Files:**
- Create: `docker-compose.yml`
- Create: `scripts/bootstrap-matomo.sh`
- Create: `.github/workflows/acceptance.yml`
- Create: `internal/provider/acc_test_helpers.go`
- Create: `internal/provider/data_source_site_acc_test.go` (smoke test only in this task; full suite is Task 3)

**Interfaces:**
- Consumes: `provider.New` (existing, from `internal/provider/provider.go`); `providerserver.NewProtocol6WithError` (already imported elsewhere in the package).
- Produces: `testAccPreCheck(t *testing.T)` and `testAccProtoV6ProviderFactories` (a `map[string]func() (tfprotov6.ProviderServer, error)` built from the real `provider.New` and real env-var-driven config) — every later acceptance test file in this plan uses both, unchanged.

- [ ] **Step 1: Write `docker-compose.yml`**

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
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost/matomo.php"]
      interval: 5s
      retries: 20
```

- [ ] **Step 2: Write `scripts/bootstrap-matomo.sh`**

```bash
#!/usr/bin/env bash
# Bootstraps a freshly-started Matomo container into a usable state for
# acceptance tests: installs Matomo non-interactively, activates Tag
# Manager, and prints MATOMO_BASE_URL/MATOMO_API_TOKEN in `KEY=value` form
# suitable for appending to $GITHUB_ENV.
#
# IMPLEMENTER NOTE: this script's exact `console` subcommands/flags have
# not been verified against a running Matomo instance (this development
# environment cannot run Docker). Verify against the pinned `matomo:latest`
# image's actual `console list` output before trusting this script in CI,
# and fix flag names/order here if they've changed.
set -euo pipefail

MATOMO_CONTAINER="${MATOMO_CONTAINER:-$(docker compose ps -q matomo)}"
SUPERUSER_LOGIN="acceptance-admin"
SUPERUSER_PASSWORD="acceptance-password-not-a-secret"
SUPERUSER_EMAIL="acceptance@example.com"

docker compose exec -T matomo php console core:install \
  --superuser-login="$SUPERUSER_LOGIN" \
  --superuser-password="$SUPERUSER_PASSWORD" \
  --superuser-email="$SUPERUSER_EMAIL" \
  --db-host=db \
  --db-username=matomo \
  --db-password=matomo \
  --db-name=matomo \
  --matomo-url="http://localhost:8080/" \
  --do-not-track=1 \
  --no-interaction

docker compose exec -T matomo php console plugin:activate TagManager

TOKEN=$(docker compose exec -T matomo php console user:generate-api-token \
  "$SUPERUSER_LOGIN" "$SUPERUSER_PASSWORD" | tail -n1 | tr -d '\r\n')

echo "MATOMO_BASE_URL=http://localhost:8080"
echo "MATOMO_API_TOKEN=$TOKEN"
```

- [ ] **Step 3: Make the script executable**

Run: `chmod +x scripts/bootstrap-matomo.sh`

- [ ] **Step 4: Write `.github/workflows/acceptance.yml`**

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
      - run: docker compose up -d --wait
      - run: ./scripts/bootstrap-matomo.sh >> "$GITHUB_ENV"
      - run: TF_ACC=1 go test ./... -run TestAcc -v -count=1 -timeout 20m
      - if: always()
        run: docker compose down -v
```

- [ ] **Step 5: Write the shared acceptance test helpers**

`internal/provider/acc_test_helpers.go`:
```go
package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAccPreCheck skips the calling test unless a real Matomo instance is
// configured. Every *_acc_test.go file's TestAcc* functions call this
// first, so these tests are inert (compiled, never executed) in the fast
// ci.yml job and only run in acceptance.yml.
func testAccPreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set, skipping acceptance test")
	}
	if os.Getenv("MATOMO_BASE_URL") == "" {
		t.Skip("MATOMO_BASE_URL not set, skipping acceptance test")
	}
	if os.Getenv("MATOMO_API_TOKEN") == "" {
		t.Skip("MATOMO_API_TOKEN not set, skipping acceptance test")
	}
}

// testAccProtoV6ProviderFactories builds the real provider (not backed by
// any httptest fixture) for use in resource.Test steps. The provider reads
// MATOMO_BASE_URL/MATOMO_API_TOKEN itself via its own Configure()
// env-var fallback, so no explicit `provider "matomo" { ... }` block is
// required in test configs — an empty `provider "matomo" {}` is enough.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"matomo": providerserver.NewProtocol6WithError(New("test")()),
}
```

- [ ] **Step 6: Write the smoke test**

`internal/provider/data_source_site_acc_test.go`:
```go
package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccSmoke_ProviderReachesRealMatomo is the first real-Matomo test in
// this codebase. It exists to prove the docker-compose fixture, bootstrap
// script, and acceptance.yml workflow reach a genuinely running Matomo
// instance end-to-end, before any resource-specific acceptance tests are
// added in later tasks. It creates a site directly via HCL and reads it
// back via the data source — the simplest possible real round trip.
func TestAccSmoke_ProviderReachesRealMatomo(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "smoke" {
  name = "Acceptance Smoke Test"
  urls = ["https://smoke-test.example.com"]
}

data "matomo_site" "smoke" {
  id = matomo_site.smoke.id
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.matomo_site.smoke", "name", "Acceptance Smoke Test"),
					resource.TestCheckResourceAttrPair("data.matomo_site.smoke", "id", "matomo_site.smoke", "id"),
				),
			},
		},
	})
}
```

- [ ] **Step 7: Verify what can be verified locally**

Run: `GOFLAGS=-mod=readonly go build -o /dev/null .` — expect success (no output).
Run: `go vet ./...` — expect no output.
Run: `gofmt -l docker-compose.yml scripts/bootstrap-matomo.sh internal/provider/acc_test_helpers.go internal/provider/data_source_site_acc_test.go` (gofmt only checks the two `.go` files; ignore it complaining about non-Go files, or just run it on the two `.go` paths) — expect no output.
Run: `go test ./internal/provider/... -run TestAccSmoke_ProviderReachesRealMatomo -v` (no `TF_ACC` set) — expect `--- SKIP` with the reason "TF_ACC not set, skipping acceptance test", confirming the pre-check guard works. This is the only local confirmation possible in this environment; genuine PASS against real Matomo is verified later by the controller via `acceptance.yml`.

- [ ] **Step 8: Commit**

```bash
git add docker-compose.yml scripts/bootstrap-matomo.sh .github/workflows/acceptance.yml internal/provider/acc_test_helpers.go internal/provider/data_source_site_acc_test.go
git commit -m "feat: add Matomo acceptance testing fixture, CI workflow, and smoke test"
```

---

### Task 2: Rename existing httptest-backed tests off the TestAcc* prefix

**Files:**
- Modify: `internal/provider/resource_site_test.go`
- Modify: `internal/provider/data_source_site_test.go`
- Modify: `internal/provider/resource_custom_dimension_test.go`
- Modify: `internal/provider/resource_tagmanager_container_test.go`
- Modify: `internal/provider/resource_tagmanager_tag_test.go`
- Modify: `internal/provider/resource_tagmanager_trigger_test.go`
- Modify: `internal/provider/resource_tagmanager_variable_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces: nothing new — purely a rename, no behavior change. Every `TestAccXxx` function name in these files becomes `TestUnitXxx` (same body, same assertions, same `resource.UnitTest` calls — only the function name changes).

This is mechanical. Do not touch anything else in these files — no logic changes, no reformatting beyond what the rename itself requires.

- [ ] **Step 1: List every TestAcc* function name currently in the package**

Run: `grep -rn 'func TestAcc' internal/provider/*_test.go`
Expected output (one line per existing test, do not rename anything not in this list — `data_source_site_acc_test.go` and any other `_acc_test.go` file from Task 1 must NOT be touched by this task, only the original httptest-backed files):
```
internal/provider/resource_site_test.go:func TestAccSiteResource_basic(t *testing.T) {
internal/provider/data_source_site_test.go:func TestAccSiteDataSource_byName(t *testing.T) {
internal/provider/resource_custom_dimension_test.go:func TestAccCustomDimensionResource_createsNewSlot(t *testing.T) {
internal/provider/resource_custom_dimension_test.go:func TestAccCustomDimensionResource_adoptsExistingSlot(t *testing.T) {
internal/provider/resource_custom_dimension_test.go:func TestAccCustomDimensionResource_idAndIndexDiffer(t *testing.T) {
internal/provider/resource_tagmanager_container_test.go:func TestAccTagManagerContainerResource_basic(t *testing.T) {
internal/provider/resource_tagmanager_tag_test.go:func TestAccTagManagerTagResource_basic(t *testing.T) {
internal/provider/resource_tagmanager_tag_test.go:func TestAccTagManagerTagResource_multipleParameters(t *testing.T) {
internal/provider/resource_tagmanager_trigger_test.go:func TestAccTagManagerTriggerResource_basic(t *testing.T) {
internal/provider/resource_tagmanager_trigger_test.go:func TestAccTagManagerTriggerResource_multipleParameters(t *testing.T) {
internal/provider/resource_tagmanager_variable_test.go:func TestAccTagManagerVariableResource_basic(t *testing.T) {
internal/provider/resource_tagmanager_variable_test.go:func TestAccTagManagerVariableResource_multipleParameters(t *testing.T) {
```
(If the actual output differs from this list — a test was added or renamed since this plan was written — rename exactly what `grep` finds, following the same `TestAcc` -> `TestUnit` substitution, and note the discrepancy in your report.)

- [ ] **Step 2: Rename each function**

For each file, replace `func TestAcc` with `func TestUnit` at the start of each matching function declaration (keep everything after the prefix identical — e.g. `TestAccSiteResource_basic` becomes `TestUnitSiteResource_basic`). Do this with a targeted find-and-replace per occurrence (not a blind global replace of the substring "Acc", which could corrupt unrelated identifiers like a variable named `accountID` if one existed — check there are none before using a broad tool-assisted replace).

- [ ] **Step 3: Run the full existing suite to confirm no regressions**

Run: `go test ./internal/provider/... -v -count=1 2>&1 | grep -E '^(--- (PASS|FAIL)|FAIL|ok)'`
Expected: every renamed test shows as `--- PASS: TestUnitXxx`, no `TestAccXxx` names remain except the new `TestAccSmoke_ProviderReachesRealMatomo` from Task 1 (which correctly shows `--- SKIP`, not `PASS` or `FAIL`, since `TF_ACC` isn't set locally).

- [ ] **Step 4: Confirm no stray TestAcc* names remain in the renamed files**

Run: `grep -rn 'func TestAcc' internal/provider/resource_site_test.go internal/provider/data_source_site_test.go internal/provider/resource_custom_dimension_test.go internal/provider/resource_tagmanager_container_test.go internal/provider/resource_tagmanager_tag_test.go internal/provider/resource_tagmanager_trigger_test.go internal/provider/resource_tagmanager_variable_test.go`
Expected: no output (empty).

- [ ] **Step 5: gofmt and build check**

Run: `gofmt -l internal/provider/*.go` — expect no output.
Run: `GOFLAGS=-mod=readonly go build -o /dev/null .` — expect success.

- [ ] **Step 6: Commit**

```bash
git add internal/provider/resource_site_test.go internal/provider/data_source_site_test.go internal/provider/resource_custom_dimension_test.go internal/provider/resource_tagmanager_container_test.go internal/provider/resource_tagmanager_tag_test.go internal/provider/resource_tagmanager_trigger_test.go internal/provider/resource_tagmanager_variable_test.go
git commit -m "test: rename httptest-backed TestAcc* to TestUnit*"
```

---

### Task 3: `matomo_site` + `data.matomo_site` acceptance tests

**Files:**
- Create: `internal/provider/resource_site_acc_test.go`
- Modify: `internal/provider/data_source_site_acc_test.go` (extend the Task 1 smoke test file with the data-source-specific tests; rename the smoke test's file role — see Step 1)

**Interfaces:**
- Consumes: `testAccPreCheck`, `testAccProtoV6ProviderFactories` (Task 1); `matomo.NewClient`, `matomo.Client.DeleteSite` (existing, `internal/matomo/sites.go`).
- Produces: the `TestAccXxx_basic` / `TestAccXxx_import` / `TestAccXxx_disappears` naming pattern every later resource task in this plan follows.

- [ ] **Step 1: Add an inline-client helper for disappears tests**

Every `_disappears` test in this plan needs to construct a `*matomo.Client` directly (bypassing Terraform) to delete a resource out-of-band. Add this shared helper to `internal/provider/acc_test_helpers.go` (append, don't replace):
```go

// testAccMatomoClient builds a *matomo.Client from the same environment
// variables the provider itself reads, for use in "disappears" tests that
// need to mutate Matomo directly, bypassing Terraform.
func testAccMatomoClient(t *testing.T) *matomo.Client {
	t.Helper()
	return matomo.NewClient(os.Getenv("MATOMO_BASE_URL"), os.Getenv("MATOMO_API_TOKEN"), nil)
}
```
Add `"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"` to `acc_test_helpers.go`'s imports.

- [ ] **Step 2: Write the failing site resource acceptance tests**

`internal/provider/resource_site_acc_test.go`:
```go
package provider

import (
	"context"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccSiteResource_basic(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Acceptance Test Site"
  urls = ["https://acc-test.example.com"]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("matomo_site.test", "name", "Acceptance Test Site"),
					resource.TestCheckResourceAttr("matomo_site.test", "urls.0", "https://acc-test.example.com"),
					resource.TestCheckResourceAttrSet("matomo_site.test", "id"),
				),
			},
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Acceptance Test Site Renamed"
  urls = ["https://acc-test.example.com"]
}
`,
				Check: resource.TestCheckResourceAttr("matomo_site.test", "name", "Acceptance Test Site Renamed"),
			},
		},
	})
}

func TestAccSiteResource_import(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Acceptance Import Test Site"
  urls = ["https://acc-import-test.example.com"]
}
`,
			},
			{
				ResourceName:      "matomo_site.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccSiteResource_disappears(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Acceptance Disappears Test Site"
  urls = ["https://acc-disappears-test.example.com"]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("matomo_site.test", "id"),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["matomo_site.test"]
						if !ok {
							return fmt.Errorf("matomo_site.test not found in state")
						}
						idSite, err := strconv.Atoi(rs.Primary.ID)
						if err != nil {
							return fmt.Errorf("invalid site id %q: %w", rs.Primary.ID, err)
						}
						client := testAccMatomoClient(t)
						return client.DeleteSite(context.Background(), idSite)
					},
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}
```

- [ ] **Step 3: Fix the disappears test's missing imports**

The `TestAccSiteResource_disappears` test above uses `*terraform.State` and `fmt.Errorf` — add `"fmt"` and `"github.com/hashicorp/terraform-plugin-testing/terraform"` to `resource_site_acc_test.go`'s imports. `resource.TestCheckFunc` (used by custom `Check` functions like this one) is defined as `func(*terraform.State) error` where `terraform` is this exact package — it's part of `terraform-plugin-testing`, already a direct dependency of this module (added in the foundation plan), so no new `go get` should be needed.

- [ ] **Step 4: Verify what can be verified locally**

Run: `GOFLAGS=-mod=readonly go build -o /dev/null .` — expect success. If it fails on the `terraform-plugin-testing/terraform` import (e.g. `go.mod` doesn't yet list a package version new enough to export it), run `go get github.com/hashicorp/terraform-plugin-testing@latest && go mod tidy` and retry.
Run: `go test ./internal/provider/... -run 'TestAccSiteResource' -v` (no `TF_ACC` set) — expect all three tests to `--- SKIP`.
Run: `gofmt -l internal/provider/resource_site_acc_test.go internal/provider/acc_test_helpers.go` — expect no output.

- [ ] **Step 5: Write the data source acceptance test**

Append to `internal/provider/data_source_site_acc_test.go` (the file already has `TestAccSmoke_ProviderReachesRealMatomo` from Task 1 — leave it in place, it's still a useful trivial round-trip check; add this alongside it):
```go

func TestAccSiteDataSource_byID(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Acceptance Data Source Test Site"
  urls = ["https://acc-ds-test.example.com"]
}

data "matomo_site" "test" {
  id = matomo_site.test.id
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.matomo_site.test", "name", "Acceptance Data Source Test Site"),
					resource.TestCheckResourceAttrPair("data.matomo_site.test", "id", "matomo_site.test", "id"),
				),
			},
		},
	})
}
```

- [ ] **Step 6: Verify and commit**

Run: `GOFLAGS=-mod=readonly go build -o /dev/null .` — expect success.
Run: `go test ./internal/provider/... -run 'TestAccSite' -v` (no `TF_ACC` set) — expect all tests to `--- SKIP`.
Run: `gofmt -l internal/provider/*.go` — expect no output.

```bash
git add internal/provider/resource_site_acc_test.go internal/provider/data_source_site_acc_test.go internal/provider/acc_test_helpers.go go.mod go.sum
git commit -m "test: add matomo_site and data.matomo_site acceptance tests"
```

---

### Task 4: `matomo_custom_dimension` acceptance tests, including id/index divergence

**Files:**
- Create: `internal/provider/resource_custom_dimension_acc_test.go`

**Interfaces:**
- Consumes: `testAccPreCheck`, `testAccProtoV6ProviderFactories`, `testAccMatomoClient` (Task 1/3); `matomo.Client.ConfigureExistingCustomDimension` (existing, `internal/matomo/customdimensions.go`), used by the disappears test to deactivate a dimension out-of-band (matching this resource's own Delete semantics — there is no real "delete" for custom dimensions, so the disappears test here checks that setting `active=false` directly is detected as drift, not a hard "not found").
- Produces: nothing new consumed elsewhere in this plan.

- [ ] **Step 1: Write the basic/import acceptance tests**

`internal/provider/resource_custom_dimension_acc_test.go`:
```go
package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCustomDimensionResource_basic(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Custom Dimension Acceptance Site"
  urls = ["https://acc-dimension-test.example.com"]
}

resource "matomo_custom_dimension" "test" {
  site_id = matomo_site.test.id
  index   = 1
  scope   = "visit"
  name    = "Acceptance Test Dimension"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("matomo_custom_dimension.test", "name", "Acceptance Test Dimension"),
					resource.TestCheckResourceAttr("matomo_custom_dimension.test", "active", "true"),
				),
			},
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Custom Dimension Acceptance Site"
  urls = ["https://acc-dimension-test.example.com"]
}

resource "matomo_custom_dimension" "test" {
  site_id = matomo_site.test.id
  index   = 1
  scope   = "visit"
  name    = "Acceptance Test Dimension Renamed"
}
`,
				Check: resource.TestCheckResourceAttr("matomo_custom_dimension.test", "name", "Acceptance Test Dimension Renamed"),
			},
		},
	})
}

func TestAccCustomDimensionResource_import(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Custom Dimension Import Site"
  urls = ["https://acc-dimension-import.example.com"]
}

resource "matomo_custom_dimension" "test" {
  site_id = matomo_site.test.id
  index   = 1
  scope   = "action"
  name    = "Acceptance Import Dimension"
}
`,
			},
			{
				ResourceName:      "matomo_custom_dimension.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
```

- [ ] **Step 2: Write the disappears test**

Custom dimensions can't be truly deleted, only deactivated (per spec/foundation plan). "Disappears" here means: deactivate directly via the client (bypassing Terraform), then confirm the next plan shows drift (the resource's own `active` attribute flips, which the framework surfaces as a non-empty plan proposing to set it back to `true`) rather than a hard error — this exercises the resource's Read()-driven drift detection, not a not-found error string (custom dimensions never 404, they only get deactivated).

Append to `internal/provider/resource_custom_dimension_acc_test.go`:
```go

func TestAccCustomDimensionResource_disappears(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Custom Dimension Disappears Site"
  urls = ["https://acc-dimension-disappears.example.com"]
}

resource "matomo_custom_dimension" "test" {
  site_id = matomo_site.test.id
  index   = 1
  scope   = "visit"
  name    = "Acceptance Disappears Dimension"
}
`,
				Check: func(s *terraform.State) error {
					rs, ok := s.RootModule().Resources["matomo_custom_dimension.test"]
					if !ok {
						return fmt.Errorf("matomo_custom_dimension.test not found in state")
					}
					siteID, index, err := parseDimensionID(rs.Primary.ID)
					if err != nil {
						return fmt.Errorf("invalid custom dimension id %q: %w", rs.Primary.ID, err)
					}
					client := testAccMatomoClient(t)
					ctx := context.Background()
					dims, err := client.GetConfiguredCustomDimensions(ctx, siteID)
					if err != nil {
						return err
					}
					for _, d := range dims {
						if d.Index == index && d.Scope == "visit" {
							return client.ConfigureExistingCustomDimension(ctx, d.ID, siteID, d.Name, false)
						}
					}
					return fmt.Errorf("dimension at index %d not found for out-of-band deactivation", index)
				},
				ExpectNonEmptyPlan: true,
			},
		},
	})
}
```
Add `"fmt"` and `"github.com/hashicorp/terraform-plugin-testing/terraform"` to this file's imports (same as Task 3 Step 3).

- [ ] **Step 3: Write the id/index divergence test**

This test directly verifies the assumption `internal/matomo/customdimensions.go`'s comments claim (added during the foundation plan's Task 13 fix): Matomo's `id` and `index` are different values that diverge once a site has more than one dimension across scopes.

Append to `internal/provider/resource_custom_dimension_acc_test.go`:
```go

func TestAccCustomDimensionResource_idIndexDivergence(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Dimension Divergence Site"
  urls = ["https://acc-dimension-divergence.example.com"]
}

resource "matomo_custom_dimension" "visit_dim" {
  site_id = matomo_site.test.id
  index   = 1
  scope   = "visit"
  name    = "Visit Dimension"
}

resource "matomo_custom_dimension" "action_dim" {
  site_id = matomo_site.test.id
  index   = 1
  scope   = "action"
  name    = "Action Dimension"
}
`,
				Check: func(s *terraform.State) error {
					visit, ok := s.RootModule().Resources["matomo_custom_dimension.visit_dim"]
					if !ok {
						return fmt.Errorf("matomo_custom_dimension.visit_dim not found in state")
					}
					action, ok := s.RootModule().Resources["matomo_custom_dimension.action_dim"]
					if !ok {
						return fmt.Errorf("matomo_custom_dimension.action_dim not found in state")
					}
					siteID, _, err := parseDimensionID(visit.Primary.ID)
					if err != nil {
						return err
					}
					client := testAccMatomoClient(t)
					dims, err := client.GetConfiguredCustomDimensions(context.Background(), siteID)
					if err != nil {
						return err
					}
					var visitID, actionID int
					for _, d := range dims {
						if d.Scope == "visit" && d.Index == 1 {
							visitID = d.ID
						}
						if d.Scope == "action" && d.Index == 1 {
							actionID = d.ID
						}
					}
					if visitID == 0 || actionID == 0 {
						return fmt.Errorf("could not find both dimensions via GetConfiguredCustomDimensions: visitID=%d actionID=%d", visitID, actionID)
					}
					if visitID == actionID {
						return fmt.Errorf("expected visit and action dimension ids to diverge (both index=1, different scopes), got same id=%d for both — Matomo's id/index behavior may not match internal/matomo/customdimensions.go's documented assumption, re-verify that comment", visitID)
					}
					t.Logf("confirmed id/index divergence: visit dimension id=%d index=1, action dimension id=%d index=1", visitID, actionID)
					_ = action // referenced only via Primary.ID above; keep for clarity that both resources are checked
					return nil
				},
			},
		},
	})
}
```

- [ ] **Step 4: Verify and commit**

Run: `GOFLAGS=-mod=readonly go build -o /dev/null .` — expect success.
Run: `go test ./internal/provider/... -run 'TestAccCustomDimensionResource' -v` (no `TF_ACC` set) — expect all four tests to `--- SKIP`.
Run: `gofmt -l internal/provider/resource_custom_dimension_acc_test.go` — expect no output.

```bash
git add internal/provider/resource_custom_dimension_acc_test.go
git commit -m "test: add matomo_custom_dimension acceptance tests including id/index divergence"
```

---

### Task 5: `matomo_tagmanager_container` acceptance tests

**Files:**
- Create: `internal/provider/resource_tagmanager_container_acc_test.go`

**Interfaces:**
- Consumes: `testAccPreCheck`, `testAccProtoV6ProviderFactories`, `testAccMatomoClient` (Task 1/3); `matomo.Client.DeleteContainer` (existing, `internal/matomo/tagmanager_containers.go`); `parseContainerID` (existing, `internal/provider/ids.go`).

- [ ] **Step 1: Write basic/import/disappears tests**

`internal/provider/resource_tagmanager_container_acc_test.go`:
```go
package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTagManagerContainerResource_basic(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Container Acceptance Site"
  urls = ["https://acc-container-test.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Acceptance Test Container"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("matomo_tagmanager_container.test", "name", "Acceptance Test Container"),
					resource.TestCheckResourceAttr("matomo_tagmanager_container.test", "context", "web"),
				),
			},
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Container Acceptance Site"
  urls = ["https://acc-container-test.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Acceptance Test Container Renamed"
}
`,
				Check: resource.TestCheckResourceAttr("matomo_tagmanager_container.test", "name", "Acceptance Test Container Renamed"),
			},
		},
	})
}

func TestAccTagManagerContainerResource_import(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Container Import Site"
  urls = ["https://acc-container-import.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Acceptance Import Container"
}
`,
			},
			{
				ResourceName:      "matomo_tagmanager_container.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccTagManagerContainerResource_disappears(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Container Disappears Site"
  urls = ["https://acc-container-disappears.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Acceptance Disappears Container"
}
`,
				Check: func(s *terraform.State) error {
					rs, ok := s.RootModule().Resources["matomo_tagmanager_container.test"]
					if !ok {
						return fmt.Errorf("matomo_tagmanager_container.test not found in state")
					}
					siteID, idContainer, err := parseContainerID(rs.Primary.ID)
					if err != nil {
						return fmt.Errorf("invalid container id %q: %w", rs.Primary.ID, err)
					}
					client := testAccMatomoClient(t)
					return client.DeleteContainer(context.Background(), siteID, idContainer)
				},
				ExpectNonEmptyPlan: true,
			},
		},
	})
}
```

- [ ] **Step 2: Verify and commit**

Run: `GOFLAGS=-mod=readonly go build -o /dev/null .` — expect success.
Run: `go test ./internal/provider/... -run 'TestAccTagManagerContainerResource' -v` (no `TF_ACC` set) — expect all three tests to `--- SKIP`.
Run: `gofmt -l internal/provider/resource_tagmanager_container_acc_test.go` — expect no output.

```bash
git add internal/provider/resource_tagmanager_container_acc_test.go
git commit -m "test: add matomo_tagmanager_container acceptance tests"
```

---

### Task 6: `matomo_tagmanager_tag` acceptance tests, including wire-format verification

**Files:**
- Create: `internal/provider/resource_tagmanager_tag_acc_test.go`

**Interfaces:**
- Consumes: `testAccPreCheck`, `testAccProtoV6ProviderFactories`, `testAccMatomoClient` (Task 1/3); `matomo.Client.DeleteContainerTag` (existing, `internal/matomo/tagmanager_tags.go`); `parseEntityID` (existing, `internal/provider/ids.go`).

- [ ] **Step 1: Write basic/import/disappears tests plus a multi-parameter wire-format test**

`internal/provider/resource_tagmanager_tag_acc_test.go`:
```go
package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTagManagerTagResource_basic(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Tag Acceptance Site"
  urls = ["https://acc-tag-test.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Tag Acceptance Container"
}

resource "matomo_tagmanager_tag" "test" {
  container_id = matomo_tagmanager_container.test.id
  type         = "CustomHtml"
  name         = "Acceptance Test Tag"
  parameter {
    name  = "customHtml"
    value = "<script>console.log('acceptance test')</script>"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("matomo_tagmanager_tag.test", "name", "Acceptance Test Tag"),
					resource.TestCheckResourceAttr("matomo_tagmanager_tag.test", "status", "active"),
					resource.TestCheckResourceAttr("matomo_tagmanager_tag.test", "parameter.0.name", "customHtml"),
				),
			},
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Tag Acceptance Site"
  urls = ["https://acc-tag-test.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Tag Acceptance Container"
}

resource "matomo_tagmanager_tag" "test" {
  container_id = matomo_tagmanager_container.test.id
  type         = "CustomHtml"
  name         = "Acceptance Test Tag"
  status       = "paused"
  parameter {
    name  = "customHtml"
    value = "<script>console.log('acceptance test')</script>"
  }
}
`,
				Check: resource.TestCheckResourceAttr("matomo_tagmanager_tag.test", "status", "paused"),
			},
		},
	})
}

func TestAccTagManagerTagResource_multipleParameters(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Tag Multi-Param Acceptance Site"
  urls = ["https://acc-tag-multiparam-test.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Tag Multi-Param Acceptance Container"
}

resource "matomo_tagmanager_tag" "test" {
  container_id = matomo_tagmanager_container.test.id
  type         = "CustomHtml"
  name         = "Acceptance Multi-Param Tag"
  parameter {
    name  = "alpha"
    value = "1"
  }
  parameter {
    name  = "bravo"
    value = "2"
  }
  parameter {
    name  = "charlie"
    value = "3"
  }
  parameter {
    name  = "delta"
    value = "4"
  }
}
`,
				Check: resource.TestCheckResourceAttr("matomo_tagmanager_tag.test", "parameter.#", "4"),
			},
		},
	})
}

func TestAccTagManagerTagResource_import(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Tag Import Site"
  urls = ["https://acc-tag-import.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Tag Import Container"
}

resource "matomo_tagmanager_tag" "test" {
  container_id = matomo_tagmanager_container.test.id
  type         = "CustomHtml"
  name         = "Acceptance Import Tag"
  parameter {
    name  = "customHtml"
    value = "<script></script>"
  }
}
`,
			},
			{
				ResourceName:      "matomo_tagmanager_tag.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccTagManagerTagResource_disappears(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Tag Disappears Site"
  urls = ["https://acc-tag-disappears.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Tag Disappears Container"
}

resource "matomo_tagmanager_tag" "test" {
  container_id = matomo_tagmanager_container.test.id
  type         = "CustomHtml"
  name         = "Acceptance Disappears Tag"
  parameter {
    name  = "customHtml"
    value = "<script></script>"
  }
}
`,
				Check: func(s *terraform.State) error {
					rs, ok := s.RootModule().Resources["matomo_tagmanager_tag.test"]
					if !ok {
						return fmt.Errorf("matomo_tagmanager_tag.test not found in state")
					}
					siteID, idContainer, idTag, err := parseEntityID(rs.Primary.ID)
					if err != nil {
						return fmt.Errorf("invalid tag id %q: %w", rs.Primary.ID, err)
					}
					client := testAccMatomoClient(t)
					ctx := context.Background()
					versionID, err := resolveDraftVersionID(ctx, client, siteID, idContainer)
					if err != nil {
						return err
					}
					return client.DeleteContainerTag(ctx, siteID, idContainer, versionID, idTag)
				},
				ExpectNonEmptyPlan: true,
			},
		},
	})
}
```

- [ ] **Step 2: Verify and commit**

Run: `GOFLAGS=-mod=readonly go build -o /dev/null .` — expect success.
Run: `go test ./internal/provider/... -run 'TestAccTagManagerTagResource' -v` (no `TF_ACC` set) — expect all four tests to `--- SKIP`.
Run: `gofmt -l internal/provider/resource_tagmanager_tag_acc_test.go` — expect no output.

```bash
git add internal/provider/resource_tagmanager_tag_acc_test.go
git commit -m "test: add matomo_tagmanager_tag acceptance tests including multi-parameter wire format"
```

---

### Task 7: `matomo_tagmanager_trigger` acceptance tests, including conditions wire-format verification

**Files:**
- Create: `internal/provider/resource_tagmanager_trigger_acc_test.go`

**Interfaces:**
- Consumes: `testAccPreCheck`, `testAccProtoV6ProviderFactories`, `testAccMatomoClient` (Task 1/3); `matomo.Client.DeleteContainerTrigger` (existing, `internal/matomo/tagmanager_triggers.go`); `resolveDraftVersionID`, `parseEntityID` (existing, Tasks 15/10 of the foundation plan).

- [ ] **Step 1: Write basic/import/disappears tests plus a conditions wire-format test**

`internal/provider/resource_tagmanager_trigger_acc_test.go`:
```go
package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTagManagerTriggerResource_basic(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Trigger Acceptance Site"
  urls = ["https://acc-trigger-test.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Trigger Acceptance Container"
}

resource "matomo_tagmanager_trigger" "test" {
  container_id = matomo_tagmanager_container.test.id
  type         = "PageView"
  name         = "Acceptance Test Trigger"
}
`,
				Check: resource.TestCheckResourceAttr("matomo_tagmanager_trigger.test", "name", "Acceptance Test Trigger"),
			},
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Trigger Acceptance Site"
  urls = ["https://acc-trigger-test.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Trigger Acceptance Container"
}

resource "matomo_tagmanager_trigger" "test" {
  container_id = matomo_tagmanager_container.test.id
  type         = "PageView"
  name         = "Acceptance Test Trigger Renamed"
}
`,
				Check: resource.TestCheckResourceAttr("matomo_tagmanager_trigger.test", "name", "Acceptance Test Trigger Renamed"),
			},
		},
	})
}

func TestAccTagManagerTriggerResource_withConditions(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Trigger Conditions Acceptance Site"
  urls = ["https://acc-trigger-conditions-test.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Trigger Conditions Acceptance Container"
}

resource "matomo_tagmanager_trigger" "test" {
  container_id = matomo_tagmanager_container.test.id
  type         = "PageView"
  name         = "Acceptance Conditions Trigger"
  condition {
    comparison = "equals"
    actual     = "url_path"
    value      = "/checkout"
  }
  condition {
    comparison = "contains"
    actual     = "url_domain"
    value      = "example.com"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("matomo_tagmanager_trigger.test", "condition.#", "2"),
					resource.TestCheckResourceAttr("matomo_tagmanager_trigger.test", "condition.0.comparison", "equals"),
					resource.TestCheckResourceAttr("matomo_tagmanager_trigger.test", "condition.1.comparison", "contains"),
				),
			},
		},
	})
}

func TestAccTagManagerTriggerResource_import(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Trigger Import Site"
  urls = ["https://acc-trigger-import.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Trigger Import Container"
}

resource "matomo_tagmanager_trigger" "test" {
  container_id = matomo_tagmanager_container.test.id
  type         = "PageView"
  name         = "Acceptance Import Trigger"
}
`,
			},
			{
				ResourceName:      "matomo_tagmanager_trigger.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccTagManagerTriggerResource_disappears(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Trigger Disappears Site"
  urls = ["https://acc-trigger-disappears.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Trigger Disappears Container"
}

resource "matomo_tagmanager_trigger" "test" {
  container_id = matomo_tagmanager_container.test.id
  type         = "PageView"
  name         = "Acceptance Disappears Trigger"
}
`,
				Check: func(s *terraform.State) error {
					rs, ok := s.RootModule().Resources["matomo_tagmanager_trigger.test"]
					if !ok {
						return fmt.Errorf("matomo_tagmanager_trigger.test not found in state")
					}
					siteID, idContainer, idTrigger, err := parseEntityID(rs.Primary.ID)
					if err != nil {
						return fmt.Errorf("invalid trigger id %q: %w", rs.Primary.ID, err)
					}
					client := testAccMatomoClient(t)
					ctx := context.Background()
					versionID, err := resolveDraftVersionID(ctx, client, siteID, idContainer)
					if err != nil {
						return err
					}
					return client.DeleteContainerTrigger(ctx, siteID, idContainer, versionID, idTrigger)
				},
				ExpectNonEmptyPlan: true,
			},
		},
	})
}
```

- [ ] **Step 2: Verify and commit**

Run: `GOFLAGS=-mod=readonly go build -o /dev/null .` — expect success.
Run: `go test ./internal/provider/... -run 'TestAccTagManagerTriggerResource' -v` (no `TF_ACC` set) — expect all four tests to `--- SKIP`.
Run: `gofmt -l internal/provider/resource_tagmanager_trigger_acc_test.go` — expect no output.

```bash
git add internal/provider/resource_tagmanager_trigger_acc_test.go
git commit -m "test: add matomo_tagmanager_trigger acceptance tests including conditions wire format"
```

---

### Task 8: `matomo_tagmanager_variable` acceptance tests

**Files:**
- Create: `internal/provider/resource_tagmanager_variable_acc_test.go`

**Interfaces:**
- Consumes: `testAccPreCheck`, `testAccProtoV6ProviderFactories`, `testAccMatomoClient` (Task 1/3); `matomo.Client.DeleteContainerVariable` (existing, `internal/matomo/tagmanager_variables.go`); `resolveDraftVersionID`, `parseEntityID` (existing).

- [ ] **Step 1: Write basic/import/disappears tests**

`internal/provider/resource_tagmanager_variable_acc_test.go`:
```go
package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTagManagerVariableResource_basic(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Variable Acceptance Site"
  urls = ["https://acc-variable-test.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Variable Acceptance Container"
}

resource "matomo_tagmanager_variable" "test" {
  container_id  = matomo_tagmanager_container.test.id
  type          = "Constant"
  name          = "Acceptance Test Variable"
  default_value = "acceptance-default"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("matomo_tagmanager_variable.test", "name", "Acceptance Test Variable"),
					resource.TestCheckResourceAttr("matomo_tagmanager_variable.test", "default_value", "acceptance-default"),
				),
			},
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Variable Acceptance Site"
  urls = ["https://acc-variable-test.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Variable Acceptance Container"
}

resource "matomo_tagmanager_variable" "test" {
  container_id  = matomo_tagmanager_container.test.id
  type          = "Constant"
  name          = "Acceptance Test Variable Renamed"
  default_value = "acceptance-default"
}
`,
				Check: resource.TestCheckResourceAttr("matomo_tagmanager_variable.test", "name", "Acceptance Test Variable Renamed"),
			},
		},
	})
}

func TestAccTagManagerVariableResource_import(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Variable Import Site"
  urls = ["https://acc-variable-import.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Variable Import Container"
}

resource "matomo_tagmanager_variable" "test" {
  container_id  = matomo_tagmanager_container.test.id
  type          = "Constant"
  name          = "Acceptance Import Variable"
  default_value = "n/a"
}
`,
			},
			{
				ResourceName:      "matomo_tagmanager_variable.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccTagManagerVariableResource_disappears(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Variable Disappears Site"
  urls = ["https://acc-variable-disappears.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Variable Disappears Container"
}

resource "matomo_tagmanager_variable" "test" {
  container_id  = matomo_tagmanager_container.test.id
  type          = "Constant"
  name          = "Acceptance Disappears Variable"
  default_value = "n/a"
}
`,
				Check: func(s *terraform.State) error {
					rs, ok := s.RootModule().Resources["matomo_tagmanager_variable.test"]
					if !ok {
						return fmt.Errorf("matomo_tagmanager_variable.test not found in state")
					}
					siteID, idContainer, idVariable, err := parseEntityID(rs.Primary.ID)
					if err != nil {
						return fmt.Errorf("invalid variable id %q: %w", rs.Primary.ID, err)
					}
					client := testAccMatomoClient(t)
					ctx := context.Background()
					versionID, err := resolveDraftVersionID(ctx, client, siteID, idContainer)
					if err != nil {
						return err
					}
					return client.DeleteContainerVariable(ctx, siteID, idContainer, versionID, idVariable)
				},
				ExpectNonEmptyPlan: true,
			},
		},
	})
}
```

- [ ] **Step 2: Verify and commit**

Run: `GOFLAGS=-mod=readonly go build -o /dev/null .` — expect success.
Run: `go test ./internal/provider/... -run 'TestAccTagManagerVariableResource' -v` (no `TF_ACC` set) — expect all three tests to `--- SKIP`.
Run: `gofmt -l internal/provider/resource_tagmanager_variable_acc_test.go` — expect no output.

```bash
git add internal/provider/resource_tagmanager_variable_acc_test.go
git commit -m "test: add matomo_tagmanager_variable acceptance tests"
```

---

### Task 9: Full acceptance run verification and error-string comment updates

**Files:** none created — this task is controller-driven verification against real CI, plus targeted comment edits based on the outcome.
- Modify (comment-only, or corrective if a real discrepancy is found): `internal/provider/resource_site.go`, `internal/provider/resource_tagmanager_container.go`, `internal/provider/resource_tagmanager_tag.go`, `internal/provider/resource_tagmanager_trigger.go`, `internal/provider/resource_tagmanager_variable.go` (the five NOTE comments added during the foundation plan's final-review fix).

**Interfaces:** none — this task consumes the full suite built in Tasks 1-8 and produces a verified/corrected codebase.

- [ ] **Step 1: Trigger the acceptance workflow**

This step is performed by the controller (not a dispatched implementer subagent), since it requires pushing the branch and using repository-level GitHub Actions access:
```bash
git push origin <branch-name>
```
Then trigger `workflow_dispatch` for `acceptance.yml` on that branch ref via the GitHub Actions API/MCP tool, and poll for completion.

- [ ] **Step 2: Read the run's logs**

If every job step (docker compose up, bootstrap script, `go test -run TestAcc`, docker compose down) succeeds: proceed to Step 3. If any step fails, the failure log is the deliverable — dispatch a fix task (not part of this plan's fixed step list, since the fix depends entirely on what actually failed: a wrong `console` flag in `scripts/bootstrap-matomo.sh`, a wrong hardcoded error string in a resource's `Read()`, a wrong wire-format assumption in `internal/matomo/tagmanager_{tags,triggers,variables}.go`, etc.) with the exact log output, then re-trigger and re-verify.

- [ ] **Step 3: Update the five error-string NOTE comments**

For each of `resource_site.go`, `resource_tagmanager_container.go`, `resource_tagmanager_tag.go`, `resource_tagmanager_trigger.go`, `resource_tagmanager_variable.go`: find the `// NOTE: ... unverified against live Matomo ...` comment (added during the foundation plan's final-review fix) above each `apiErr.Message == "..."` check, and replace "unverified against live Matomo" with "verified against a live Matomo instance via the acceptance suite in `*_acc_test.go` (see `TestAccXxxResource_disappears`)" — or, if Step 2 revealed the string was actually wrong, fix the string itself and note in the comment that it was corrected based on real Matomo's response, with the date.

- [ ] **Step 4: Final commit**

```bash
git add internal/provider/resource_site.go internal/provider/resource_tagmanager_container.go internal/provider/resource_tagmanager_tag.go internal/provider/resource_tagmanager_trigger.go internal/provider/resource_tagmanager_variable.go
git commit -m "docs: confirm drift-detection error strings verified against live Matomo"
git push origin <branch-name>
```

---

## Out of scope for this plan

Per spec §8:
- Any new provider resource, data source, or action.
- `tools/gen` codegen against the fixture (a separate future plan).
- Running the acceptance suite in this development session directly (blocked by the Docker daemon constraint) — verification happens via CI logs, per Task 9.
- Matomo versions other than whatever `matomo:latest` resolves to at fixture-build time.
