# ADR 0008: Marketplace Deferral And Repository Strategy

Status: Accepted

Date: 2026-04-24

## Context

SetupProof should dogfood its own workflows and keep release automation in the
main repository. GitHub Marketplace repository-shape requirements may conflict
with that layout.

## Decision

Defer GitHub Marketplace listing for v0.1.

Repository strategy:

- Keep the product, CLI source, docs, workflows, and Action packaging in the
  main repository through v0.1.
- Do not advertise Marketplace availability until current Marketplace
  requirements are checked and the listing exists.
- Do not create a separate Action-only repository unless Marketplace rules make
  it necessary.
- If Marketplace still requires an action-only repository with no workflow
  files, choose between continued deferral and a separate Action repository in a
  later ADR.

## Consequences

- A usable Action may ship without making Marketplace a release blocker.
- Public docs must not imply Marketplace availability before it exists.
- The main repository can keep CI workflows needed for dogfooding and release
  checks.
