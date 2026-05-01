# ADR 0007: No-Argument Behavior

Status: Accepted

Date: 2026-04-24

## Context

SetupProof should be convenient in the common README case without guessing
across the whole repository.

## Decision

When no target Markdown files are provided:

1. If config exists and lists files, use those files.
2. Otherwise, if `README.md` exists at the repository root, use `README.md`.
3. Otherwise, fail with exit code `2`.

The exit-code `2` message must include one concrete fix, such as passing a
Markdown file explicitly or adding `files` to `setupproof.yml`.

Generated CI examples should pass files explicitly or enable
`requireBlocks: true`.

## Consequences

- No-argument behavior must not perform a repository-wide Markdown walk in
  v0.1.
- `--all` remains out of scope.
- Doctor should verify no-argument resolution without executing marked blocks.
