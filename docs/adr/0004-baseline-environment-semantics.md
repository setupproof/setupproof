# ADR 0004: Baseline Environment Semantics

Status: Accepted

Date: 2026-04-24

## Context

SetupProof should not inherit the user's full environment. It must pass only a
small baseline plus variables explicitly allowed by config, and it must redact
secrets before every output sink.

## Decision

No user environment variables pass by default beyond the sanitized baseline.

Baseline environment for `local` and `action-local`:

- `PATH` inherited from the host.
- `HOME` set to a directory inside the temporary workspace.
- `TMPDIR` set to a directory inside the temporary workspace when supported.
- `CI=true`.
- `SETUPPROOF=1`.
- UTF-8 locale variables set only when the host value is missing or unsafe.

Baseline environment for `docker`:

- `PATH` provided by the image.
- The host `PATH` is not inherited unless the user explicitly passes it through
  `env.allow`.
- `HOME` inside the container workspace.
- `TMPDIR` inside the container workspace when supported.
- `CI=true`.
- `SETUPPROOF=1`.
- UTF-8 locale variables set only when needed.

Environment config:

- `env.allow` passes named non-secret variables.
- `env.pass` passes named variables and may mark them `secret: true`.
- Missing optional variables are reported but do not fail.
- Missing required variables fail configuration or plan validation.
- Secret values are redacted before terminal output, Markdown reports, JSON
  reports, GitHub step summaries, workflow annotations, debug logs, and kept
  workspace diagnostics.

Network policy:

- SetupProof itself sends no telemetry, crash reports, usage data, repository
  contents, command logs, or hidden update checks.
- `network: false` is not enforceable for `local` or `action-local` in v0.1.
- If `network: false` is requested with `local` or `action-local`, plan
  validation fails with exit code `2` and suggests choosing `docker` or removing
  that network policy.

## Consequences

- Runner implementation must build environments from an allowlist, not by
  mutating inherited process environment in place.
- Redaction must be implemented before reports or GitHub summaries are treated
  as complete.
- Doctor should identify missing required environment variables without running
  marked blocks.
