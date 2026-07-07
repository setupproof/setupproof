# ADR 0011: Native Windows Shell Semantics

Status: Accepted

Date: 2026-07-07

## Context

ADR 0010 keeps native Windows execution outside v0.1 scope. The current
execution model is POSIX-shaped and supports only `sh`, `bash`, and `shell`
fences. Native Windows support needs a separate shell decision before users can
reason about command syntax, strict-mode behavior, state persistence, timeout
cleanup, and report output.

PowerShell and `cmd` differ materially from POSIX shells. They use different
quoting rules, line continuations, environment access syntax, exit-status
conventions, path parsing, and command-discovery behavior. Treating them as an
implicit variation of `shell` would make review output and CI behavior
ambiguous.

## Decision

Native Windows shell execution is not supported in v0.1.

The v0.1 shell contract remains:

- `sh` fences run with the `sh` shell.
- `bash` fences run with the `bash` shell.
- `shell` fences are an alias for `sh`.
- `powershell`, `pwsh`, and `cmd` fences are unsupported, including when they
  are explicitly marked for SetupProof.
- Unsupported marked shell languages fail during planning and do not produce an
  executable block.
- The local and Action runners must not reinterpret `shell` as PowerShell or
  `cmd` on Windows.

Future native Windows shell support requires a new ADR or an accepted update to
this ADR. That decision must define:

- the accepted fence language names and any aliases;
- strict-mode prologues for each supported shell;
- line-continuation handling and interactive-command detection;
- stdin closure and no-TTY behavior;
- current-working-directory state between blocks;
- exported or persisted environment state between blocks;
- timeout and process-tree cleanup behavior;
- command quoting and source-vs-prologue reporting boundaries;
- JSON report values for `language`, `shell`, `stdin`, `tty`, `stateMode`,
  `reason`, and timeout cleanup details.

## Consequences

- Current Windows users continue to run SetupProof through WSL2.
- Review, dry-run, terminal, Markdown, and JSON outputs keep one unambiguous
  meaning for `shell`: POSIX `sh`.
- PowerShell and `cmd` work can be split into focused implementation issues
  after the path, environment, workspace, and report contracts are ready.
- Windows CI should keep checking that native Windows shell fences fail clearly
  until native shell support is intentionally added.
