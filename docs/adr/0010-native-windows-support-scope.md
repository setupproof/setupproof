# ADR 0010: Native Windows Support Scope

Status: Accepted

Date: 2026-07-07

## Context

SetupProof v0.1 supports Linux as the primary local and CI target, uses
GitHub-hosted Ubuntu runners as the primary Action environment, treats macOS as
best effort, and directs Windows users to WSL2. The current execution model is
intentionally POSIX-shaped: marked `sh`, `bash`, and `shell` fences run as
non-interactive scripts with closed stdin, no TTY, strict-mode prologues, shared
per-file state, timeout handling, sanitized environment behavior, and temporary
workspace copies.

Native Windows support is not a packaging-only change. It affects shell
selection, command quoting, drive-letter and UNC paths, backslash separators,
case-insensitive environment variables, CRLF handling, symlink behavior,
temporary workspace copying, process-tree termination, and report
compatibility.

## Decision

Native Windows execution remains unsupported in v0.1. Windows users should run
SetupProof through WSL2 until native support is explicitly designed,
implemented, and tested.

The current v0.1 shell contract remains:

- `sh`, `bash`, and `shell` fences are supported.
- `shell` means `sh`.
- PowerShell and `cmd` fences are unsupported.
- Release archives and install docs advertise Linux and macOS binaries, not
  native Windows binaries.

Native Windows support must not be documented as available until all of these
conditions are met:

- Windows CI coverage runs the relevant CLI, runner, report, and docs tests on
  GitHub-hosted Windows runners.
- A path contract covers drive-letter paths, UNC paths, backslash normalization,
  relative paths, workspace roots, and report paths.
- An environment contract covers case-insensitive names, baseline variables,
  pass-through variables, required variables, and redaction.
- A shell contract decides whether PowerShell, `cmd`, or both are supported,
  and how strict mode, line continuations, working directory updates, exported
  state, stdin closure, no-TTY execution, timeouts, and process-tree cleanup
  behave.
- Workspace copy behavior covers Windows file modes, symlinks, ignored files,
  untracked files, linked worktrees, submodules, temporary directories, and
  cleanup.
- Report JSON and terminal output stay compatible with existing schemas and
  consumers, or any schema change is handled through the public schema process.
- Install and troubleshooting docs continue to name WSL2 as the Windows path
  until native Windows binaries and behavior are verified.

Implementation should land as focused follow-up issues instead of opportunistic
platform changes mixed into unrelated runner, packaging, or docs pull requests.

## Consequences

- Current Windows documentation remains intentionally narrow and points to WSL2.
- Platform proposals have a checklist for the semantics and CI coverage they
  must define before support can be advertised.
- Native Windows packaging is blocked on execution semantics and CI coverage,
  not only on adding another release archive target.
- PowerShell support is treated as an explicit shell-design decision, not as an
  automatic side effect of running on Windows.
