# ADR 0006: JSON Report And Plan Schema Boundaries

Status: Accepted

Date: 2026-04-24

## Context

SetupProof needs machine-readable output for both dry-run planning and real
execution. Those outputs serve different consumers and must not blur execution
results with plans.

## Decision

Publish separate schemas:

- `schemas/setupproof-plan.schema.json`.
- `schemas/setupproof-report.schema.json`.

Both schemas start at `1.0.0` and include required `kind` and `schemaVersion`
fields.

Plan JSON:

- Has `kind: "plan"`.
- Is emitted by `--dry-run --json`.
- Describes selected files, discovered blocks, effective options, report sinks,
  runner choice, workspace copy mode, state mode, network policy, and validation
  warnings or errors.
- Describes configured environment passthrough names and whether `env.pass`
  variables are present, but never includes environment values.
- Does not include execution durations, exit status from user commands, stdout
  tails, stderr tails, or command results.

Report JSON:

- Has `kind: "report"`.
- Is emitted by an execution run with `--json`.
- Is written to a file by `--report-json <path>` during execution runs.
- Includes run timing, result, exit code, runner details, workspace details,
  warnings, block results, redacted output tails, and truncation indicators.
- Represents no-block successful runs as `result: "noop"` and `blocks: []`.
- Distinguishes common interrupted-run infrastructure reasons as
  `signal_interrupt` and `signal_terminate` when the host reports the signal.

Boundary rules:

- `--dry-run --report-json <path>` is rejected with exit code `2`.
- `--dry-run --report-file <path>` is rejected with exit code `2`.
- Use `--dry-run --json > plan.json` for a plan file.
- `--json` writes only the primary JSON payload plus one trailing newline to
  stdout.
- Progress, diagnostics, warnings, and redacted command logs go to stderr.
- JSON key ordering should be deterministic.
- Durations in JSON are numeric milliseconds.

## Consequences

- Plan JSON can ship independently from execution report JSON.
- Report JSON and both published schemas must remain in sync.
- Tests must prove that JSON stdout has no progress noise.
