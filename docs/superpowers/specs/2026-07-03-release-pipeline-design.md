# Release Pipeline (Phase 8b) - Design

## 1. Goal and scope

Automate building, signing, and publishing versioned release artifacts for
`terraform-provider-matomo`, in the exact format the Terraform Registry
requires, so that:

- the provider can be installed privately today via a Terraform filesystem
  mirror, using real signed release artifacts (not a locally-built binary);
- publishing to the public Terraform Registry later requires no changes to
  the build/release pipeline itself - only a one-time registry-side
  listing step.

This is the last unbuilt piece of the original design spec's Phase 8
("Docs, examples, acceptance test suite, release pipeline"); docs, examples,
and the acceptance test suite were completed in Phase 8a (PR #11) and the
`matomo_custom_dimension` follow-up (PR #12).

Out of scope for this phase: actually submitting/listing the provider on
registry.terraform.io (GPG public key upload, manifest review, listing) -
that remains a deferred manual step for whenever the user chooses to
publish.

## 2. GoReleaser configuration (`.goreleaser.yml`)

Follows HashiCorp's standard `terraform-provider-scaffolding-framework`
template, since it produces exactly the artifact shapes and naming the
registry's ingestion pipeline expects:

- **Build matrix**: `darwin` (amd64, arm64), `freebsd` (386, amd64, arm,
  arm64), `linux` (386, amd64, arm, arm64), `windows` (386, amd64, arm) - the
  standard HashiCorp combination (11 total build targets; `arm` on Windows
  omits `arm64` and freebsd includes it, matching the upstream template
  exactly rather than a naive cross product).
- Binaries are built with `-ldflags="-X main.version={{.Version}} -X
  main.commit={{.Commit}}"`, consistent with `main.go`'s existing `var
  version = "dev"`.
- **Archives**: one zip per build target, named
  `terraform-provider-matomo_{{.Version}}_{{.Os}}_{{.Arch}}.zip`, containing
  just the binary and `LICENSE`.
- **Checksums**: a single `terraform-provider-matomo_{{.Version}}_SHA256SUMS`
  file covering all archives.
- **Signing**: `terraform-provider-matomo_{{.Version}}_SHA256SUMS` is
  GPG-detached-signed, producing `..._SHA256SUMS.sig`, using
  `gpg --batch --local-user {{ .Env.GPG_FINGERPRINT }} --detach-sign`.
- **Manifest**: emits `terraform-registry-manifest.json` declaring
  `metadata.protocol_versions: ["6.0"]` (matching
  `terraform-plugin-framework`'s protocol version 6, already in use by this
  provider) - a static file checked into the repo root, copied into the
  release rather than templated, per HashiCorp's own convention.
- **Release**: publishes a GitHub Release for the pushed tag, attaching all
  of the above artifacts.

## 3. Release workflow (`.github/workflows/release.yml`)

- **Trigger**: `push.tags: ['v*.*.*']` only - never runs on branch pushes or
  PRs, so it can't fire accidentally.
- **Steps**: checkout (`fetch-depth: 0`, needed for GoReleaser's changelog
  generation), `actions/setup-go`, import the GPG key
  (`crazy-max/ghaction-import-gpg@v6`, populating `GPG_FINGERPRINT` as an
  output consumed by `.goreleaser.yml`) from the `GPG_PRIVATE_KEY` +
  `PASSPHRASE` repo secrets, then
  `goreleaser/goreleaser-action@v6` running `release --clean`.
- Requires `contents: write` permission (to publish the GitHub Release) -
  no other workflows in this repo need or have that permission, so it's
  scoped to this one workflow file only.

## 4. Versioning

Plain semver git tags (`v0.1.0`, `v0.2.0`, `v1.0.0`, ...), pushed manually by
the repo owner when a release is wanted. No in-repo version file or bump
tooling - GoReleaser reads `{{.Version}}` directly from the tag, matching how
`main.go`'s `version` var is already designed to be set via `-ldflags` at
build time (currently defaults to the literal string `"dev"` for local
builds).

## 5. Manual, one-time prerequisite: GPG key setup

Not a pipeline component and not an implementation task - a set of commands
the repo owner runs once, locally, before the first tagged release. Recorded
here so the design is self-contained, and referenced (not repeated) from the
implementation plan:

```bash
# Generate a dedicated release-signing key (no passphrase-less keys - GitHub
# Actions needs the passphrase as a secret regardless, and an unprotected
# private key sitting in GitHub Secrets is a needlessly larger blast radius
# if the secret store is ever compromised).
gpg --full-generate-key
# ... choose RSA and RSA, 4096 bits, no expiration (or a long one), real
# name/email, and a passphrase.

# Export the private key (armored) for the GPG_PRIVATE_KEY secret:
gpg --armor --export-secret-keys <KEY_ID> | pbcopy   # or xclip/wl-copy

# Export the public key - needed later for registry submission, not for
# this phase, but worth saving now:
gpg --armor --export <KEY_ID> > matomo-provider-public-key.asc
```

Then, in the repo's GitHub Settings -> Secrets and variables -> Actions, add:

- `GPG_PRIVATE_KEY`: the armored private key output above.
- `PASSPHRASE`: the passphrase chosen during generation.

## 6. Private testing via filesystem mirror

New doc, `docs/guides/private-testing.md` (a Terraform Registry "guide" page,
consistent with `tfplugindocs`' existing guide-rendering support), covering:

1. Download and unzip the release archive matching the user's OS/arch from
   the GitHub Release.
2. Lay it out under a filesystem-mirror-compatible directory structure,
   e.g.:
   ```
   ~/.terraform.d/plugins/registry.terraform.io/nicole-ashley/matomo/0.1.0/<os>_<arch>/terraform-provider-matomo_v0.1.0
   ```
3. Add a filesystem mirror block to `~/.terraformrc`:
   ```hcl
   provider_installation {
     filesystem_mirror {
       path    = "/Users/you/.terraform.d/plugins"
       include = ["registry.terraform.io/nicole-ashley/matomo"]
     }
     direct {
       exclude = ["registry.terraform.io/nicole-ashley/matomo"]
     }
   }
   ```
4. A normal `required_providers` block referencing
   `nicole-ashley/matomo`, version-pinned to the installed release - no
   `dev_overrides`, no special provider source syntax, so the config used
   for private testing is identical to what a real registry consumer will
   write later.

This exercises the real signed artifact end to end (unzip, checksum
matching Terraform's own verification, binary execution) without requiring
registry publication.

## 7. Testing / verification

- No new Go code is introduced by this phase (workflow YAML, GoReleaser
  config, and docs only), so there's nothing to unit-test.
- Verification is: push a test tag (e.g. `v0.0.1-test1`) to a scratch branch
  or the real repo, confirm the `release` workflow runs green and produces a
  GitHub Release with all expected artifacts (11 zips, SHA256SUMS,
  SHA256SUMS.sig, manifest.json), then actually walk through the private
  filesystem-mirror guide against that release to confirm `terraform init`
  and a basic `plan` succeed against a signed, downloaded artifact (not a
  local build).
- `goreleaser check` (validates `.goreleaser.yml` syntax without building)
  can run in regular CI (`ci.yml`) as a fast sanity check on every push,
  independent of the tag-gated release workflow itself.

## 8. Repo layout additions

- `.goreleaser.yml` (repo root)
- `terraform-registry-manifest.json` (repo root)
- `.github/workflows/release.yml`
- `docs/guides/private-testing.md`
