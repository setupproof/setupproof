# ADR 0000: Runtime And npm Package Strategy

Status: Accepted

Date: 2026-04-24

## Context

SetupProof needs a local CLI that is easy to audit, has low startup latency for
non-executing commands, and can later be used by a GitHub Action. Future
package distribution should work in a clean temporary project without
postinstall scripts, install-time network calls, native build steps, telemetry,
or a large dependency surface.

## Decision

Use Go for the primary CLI runtime.

The app layout is:

- Go module at the repository root.
- CLI entrypoint at `cmd/setupproof`.
- Internal packages under `internal/`.
- Public JSON schemas under `schemas/` when report and plan schemas are added.
- Runner code lives under `internal/runner`.

The npm package is a distribution wrapper for the Go binary, not the primary
implementation. The npm package will contain:

- a small JavaScript `bin` wrapper with no production dependencies;
- prebuilt SetupProof binaries for supported npm platforms;
- a manifest that maps platform and architecture to the bundled binary;
- release checksums for audit and packaging checks.

The npm package must not use postinstall scripts, install-time downloads,
install-time native builds, hidden update checks, or telemetry. `npm pack
--dry-run` and a packed-tarball smoke test must run before publish. The smoke
test must prove that a clean temporary project can install the package and run
the CLI.

Initial binary targets are:

- Linux x64.
- Linux arm64.
- macOS x64.
- macOS arm64.

Native Windows execution is not supported in v0.1. Windows users should use
WSL2 until native support is explicitly added.

## Options Considered

- Node.js CLI plus JavaScript Action: easiest npm path, but larger dependency
  risk and less attractive for a small auditable command runner.
- Go CLI plus composite Action: small static binary, fast startup, simple
  cross-platform builds, and direct fit for checksum-verified Action downloads.
- Rust CLI plus composite Action: also suitable, but slower to implement for
  this project given maintainer familiarity.
- Go or Rust CLI plus npm binary wrapper: satisfies the first documented local
  install path while keeping the implementation in a compiled CLI.

## Consequences

- The Go module and `cmd/setupproof` entrypoint are the base implementation.
- No JavaScript application framework is needed for the CLI.
- npm packaging is deferred, but the runtime choice must keep the
  packed-tarball smoke test viable.
- Public docs must not advertise npm install commands until the package exists
  and the packed-tarball smoke test passes.
