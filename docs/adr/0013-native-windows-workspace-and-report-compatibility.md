# ADR 0013: Native Windows Workspace And Report Compatibility

Status: Accepted

Date: 2026-07-07

## Context

ADR 0010 keeps native Windows execution outside v0.1 scope. ADR 0011 keeps
native Windows shell execution unsupported, and ADR 0012 keeps native Windows
path and environment behavior unsupported. The remaining compatibility work is
to verify that workspace copying and report output have a conservative,
schema-compatible boundary before SetupProof can ever advertise native Windows
support.

Native Windows differs from the current POSIX-shaped runner in file modes,
symlink privileges, temporary-directory behavior, submodule checkout shape,
linked worktree metadata, path separators, and report consumers that expect
repository-relative path strings.

## Decision

Native Windows workspace execution remains unsupported in v0.1. Windows users
should run SetupProof through WSL2.

The current v0.1 compatibility contract remains:

- Workspace-copy helpers are tested on native Windows without executing marked
  shell blocks.
- Tracked files are copied from the Git working tree, including staged and
  unstaged tracked modifications.
- Ignored files and untracked files are omitted by default.
- `--include-untracked` copies untracked non-ignored files and reports the
  count.
- Temporary workspaces include workspace-scoped home and tmp directories.
- Cleanup makes copied trees writable before removal so read-only files do not
  leave temporary directories behind.
- Symlinks are copied only when the host permits symlink creation, and copied
  symlink targets must resolve inside the repository root.
- Linked worktrees are treated as their own Git workspace root.
- Git submodule Gitlink entries are omitted from copied workspaces and reported
  as omitted submodule paths.
- Report JSON and terminal output keep repository file identities
  slash-normalized, including on native Windows.
- Native Windows execution reports stop at `unsupported_platform` and remain
  valid against the published report schema.

Future native Windows workspace support requires a new ADR or an accepted
update to this ADR. That decision must define:

- whether submodule working tree contents are copied, materialized, or remain
  explicitly omitted;
- how Windows junctions, reparse points, and unavailable symlink privileges are
  represented;
- how file modes and executable bits are reported when native Windows cannot
  preserve POSIX permissions;
- where temporary workspaces are created for local and Action runners;
- how cleanup reports partial removal failures;
- whether report paths continue to use slash-normalized repository-relative
  paths for all consumers.

## Consequences

- Public docs continue to name WSL2 as the Windows path.
- Native Windows CI can verify workspace and report compatibility without
  implying native execution support.
- Existing plan and report schemas do not change for this compatibility work.
- Submodule-dependent setup commands need a future explicit copy policy before
  native Windows support can be advertised.
