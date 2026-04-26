# Agent Usage

SetupProof gives coding agents a narrow contract for README setup commands:
marked shell blocks are maintainer-approved verification targets; unmarked shell
blocks are not execution targets.

## Agent Protocol

Use this sequence for a target Markdown file:

```sh
setupproof --list README.md
setupproof review README.md
setupproof --dry-run --json --require-blocks README.md
setupproof --require-blocks --no-color --no-glyphs README.md
```

Meaning:

- Discover: `setupproof --list README.md` prints marked block IDs without
  execution.
- Review: `setupproof review README.md` prints file, line, block ID, runner,
  shell, timeout, environment pass-through, and source without execution.
- Machine plan: `setupproof --dry-run --json --require-blocks README.md`
  emits a `kind: "plan"` document without execution.
- Execute: `setupproof --require-blocks --no-color --no-glyphs README.md`
  runs only marked shell blocks.

Never execute unmarked Markdown shell blocks as SetupProof targets.

For this source checkout, prefix commands with `go run ./cmd/setupproof` until
a local binary is built.

## Markers

Canonical marker:

````md
<!-- setupproof id=quickstart -->
```sh
go test ./...
```
````

Compact marker:

````md
```sh setupproof id=quickstart
go test ./...
```
````

Use stable IDs such as `quickstart`, `install`, `test`, or
`docker-compose-smoke`. IDs are local to one Markdown file.

## Non-Executing Commands

These commands must stay inspection-only:

- `setupproof suggest README.md`
- `setupproof review README.md`
- `setupproof doctor README.md`
- `setupproof --list README.md`
- `setupproof --dry-run --json --require-blocks README.md`

## Exit Codes

- `0`: passed, or no-op when blocks are not required.
- `1`: one or more marked blocks failed or timed out.
- `2`: usage, config, validation, or target selection error.
- `3`: runner infrastructure error.
- `4`: `--require-blocks` was set and no marked blocks were found.

## JSON Contracts

- Plan JSON: `schemas/setupproof-plan.schema.json`
- Report JSON: `schemas/setupproof-report.schema.json`
- Config JSON Schema: `schemas/setupproof-config.schema.json`

`--dry-run --json` emits plan JSON. Execution with `--json` or
`--report-json` emits report JSON. Do not treat plan JSON as proof that a
command passed.

## Safety Notes

SetupProof does not make arbitrary shell commands safe. It makes the execution
boundary explicit and reviewable.

The `local` and `action-local` runners use the same trusted-code execution
path. Use `local` for normal CLI runs; the GitHub Action defaults to
`action-local` so reports identify Action-originated execution. The Docker
runner improves isolation but is not a perfect sandbox.

SetupProof sends no telemetry and performs no hidden update checks. Secrets pass
only when configured.

For untrusted pull requests, inspect first:

```sh
setupproof review README.md
```

Do not pass repository secrets to jobs that execute contributor-controlled
commands.
