# Decisions

This page summarizes public-facing decisions. Additional decision records live
in `docs/adr/`.

## Runtime

SetupProof uses a Go CLI. The GitHub Action is a composite Action that invokes
the CLI. v0.1.0 is distributed through `go install`, GitHub release archives,
and a pinned composite Action. npm and operating-system package managers remain
deferred until their package checks exist.

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
archive declared by `cli-version`. The source-tree workflow remains available
for this repository and vendored copies. ADR 0009 records the earlier bootstrap
tradeoff and the source-tree workflow shape.

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

GitHub Marketplace listing is deferred for v0.1. The main product repository
remains the source of truth until current Marketplace requirements are checked
against the repository strategy.
