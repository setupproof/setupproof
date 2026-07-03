# ADR 0008: Marketplace Listing Strategy

Status: Accepted

Date: 2026-04-24
Updated: 2026-07-03

## Context

SetupProof should dogfood its own workflows and keep release automation in the
main repository. Earlier v0.1 planning deferred Marketplace listing until the
current repository-shape requirements were checked against that layout.

## Decision

List the existing public repository on GitHub Marketplace from the next release
that includes current Action metadata.

Repository strategy:

- Keep the product, CLI source, docs, workflows, and Action packaging in the
  main repository.
- Use the single root `action.yml` as the Marketplace action metadata file.
- Do not advertise Marketplace availability until the listing exists.
- Do not create a separate Action-only repository unless GitHub rejects the
  main repository listing.

## Consequences

- A usable Action may ship before Marketplace listing is complete.
- Public docs must not imply Marketplace availability before it exists.
- The main repository can keep CI workflows needed for dogfooding and release
  checks.
- If GitHub blocks listing because of repository shape or account requirements,
  record the blocker here before changing repository strategy.
