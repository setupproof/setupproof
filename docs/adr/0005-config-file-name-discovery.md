# ADR 0005: Config File Name And Discovery

Status: Accepted

Date: 2026-04-24

## Context

SetupProof needs one predictable v0.1 config location. Extra discovery surfaces
increase ambiguity and make reports harder to audit.

## Decision

The v0.1 config file is `setupproof.yml` at the repository root.

Discovery rules:

- Discover only `setupproof.yml` at the repository root.
- Do not discover dotfile configs in v0.1.
- Do not read `package.json` config in v0.1.
- Do not use an environment variable to point to config in v0.1.
- `--config <path>` overrides discovery with one explicit file.

Path rules:

- Resolve the repository root before resolving config.
- `--config <path>` resolves relative to the invocation working directory.
- Config paths must point to files inside the repository root.
- Config paths that escape the repository root are rejected.
- Symlinks that resolve outside the repository root are rejected.
- Config `files` entries are repository-root-relative and must not be absolute.

Validation rules:

- `version: 1` is required.
- Unknown top-level fields are rejected unless prefixed with `x-`.
- Invalid runners, invalid network values, invalid timeouts, missing files,
  duplicate explicit IDs within a file, and block config entries without both
  `file` and `id` fail validation.
- No marked blocks is a warning when `requireBlocks` is false and exit code `4`
  when it is true.

## Consequences

- Config loading should build on Markdown discovery.
- Public examples should show `setupproof.yml`, not dotfiles or package
  metadata.
- Additional config names require a later ADR.
