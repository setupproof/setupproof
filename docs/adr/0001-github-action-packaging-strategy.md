# ADR 0001: GitHub Action Packaging Strategy

Status: Accepted

Date: 2026-04-24

## Context

SetupProof needs a GitHub Action integration after the CLI exists. The product
spec requires the Action to run the exact CLI version pinned by the Action
release, verify checksums before execution, map inputs explicitly, write a
bounded step summary, avoid repository secrets by default, and fail the Action
when SetupProof exits non-zero.

## Decision

Use a composite GitHub Action that invokes the Go CLI.

The Action lives in the main product repository. It should not be advertised
until `action.yml` exists and an end-to-end Action smoke test passes.

Release packaging will publish platform archives and a checksum manifest for
each immutable release tag. Until those archives exist, source-tree workflows
must pass `cli-path` to a locally built CLI and the `cli-version` default stays
empty. The composite Action will:

- resolve the CLI version from an explicit `cli-version` input when binary
  download mode is used;
- download the matching platform archive for that exact version;
- verify the archive checksum from the release manifest before extracting or
  executing it;
- optionally enforce a caller-provided `cli-sha256` archive digest when users
  want the workflow to pin the archive independently of the release manifest;
- pass explicit CLI flags derived from Action inputs;
- expose the report file path as an output;
- write a bounded step summary;
- exit with the same failure semantics as the CLI.

Generated workflow examples must use `pull_request`, `push` for the default
branch, `permissions: contents: read`, no repository secrets, and
`--require-blocks`.

## Consequences

- A JavaScript Action bundle is not needed.
- If a released Action enables a default `cli-version`, the release process
  must check that the Action default, release tag, archive names, and checksums
  agree.
- Moving major Action tags must not be documented until tag movement policy and
  release automation exist.
- Marketplace listing is handled separately by ADR 0008.
