# ADR 0009: GitHub Actions Checkout Strategy

Status: Accepted

Date: 2026-04-26

Updated: 2026-06-26

## Context

SetupProof v0.1 documents a source-tree GitHub Actions workflow before
immutable Action tags and release archives exist. That workflow must run in
this repository, or in a repository that vendors the SetupProof Action files at
its root, without adding secrets or default write permissions.

The workflow needs a repository checkout before it can build
`./cmd/setupproof` and invoke the local composite Action with `uses: ./`.

As of v0.1.1, release archives and immutable Action tags exist. Normal
repositories should use the released Action workflow instead of the source-tree
workflow.

## Decision

Use an inline `git init`/`git fetch` checkout in the repository-maintained
source-tree workflow.

The source-tree workflow must:

- run on `pull_request` and default-branch `push`;
- set `permissions: contents: read`;
- avoid repository secrets and token-specific inputs;
- fetch only the triggering commit;
- build the CLI from `./cmd/setupproof`;
- invoke the local Action with `uses: ./` and an explicit `cli-path`.

`setupproof init --workflow` generates the released Action workflow for normal
repositories. It uses `actions/checkout@v4`, pins `setupproof/setupproof` and
`cli-version` to the current release, and does not require source-tree Action
files in the target repository.

## Consequences

- The workflow does not depend on a third-party Action tag before SetupProof
  has its own release packaging.
- The checkout intentionally omits LFS and submodule handling. Repositories
  that need those features should adapt the source-tree workflow knowingly.
- Public docs and generated workflows use the released Action path.
- The source-tree workflow remains available for SetupProof itself and
  repositories that vendor the Action files intentionally.
