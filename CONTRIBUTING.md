# Contributing

SetupProof is in v0.1. Keep behavior changes aligned with the accepted ADRs in
`docs/adr/`.

Start with `docs/ARCHITECTURE.md` for the package map and invariants.
Follow `CODE_OF_CONDUCT.md` in project spaces.

## Fast Path

For most changes:

```sh
make fmt-check
make test
make vet
make dogfood
```

Run `make fmt` before staging if the formatting check reports Go files.

`make dogfood` includes untracked non-ignored files so new public docs,
examples, and tests are checked before they are staged. Ignored local-only files
stay out of the copied workspace.

Use `make race` for runner, report, stream, workspace, Docker, timeout, or
process-management changes.

For Docker runner changes, also run the real Docker integration test when
Docker is available:

```sh
SETUPPROOF_INTEGRATION_DOCKER=1 go test ./internal/runner -run Docker -count=1
```

`make build` writes a repo-root `./setupproof` binary. The binary is ignored by
Git and should not appear in commits.

## Full Gate

Before a release-oriented change, run:

```sh
make fmt-check
make check
make staticcheck
make vuln
make actionlint
```

`make check` runs the same repository checks documented in
`docs/RELEASE_READINESS.md`.

## Pull Requests

Use `.github/pull_request_template.md` when opening a pull request. Keep the
summary concrete, mark the affected area, and list the checks you ran.

For public docs, examples, schemas, reports, Action behavior, or release-facing
changes, run the full gate before requesting review.

Do not include secrets, tokens, private repository data, or vulnerability
details in public issues or pull requests. Follow `SECURITY.md` for
security-sensitive reports.

## Architecture

The core path is:

```text
Markdown -> Plan -> Workspace Copy -> Runner -> Report
```

Package ownership:

- `internal/markdown`: marked block discovery.
- `internal/config`: `setupproof.yml` parsing.
- `internal/planning`: target resolution, config merge, validation, and plan
  construction.
- `internal/runner`: workspace copy, process execution, Docker execution,
  timeout cleanup, and stream capture.
- `internal/report`: terminal, Markdown, and JSON reports.
- `internal/adoption`: `init`, `suggest`, `review`, and `doctor`.
- `schemas/`: public JSON contracts.
- `scripts/`: repository checks and release gates.
- `.github/`: issue templates, pull request template, and workflows.

## Scope Rules

- Do not document package-manager install commands until that distribution path
  exists.
- Do not document external Action tags or moving major tags until release
  packaging and tag policy exist.
- Do not add telemetry, hidden update checks, or automatic network calls to
  non-executing commands.
- Keep `suggest`, `review`, `doctor`, `--list`, and `--dry-run --json`
  non-executing.
- Preserve the plan/report JSON boundary unless a schema change is deliberate
  and documented.
- Keep native Windows and PowerShell support out of v0.1 unless the ADRs are
  updated first.

## Development Notes

Use focused tests for parser and planning changes. Use temp Git repositories for
runner tests so workspace-copy behavior is exercised through the same path users
run. Public docs should be small, direct, and honest about current packaging.

When changing public behavior, update at least one of:

- an ADR in `docs/adr/`;
- a JSON Schema in `schemas/`;
- a fixture under `examples/`;
- a focused package test under `internal/`.

Avoid broad refactors in the same change as behavior work. Small patches are
easier to review and safer to release.
