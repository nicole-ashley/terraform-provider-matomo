# Release Pipeline (Phase 8b) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a tag-triggered GoReleaser + GitHub Actions pipeline that builds, checksums, GPG-signs, and publishes multi-platform release artifacts in the exact format the Terraform Registry expects, plus a guide for consuming those artifacts privately via a Terraform filesystem mirror.

**Architecture:** `.goreleaser.yml` (built from HashiCorp's own `terraform-provider-scaffolding-framework` template, fetched and verified against the live upstream file rather than assumed) drives the build/archive/checksum/sign/release steps; `.github/workflows/release.yml` triggers it on `v*` tag pushes; a static `terraform-registry-manifest.json` declares the plugin protocol version. No new Go code - this phase only adds config, workflow YAML, and docs.

**Tech Stack:** GoReleaser v2 (consumed via `go tool`, matching this repo's existing `tfplugindocs` pattern), GitHub Actions (`crazy-max/ghaction-import-gpg`, `goreleaser/goreleaser-action`), GPG.

## Global Constraints

- GoReleaser config must match the verified upstream `hashicorp/terraform-provider-scaffolding-framework` template exactly for the build matrix, archive naming, checksum naming, and signing invocation - do not improvise a different shape, since the registry (when the user publishes later) expects this exact layout.
- No GPG private key material, passphrase, or any other secret is ever written into the repo. Task 2's release workflow only *references* `secrets.GPG_PRIVATE_KEY` / `secrets.PASSPHRASE` / `secrets.GITHUB_TOKEN` - actually creating those secrets is a manual, one-time step the user performs in GitHub Settings, documented in the spec (`docs/superpowers/specs/2026-07-03-release-pipeline-design.md`, section 5) and referenced, not repeated, from this plan.
- Actual Terraform Registry submission (public key upload, listing) is out of scope for this plan.
- Existing repo convention for GitHub Actions dependencies is tag-pinned (`@v4`, `@v5`, `@v7`), not SHA-pinned - follow that convention for consistency with `.github/workflows/ci.yml` and `.github/workflows/acceptance.yml`, even though upstream's own release.yml uses SHA pins.
- `go tool` is this repo's established pattern for CLI dependencies that aren't imported by any `.go` file (see `go.mod`'s `tool github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs` line and the `GNUmakefile`'s `docs` target) - GoReleaser must be added the same way, not as a bare `go install`.

---

### Task 1: GoReleaser config + registry manifest

**Files:**
- Modify: `go.mod` (add GoReleaser as a `tool` dependency)
- Create: `.goreleaser.yml`
- Create: `terraform-registry-manifest.json`
- Modify: `GNUmakefile` (add `release-check` and `release-snapshot` targets)

**Interfaces:**
- Consumes: nothing from other tasks.
- Produces: `go tool goreleaser` becomes runnable from anywhere in the repo; `make release-check` and `make release-snapshot` become the standard local verification commands, reused in Task 4's full verification pass.

- [ ] **Step 1: Add GoReleaser as a go tool dependency**

Run:

```bash
go get -tool github.com/goreleaser/goreleaser/v2@latest
```

This adds a `require` line for `github.com/goreleaser/goreleaser/v2` and a second `tool github.com/goreleaser/goreleaser/v2` line to `go.mod` (alongside the existing `tfplugindocs` tool line), and updates `go.sum`.

- [ ] **Step 2: Verify the tool resolves**

Run: `go tool goreleaser --version`
Expected: prints a GoReleaser version banner (e.g. `GoReleaser version X.Y.Z`), confirming the module was fetched and builds cleanly. Non-zero exit or a Go compile error means Step 1 didn't complete correctly - re-run `go mod tidy` and retry.

- [ ] **Step 3: Create `.goreleaser.yml`**

This is the verified upstream HashiCorp template verbatim (fetched from `hashicorp/terraform-provider-scaffolding-framework`'s live `.goreleaser.yml` - do not modify the build matrix, archive naming, checksum naming, or signing block; only the `before.hooks` step is worth double-checking against this repo's own `go.mod tidy` needs, which is identical here):

```yaml
# Visit https://goreleaser.com for documentation on how to customize this
# behavior.
version: 2
before:
  hooks:
    - go mod tidy
builds:
- env:
    - CGO_ENABLED=0
  mod_timestamp: '{{ .CommitTimestamp }}'
  flags:
    - -trimpath
  ldflags:
    - '-s -w -X main.version={{.Version}} -X main.commit={{.Commit}}'
  goos:
    - freebsd
    - windows
    - linux
    - darwin
  goarch:
    - amd64
    - '386'
    - arm
    - arm64
  ignore:
    - goos: darwin
      goarch: '386'
    - goos: windows
      goarch: arm
  binary: '{{ .ProjectName }}_v{{ .Version }}'
archives:
- formats:
  - zip
  name_template: '{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}'
checksum:
  extra_files:
    - glob: 'terraform-registry-manifest.json'
      name_template: '{{ .ProjectName }}_{{ .Version }}_manifest.json'
  name_template: '{{ .ProjectName }}_{{ .Version }}_SHA256SUMS'
  algorithm: sha256
signs:
  - artifacts: checksum
    args:
      - "--batch"
      - "--local-user"
      - "{{ .Env.GPG_FINGERPRINT }}"
      - "--output"
      - "${signature}"
      - "--detach-sign"
      - "${artifact}"
release:
  extra_files:
    - glob: 'terraform-registry-manifest.json'
      name_template: '{{ .ProjectName }}_{{ .Version }}_manifest.json'
changelog:
  disable: true
```

- [ ] **Step 4: Create `terraform-registry-manifest.json`**

```json
{
    "version": 1,
    "metadata": {
        "protocol_versions": ["6.0"]
    }
}
```

`"6.0"` matches this provider's existing `terraform-plugin-framework` protocol version (framework v1.x always speaks protocol 6) - confirm this by checking that `main.go`'s `providerserver.Serve` call uses no explicit `ProtocolVersion` override (it doesn't, as of the current `main.go`), which means it defaults to the protocol the framework itself implements, i.e. 6.

- [ ] **Step 5: Validate the config without building**

Run: `go tool goreleaser check`
Expected: `configuration is valid` (or similar success message), exit code 0. If it reports a schema error, the most likely cause is a typo introduced while copying Step 3's YAML - diff it against the block above.

- [ ] **Step 6: Run a snapshot build (no tag, no signing, no publish)**

Run:

```bash
go tool goreleaser release --snapshot --clean --skip=sign,publish
```

Expected: exits 0, creates a `dist/` directory. `--snapshot` lets GoReleaser run without a git tag present (it fabricates a pseudo-version like `0.1.0-next`); `--skip=sign,publish` avoids needing a GPG key or `GITHUB_TOKEN` for this local check.

- [ ] **Step 7: Confirm the artifact set**

Run: `ls dist/*.zip | wc -l`
Expected: `13` - the actual valid Go cross-compile targets (confirmed via `go tool dist list`, since `darwin/arm` and `windows/arm` are not real Go build targets at all, making the corresponding .goreleaser.yml ignore entries defensive/vestigial rather than load-bearing): freebsd x4 (386, amd64, arm, arm64), windows x3 (386, amd64, arm64), linux x4 (386, amd64, arm, arm64), darwin x2 (amd64, arm64) = 4+3+4+2 = 13.

Run: `ls dist/ | grep -E 'SHA256SUMS$|manifest.json$'`
Expected: two lines - `terraform-provider-matomo_0.1.0-next_SHA256SUMS` (or similar snapshot-versioned name) and `terraform-provider-matomo_0.1.0-next_manifest.json`.

Run: `cat dist/terraform-provider-matomo_*_manifest.json`
Expected: contents identical to Step 4's `terraform-registry-manifest.json` (GoReleaser copies it through under the versioned name), confirming the `release.extra_files` / `checksum.extra_files` wiring is correct.

- [ ] **Step 8: Add Makefile convenience targets**

Add to `GNUmakefile` (after the existing `docs` target):

```makefile
.PHONY: release-check
release-check:
	go tool goreleaser check

.PHONY: release-snapshot
release-snapshot:
	go tool goreleaser release --snapshot --clean --skip=sign,publish
```

- [ ] **Step 9: Re-run via the new Makefile targets to confirm they work**

Run: `make release-check && make release-snapshot`
Expected: same success output as Steps 5-6.

- [ ] **Step 10: Clean up local build output and commit**

`dist/` must never be committed (it's a local build artifact, regenerated by CI on every real release). Check `.gitignore` first:

```bash
grep -q '^dist/$' .gitignore || echo 'dist/' >> .gitignore
rm -rf dist/
```

```bash
git add go.mod go.sum .goreleaser.yml terraform-registry-manifest.json GNUmakefile .gitignore
git commit -m "Add GoReleaser config and Terraform Registry manifest"
```

---

### Task 2: Release workflow + CI config-drift guard

**Files:**
- Create: `.github/workflows/release.yml`
- Modify: `.github/workflows/ci.yml` (add a `goreleaser check` step)

**Interfaces:**
- Consumes: `make release-check` / `go tool goreleaser check` from Task 1 (must already pass before this task's CI step is added, since the new CI step will fail the build otherwise).
- Produces: a `release` GitHub Actions workflow that fires on `v*` tag pushes; nothing downstream in this plan consumes it further (Task 4 exercises it manually via a scratch tag, not via code).

- [ ] **Step 1: Create `.github/workflows/release.yml`**

Tag-pinned to match this repo's existing convention (`ci.yml` uses `actions/checkout@v4`, `actions/setup-go@v5`):

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write

jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: Import GPG key
        uses: crazy-max/ghaction-import-gpg@v6
        id: import_gpg
        with:
          gpg_private_key: ${{ secrets.GPG_PRIVATE_KEY }}
          passphrase: ${{ secrets.PASSPHRASE }}
      - name: Run GoReleaser
        run: go tool goreleaser release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          GPG_FINGERPRINT: ${{ steps.import_gpg.outputs.fingerprint }}
```

Note this uses `go tool goreleaser` (via `go run`, consistent with Task 1's `go.mod` tool dependency) rather than `goreleaser/goreleaser-action`, since the tool dependency already pins the exact version in `go.sum` - a separate action would need its own independent version pin and could drift out of sync.

- [ ] **Step 2: Sanity-check the new workflow YAML parses**

Run:

```bash
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))" && echo "valid YAML"
```

Expected: `valid YAML`. This only checks YAML syntax, not GitHub Actions semantics (there's no `actionlint` available in this environment) - the real test is Task 4's scratch-tag push.

- [ ] **Step 3: Add a `goreleaser check` step to `ci.yml`**

Modify `.github/workflows/ci.yml`'s `test` job by adding a new step after the existing "Check docs are up to date" step (so any future edit to `.goreleaser.yml` that breaks validation fails fast on every push, not just on a real tag):

```yaml
      - name: Check GoReleaser config is valid
        run: go tool goreleaser check
```

- [ ] **Step 4: Verify the new CI step locally**

Run: `go tool goreleaser check`
Expected: same success output confirmed in Task 1 Step 5 - this just confirms the exact command about to run in CI still passes before pushing.

- [ ] **Step 5: Sanity-check the modified `ci.yml` still parses**

Run:

```bash
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml'))" && echo "valid YAML"
```

Expected: `valid YAML`.

- [ ] **Step 6: Commit**

```bash
git add .github/workflows/release.yml .github/workflows/ci.yml
git commit -m "Add tag-triggered release workflow and GoReleaser config check to CI"
```

---

### Task 3: Private-testing guide

**Files:**
- Create: `docs/guides/private-testing.md`

**Interfaces:**
- Consumes: nothing (a static hand-written doc page; `tfplugindocs` does not generate or overwrite guide content, only resource/data-source/provider pages, so this file is exempt from `ci.yml`'s docs drift-check).
- Produces: nothing consumed elsewhere in this plan - this is a leaf deliverable, verified by Task 4's full walkthrough.

- [ ] **Step 1: Create the guide**

`tfplugindocs` guide pages use the same YAML front-matter shape as generated resource pages (see `docs/resources/custom_dimension.md` for the pattern: `page_title`, `subcategory`, `description`).

```markdown
---
page_title: "Private testing without the Terraform Registry"
subcategory: ""
description: |-
  How to install and test terraform-provider-matomo from a GitHub Release, before it's published to the Terraform Registry.
---

# Private testing without the Terraform Registry

This provider isn't published to the public Terraform Registry yet. Until it
is, you can still install and test a real, signed release build - the exact
artifact format the registry itself will serve later - using Terraform's
[filesystem mirror](https://developer.hashicorp.com/terraform/cli/config/config-file#filesystem_mirror)
provider installation method.

## 1. Download a release

Go to the repository's [Releases page](https://github.com/nicole-ashley/terraform-provider-matomo/releases)
and download the zip matching your OS and architecture, e.g.
`terraform-provider-matomo_0.1.0_linux_amd64.zip` for 64-bit Linux.

## 2. Lay the binary out for a filesystem mirror

Terraform's filesystem mirror expects a specific directory structure:
`<mirror-root>/registry.terraform.io/nicole-ashley/matomo/<version>/<os>_<arch>/`,
containing the extracted provider binary.

```shell
VERSION=0.1.0
OS_ARCH=linux_amd64  # match the zip you downloaded

mkdir -p ~/.terraform.d/plugins/registry.terraform.io/nicole-ashley/matomo/${VERSION}/${OS_ARCH}
unzip terraform-provider-matomo_${VERSION}_${OS_ARCH}.zip -d ~/.terraform.d/plugins/registry.terraform.io/nicole-ashley/matomo/${VERSION}/${OS_ARCH}
```

## 3. Point Terraform at the mirror

Add this to `~/.terraformrc` (create the file if it doesn't exist):

```hcl
provider_installation {
  filesystem_mirror {
    path    = "/home/you/.terraform.d/plugins"
    include = ["registry.terraform.io/nicole-ashley/matomo"]
  }
  direct {
    exclude = ["registry.terraform.io/nicole-ashley/matomo"]
  }
}
```

Replace `/home/you/.terraform.d/plugins` with the real, absolute path to the
directory from step 2 (not `~` - Terraform's CLI config does not expand it).

## 4. Use it like any registry provider

```hcl
terraform {
  required_providers {
    matomo = {
      source  = "nicole-ashley/matomo"
      version = "0.1.0"
    }
  }
}

provider "matomo" {
  base_url  = "https://analytics.example.com"
  api_token = var.matomo_api_token
}
```

Run `terraform init`. Terraform will read the binary straight from the
mirror instead of contacting the registry, while verifying it against the
release's checksums exactly as it would for a registry-hosted provider.
There is no `dev_overrides` block and no special provider source syntax
here - the configuration above is identical to what you'd write once the
provider is actually published, so nothing needs to change later.
```

- [ ] **Step 2: Confirm the guide doesn't interfere with the docs drift-check**

Run: `make docs && git status --short docs/`
Expected: no output (clean) other than possibly `docs/guides/private-testing.md` itself if it wasn't yet tracked - `make docs` (i.e. `tfplugindocs generate`) must not modify or delete this file, since it's hand-written, not generated from schema.

- [ ] **Step 3: Commit**

```bash
git add docs/guides/private-testing.md
git commit -m "Add private-testing guide for filesystem-mirror installs"
```

---

### Task 4: Full verification pass

**Files:** none (verification only, but may produce a fixup commit if any check fails).

**Interfaces:**
- Consumes: everything from Tasks 1-3.
- Produces: confidence the pipeline works end-to-end before the user ever pushes a real `v*` tag.

- [ ] **Step 1: Full local build/lint/test/docs/release-check pass**

Run:

```bash
go build -o /dev/null . && \
go test ./... -v -count=1 && \
golangci-lint run ./... && \
make docs && git diff --exit-code -- docs/ && \
make release-check
```

Expected: all commands succeed; `git diff --exit-code -- docs/` prints nothing (no drift).

- [ ] **Step 2: Snapshot release build, full artifact audit**

Run:

```bash
make release-snapshot
ls dist/*.zip | wc -l          # expect 13
ls dist/ | grep SHA256SUMS     # expect exactly one file
ls dist/ | grep manifest.json  # expect exactly one file
unzip -l dist/terraform-provider-matomo_*_linux_amd64.zip
```

Expected: the `unzip -l` listing shows exactly two entries - a `terraform-provider-matomo_v<version>` binary and `LICENSE` (GoReleaser's default archive `files` glob picks up `LICENSE*` automatically; confirm no unexpected extra files like `README*` snuck in, since this repo has no root `README.md` to accidentally include).

Run: `rm -rf dist/`

- [ ] **Step 3: Push a scratch tag to exercise the real release workflow**

This is the only step in this plan that requires GitHub secrets (`GPG_PRIVATE_KEY`, `PASSPHRASE`) to already be configured, per the spec's section 5 manual prerequisite - **do not attempt this step until the user confirms those secrets exist**. If they don't yet, stop here and report back what's left (this step and the guide walkthrough in Step 4) rather than pushing a tag that will predictably fail on the signing step.

```bash
git tag v0.0.1-test1
git push origin v0.0.1-test1
```

Then check the `release` workflow run in GitHub Actions.
Expected: green run, producing a GitHub Release named `v0.0.1-test1` with 13 zips, one `SHA256SUMS`, one `SHA256SUMS.sig`, and one `manifest.json` attached.

- [ ] **Step 4: Walk through the private-testing guide against the real release**

Follow `docs/guides/private-testing.md` verbatim, using the `v0.0.1-test1` release from Step 3 and a scratch Terraform config in a throwaway directory (e.g. `/tmp/matomo-mirror-test`). Confirm `terraform init` succeeds and reports installing `nicole-ashley/matomo` from the local filesystem mirror (not from the registry), and that `terraform plan` at least gets as far as provider-level validation (a full `plan`/`apply` needs live Matomo credentials, which is out of scope for this check - reaching provider configuration without an installation error is sufficient).

- [ ] **Step 5: Clean up the scratch tag**

The scratch tag and its release are test artifacts, not a real version - remove them so they don't confuse future consumers of the repo's release history:

```bash
git push --delete origin v0.0.1-test1
git tag -d v0.0.1-test1
```

Then delete the corresponding GitHub Release (`v0.0.1-test1`) via the GitHub UI or `gh release delete v0.0.1-test1 --yes` if available.

- [ ] **Step 6: Report status**

Summarize for the user: confirm the full build/lint/test/docs/release-check pass (Step 1), the snapshot artifact audit (Step 2), and - if the GPG secrets were already configured - the live scratch-tag release and filesystem-mirror walkthrough (Steps 3-5). If secrets weren't configured yet, report that Steps 3-5 are the remaining work, blocked on the user adding `GPG_PRIVATE_KEY`/`PASSPHRASE` per the spec's section 5.
