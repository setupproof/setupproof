# Release Readiness

SetupProof v0.1.0 is released through the Go module path, GitHub release
archives, and a versioned composite GitHub Action.

Before tagging a release, verify:

- `go test ./...`
- `go vet ./...`
- `go test -race ./...`
- `sh scripts/check-foundation.sh`
- `bash scripts/check-github-action.sh`
- `sh scripts/check-docs.sh`
- `sh scripts/check-examples.sh`
- `make release-archives VERSION=<major.minor.patch>`

Release archive gates:

- Linux and macOS archives exist for `amd64` and `arm64`.
- The checksum manifest includes every archive.
- Each extracted binary prints the expected version with `setupproof --version`.
- The GitHub release body includes the Go install command and the Action pin.

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

- npm package exists and packed-tarball smoke tests pass before npm install
  commands are documented.
- Homebrew, winget, Chocolatey, and Scoop packages exist before they are named
  as install options.
- Marketplace listing must exist before Marketplace availability is advertised.
