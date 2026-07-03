# Decisions

This page summarizes public-facing decisions. Additional decision records live
in `docs/adr/`.

## Runtime

SetupProof uses a Go CLI. The GitHub Action is a composite Action that invokes
the CLI. v0.1.3 is distributed through `go install`, GitHub release archives,
Homebrew tap, and a pinned composite Action. The release tooling also stages
and smoke-tests an npm tarball; npm registry publication and additional
operating-system package managers remain deferred until those packages exist.

## Markers

SetupProof executes only explicitly marked shell blocks. Supported marker forms
are the renderer-compatible HTML comment before a fenced block and the compact
info-string form.

## Workspaces

Execution uses a Git tracked-plus-modified temporary copy by default. The copy
includes tracked files as they exist in the working tree, excludes ignored and
untracked files by default, and omits `.git`.

## Runners

`local` and `action-local` share one trusted-code execution path. The
`action-local` name is reserved for GitHub Action runs so reports identify the
CI context, while normal CLI runs default to `local`.

## GitHub Actions Checkout

The released Action uses normal workflow checkout and downloads the pinned CLI
archive declared by `cli-version`. `setupproof init --workflow` writes this
released Action workflow for normal repositories. The source-tree workflow
remains in this repository for dogfooding and vendored copies can adapt it
manually. ADR 0009 records the earlier bootstrap tradeoff and the source-tree
workflow shape.

## Shell Execution

Marked blocks run as non-interactive shell scripts. Stdin is closed, no TTY is
allocated, strict mode is enabled by default, and blocks in one target file share
state unless a block is isolated.

## Environment

No user environment variables pass by default beyond the sanitized baseline.
Named variables may be allowed or passed through config. Values marked secret
are redacted from supported output sinks.

## Reports

Dry-run plan JSON and execution report JSON are separate public contracts.
Execution reports include stable result states, numeric millisecond durations,
redacted output tails, and runner/workspace details.

## Schemas

SetupProof publishes JSON Schemas for plan output, report output, and
`setupproof.yml`. The schema files do not declare `$id` values until there is
an immutable public URL for released schemas.

## Marketplace

GitHub Marketplace listing is planned from the main product repository using
the root `action.yml`. Public docs do not advertise Marketplace availability
until the listing exists. ADR 0008 records the repository strategy and fallback.
