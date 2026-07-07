# Release Readiness

SetupProof v0.1.3 is released through the Go module path, GitHub release
archives, and a versioned composite GitHub Action.

Before tagging a release, verify:

- `go test ./...`
- `go vet ./...`
- `go test -race ./...`
- `sh scripts/check-foundation.sh`
- `bash scripts/check-github-action.sh`
- `sh scripts/check-docs.sh`
- `sh scripts/check-examples.sh`
- `make schemas`
- `make release-archives VERSION=<major.minor.patch>`
- `make npm-check VERSION=<major.minor.patch>`
- `make release-check VERSION=<major.minor.patch>`

Release archive gates:

- Linux and macOS archives exist for `amd64` and `arm64`.
- The checksum manifest includes every archive.
- `scripts/check-release-archives.sh` verifies archive contents and checksums.
- Each extracted binary prints the expected version with `setupproof --version`.
- The GitHub release body includes the Go install command and the Action pin.

npm package gates:

- `scripts/package-npm.sh` stages a dependency-free package from the release
  archives.
- The package bundles Linux and macOS binaries for `amd64`/`arm64`; it does not
  use postinstall scripts, install-time downloads, native builds, telemetry, or
  hidden update checks.
- `scripts/check-npm-package.sh` runs `npm pack --dry-run`, packs the tarball,
  installs it into a clean temporary project, and verifies
  `setupproof --version`.
- npm install commands stay out of public install docs until the registry
  package is published.

Homebrew tap gates:

- `setupproof/homebrew-tap` formula uses the current release archive URLs and
  matching sha256 values from the release checksum manifest.
- `brew audit --strict --online setupproof/tap/setupproof` passes.
- `brew install setupproof/tap/setupproof`, `brew test
  setupproof/tap/setupproof`, and `setupproof --version` pass.

Release automation gates:

- `.github/workflows/release-checks.yml` runs the full repository gate, static
  analysis, vulnerability scan, workflow lint, archive verification, and the
  npm packed-tarball smoke test.
- The release checks workflow uses a patched Go toolchain for release and
  security gates rather than the oldest module language version.
- Keep the required SetupProof workflow small and fast; add slower
  release-oriented checks to the release checks workflow.

Schema publication gates:

- `schemas/` contains the source plan, report, and config schemas.
- `docs/schemas/v1.0.0/` contains the public GitHub Pages copies.
- Schema `$id` values point at the versioned public URLs.
- `make schemas` verifies the root schema files and public copies match before
  release publication.

Action wrapper gates:

- `scripts/github-action.sh` must continue to support source-tree `cli-path`
  usage.
- Binary download mode must validate the requested version, download the archive
  and checksum manifest for that exact version, verify the archive checksum, and
  check `setupproof --version` before execution.
- Binary download mode must honor `cli-sha256` when provided.
- Default examples must use `pull_request`, `permissions: contents: read`, no
  repository secrets, and `require-blocks: "true"`.
- Moving major Action tags must not be documented until tag movement policy and
  release automation exist.

Repository publication gates:

- `CODE_OF_CONDUCT.md` and `.github/pull_request_template.md` exist before the
  repository is made public.
- The SetupProof workflow is green on `main` in GitHub before public release.
- Repository metadata, topics, and the organization profile point to the
  current release path.

Deferred distribution gates:

- npm registry publication exists before npm install commands are documented.
- winget, Chocolatey, and Scoop packages exist before they are named as install
  options.
- Marketplace listing must exist before Marketplace availability is advertised.
