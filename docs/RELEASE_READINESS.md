# Release Readiness

SetupProof v0.1 is a source-tree release until packaging work is complete.

Before documenting any external distribution channel, verify:

- `go test ./...`
- `go vet ./...`
- `go test -race ./...`
- `sh scripts/check-foundation.sh`
- `bash scripts/check-github-action.sh`
- `sh scripts/check-docs.sh`
- `sh scripts/check-examples.sh`
- `sh scripts/check-launch-polish.sh`

Distribution gates:

- npm package exists and packed-tarball smoke tests pass.
- Homebrew, winget, Chocolatey, and Scoop packages exist before they are named
  as install options.
- Stable external GitHub Action tags exist before external Action examples are
  documented.
- Moving major Action tags require a tag movement policy and release automation.
- Marketplace listing must exist before Marketplace availability is advertised.

Action wrapper gates:

- `scripts/github-action.sh` must continue to support source-tree `cli-path`
  usage.
- `action.yml` must keep the `cli-version` default empty until release archives
  and checksums are published for that version.
- Binary download mode must validate the requested version, download the archive
  and checksum manifest for that exact version, verify the archive checksum, and
  check `setupproof --version` before execution.
- Binary download mode must honor `cli-sha256` when provided.
- Default examples must use `pull_request`, `permissions: contents: read`, no
  repository secrets, and `require-blocks: "true"`.

Repository publication gates:

- `CODE_OF_CONDUCT.md` and `.github/pull_request_template.md` exist before the
  repository is made public.
- The SetupProof workflow is green on `main` in GitHub before public release.
- Badges are deferred until the public repository and CI URLs exist.
