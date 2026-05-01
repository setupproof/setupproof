# ADR 0002: Shell Execution Semantics

Status: Accepted

Date: 2026-04-24

## Context

SetupProof executes only marked shell blocks. The execution model must be
predictable enough to review before running, but close enough to normal setup
docs to catch real quickstart drift.

## Decision

Each marked block runs as a non-interactive shell script.

Default shell behavior:

- `sh`, `bash`, and `shell` fences are eligible in v0.1.
- `shell` is treated as `sh`.
- `strict: true` is the default.
- Strict `sh` prepends `set -e`.
- Strict `bash` prepends `set -e -o pipefail`.
- `strict: false` may be set per block.
- Stdin is `/dev/null`.
- No TTY is allocated.

State behavior:

- One target Markdown file creates one execution group.
- Marked blocks in a file run sequentially in document order.
- Filesystem changes persist across blocks in the same target file.
- On successful block exit, the next block receives the block's final current
  working directory and exported environment.
- Failed, timed-out, skipped, or process-start-failed blocks do not update the
  next block's current working directory or exported environment.
- Shell functions, aliases, traps, and non-exported variables do not persist
  across blocks.
- Different target files never share shell state, filesystem state, or caches
  by default.
- `isolated: true` gives a block a fresh workspace and fresh shell state.

Failure behavior:

- Continue after block failures by default.
- `--fail-fast` stops after the first failing block in that file.
- Interactive-command detection produces a quickstart failure, not a runner
  infrastructure error.
- Interactive-command detection joins shell line continuations before matching,
  but dynamic forms such as `eval` are still not treated as statically
  classifiable.
- Timeouts kill the process tree where the platform supports it.
- State from timed-out blocks is not applied to later blocks.

Dry runs, reviews, terminal reports, Markdown reports, and JSON reports must
show shell, strict mode, stdin mode, TTY mode, state mode, timeout, and runner.

## Consequences

- Planning code may parse and report state metadata before execution code uses
  it.
- Runner implementation must include fixtures for shared cwd, exported
  variables, failed-block state handling, stdin closure, no TTY, and timeout.
- Reports must keep original source command text separate from any strict-mode
  prologue used to execute the block.
