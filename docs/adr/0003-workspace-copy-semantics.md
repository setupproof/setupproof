# ADR 0003: Workspace Copy Semantics

Status: Accepted

Date: 2026-04-24

## Context

SetupProof promises a clean workspace, not execution in the maintainer's live
working directory. The copy mode needs to be explicit and auditable.

## Decision

For execution runners in v0.1, the default workspace source is a Git working
tree copy with tracked files as they exist in the working tree.

Default copy mode:

- Resolve the repository root before resolving target files or config.
- For Git working trees, copy tracked files from the working tree, including
  staged and unstaged tracked modifications.
- Exclude ignored files.
- Exclude untracked files.
- Exclude the `.git` directory.
- Report when the copied workspace differs from `HEAD`.
- Reject target, config, and copied symlink paths that resolve outside the
  repository root.

`--include-untracked`:

- Adds untracked, non-ignored files to the copy.
- Reports that untracked files were included.
- Is for local debugging and must not appear in generated CI examples.

Non-Git behavior:

- Non-executing commands that only inspect Markdown may use the invocation
  directory as the root when no Git working tree is present.
- Execution commands that require the default tracked-file copy mode must fail
  with exit code `2` when no Git working tree is available, unless a later ADR
  defines a separate non-Git copy mode.

Temporary workspace behavior:

- One target file gets one independent temporary workspace by default.
- Workspace paths are omitted from stable JSON unless explicitly marked
  volatile.
- Workspaces are removed after success, failure, timeout, and interrupt.
- Docker runner containers use the host UID/GID for execution when available so
  files created inside the copied workspace remain removable by the host user.
- `--keep-workspace` preserves the workspace, prints the path, and warns that
  it may contain command output or generated files.

## Consequences

- The local runner copy must use Git-aware file selection.
- The implementation must not describe the workspace as a clean checkout.
- Reports should use `tracked-plus-modified` as the default workspace source
  name.
- A future non-Git execution mode needs its own explicit copy-mode decision.
