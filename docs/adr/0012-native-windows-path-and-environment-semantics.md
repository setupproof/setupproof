# ADR 0012: Native Windows Path And Environment Semantics

Status: Accepted

Date: 2026-07-07

## Context

ADR 0010 keeps native Windows execution outside v0.1 scope, and ADR 0011 keeps
PowerShell and `cmd` fences unsupported. Native Windows support also needs an
explicit path and environment contract before SetupProof can safely advertise
native execution.

Windows path and environment behavior differs from the current POSIX-shaped
runner model. Drive-letter paths, UNC paths, backslash separators,
case-insensitive environment names, process-level environment inheritance, and
report path rendering all affect reviewability and JSON compatibility.

## Decision

Native Windows path and environment behavior remains unsupported in v0.1.
Windows users should run SetupProof through WSL2.

The current v0.1 contract remains:

- Repository file identities in plans and reports use repository-relative slash
  paths.
- Git-provided workspace paths must resolve inside the repository root before
  copying.
- Absolute Git workspace paths, including native Windows drive-letter and UNC
  paths, are invalid as copied repository-relative paths.
- The runner builds a sanitized environment from an allowlist instead of
  inheriting the process environment wholesale.
- `HOME` and `TMPDIR` point inside the temporary workspace for local and
  `action-local` execution.
- `PATH` may be inherited for local and `action-local` so documented POSIX
  setup commands can find host tools.
- `env.allow` and `env.pass` names are explicit and are reported by name only.
- `env.pass` values marked `secret: true` are redacted from supported output
  sinks.
- Native Windows case-insensitive environment-name handling is not supported
  until a native Windows environment contract is accepted.

Future native Windows path and environment support requires a new ADR or an
accepted update to this ADR. That decision must define:

- whether report and plan paths remain slash-normalized or expose native
  separators;
- how drive-letter paths, UNC paths, and extended-length paths are displayed and
  validated;
- how relative paths are resolved from the invocation directory, repository
  root, temporary workspace, config file, and report destinations;
- how case-insensitive environment names are canonicalized and deduplicated;
- which baseline variables are set, renamed, or omitted on native Windows;
- whether `HOME`, `USERPROFILE`, `TMP`, `TEMP`, and `TMPDIR` are all set, and
  where they point;
- how redaction handles case-insensitive variable names without redacting
  unrelated values;
- how schema compatibility is preserved for existing plan and report consumers.

## Consequences

- Public docs continue to name WSL2 as the Windows path.
- Native Windows path and environment work can proceed behind explicit CI
  checks without changing user-facing support claims.
- Existing reports keep stable repository-relative path strings and named-only
  environment metadata.
- Native Windows packaging remains blocked until the path, environment,
  workspace, report, and shell contracts are all implemented and verified.
