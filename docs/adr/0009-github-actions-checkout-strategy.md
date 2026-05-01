# ADR 0009: GitHub Actions Checkout Strategy

Status: Accepted

Date: 2026-04-26

## Context

SetupProof v0.1 documents a source-tree GitHub Actions workflow before
immutable Action tags and release archives exist. That workflow must run in
this repository, or in a repository that vendors the SetupProof Action files at
its root, without adding secrets or default write permissions.

The workflow needs a repository checkout before it can build
`./cmd/setupproof` and invoke the local composite Action with `uses: ./`.

## Decision

Use an inline `git init`/`git fetch` checkout in the source-tree workflow until
external Action packaging exists.

The source-tree workflow must:

- run on `pull_request` and default-branch `push`;
- set `permissions: contents: read`;
- avoid repository secrets and token-specific inputs;
- fetch only the triggering commit;
- build the CLI from `./cmd/setupproof`;
- invoke the local Action with `uses: ./` and an explicit `cli-path`.

`setupproof init --workflow` refuses to write this workflow unless the
repository root contains the source-tree files the workflow needs.

## Consequences

- The workflow does not depend on a third-party Action tag before SetupProof
  has its own release packaging.
- The checkout intentionally omits LFS and submodule handling. Repositories
  that need those features should wait for released Action documentation or
  adapt the source-tree workflow knowingly.
- When immutable SetupProof Action tags are published, public docs can switch
  to the standard checkout guidance appropriate for that release channel.

v0.1.0 public docs now use the released Action path. The source-tree workflow
remains available for SetupProof itself and repositories that vendor the Action
files intentionally.
