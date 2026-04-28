# SetupProof

[![SetupProof](https://github.com/setupproof/setupproof/actions/workflows/setupproof.yml/badge.svg)](https://github.com/setupproof/setupproof/actions/workflows/setupproof.yml)

Verify marked README quickstarts in a clean temporary workspace.

SetupProof is for maintainers who want setup instructions to fail in CI before
users copy them. It executes only Markdown shell blocks that a maintainer
explicitly marks, then reports the block ID, file, line, runner, timeout, and
result.

````md
<!-- setupproof id=quickstart -->
```sh
go test ./...
```
````

Review first, then run:

```sh
go run ./cmd/setupproof review README.md
go run ./cmd/setupproof --require-blocks --no-color --no-glyphs README.md
```

SetupProof v0.1 runs from this source tree; packaged installs are not out yet.
The source-tree GitHub Action is usable by this repository or vendored copies
with a locally built CLI. Public package-manager installs and external Action
tag examples are deferred until release packaging exists.

## Try It Locally

From this source checkout, build the CLI:

```sh
go build -o ./setupproof ./cmd/setupproof
```

Then run the binary from any Git repository with Markdown setup docs:

```sh
/path/to/setupproof init
/path/to/setupproof review README.md
/path/to/setupproof --require-blocks --no-color --no-glyphs README.md
```

This is the v0.1 source-tree path. Package-manager installs, public external
Action tags, native Windows execution, and PowerShell fenced blocks are not
available yet.

## Adopt It

Create the default config without prompts:

```sh
go run ./cmd/setupproof init
```

That writes `setupproof.yml` with `README.md` as the only default target,
`defaults.requireBlocks: true`, the local runner, a 120 second timeout, and no
secret environment passthrough. It refuses to overwrite existing files unless
you pass `--force`.

Use non-executing inspection while adding markers:

```sh
go run ./cmd/setupproof --list README.md
go run ./cmd/setupproof review README.md
go run ./cmd/setupproof --dry-run --json --require-blocks README.md
```

Unmarked Markdown shell blocks never execute as SetupProof targets.

## Safety Model

- `suggest`, `review`, `doctor`, `--list`, and `--dry-run --json` do not
  execute commands.
- Execution uses a copied temporary workspace, not the live working directory.
- Local and Action runners are trusted-code runners.
- Docker improves isolation but is not a perfect sandbox.
- No telemetry. No update checks.
- Secrets pass only when configured.

## For Agents

Coding agents should treat SetupProof markers as the authoritative runnable
quickstart surface:

```sh
go run ./cmd/setupproof --list README.md
go run ./cmd/setupproof review README.md
go run ./cmd/setupproof --dry-run --json --require-blocks README.md
go run ./cmd/setupproof --require-blocks --no-color --no-glyphs README.md
```

Do not execute unmarked Markdown shell blocks as SetupProof targets. See
`docs/AGENT_USAGE.md` and `llms.txt` for the full agent contract.

## Demos And Docs

- `docs/demo/terminal-demo.sh` regenerates a short terminal demo from source.
- `docs/demo/terminal-demo.txt` is a checked transcript of the terminal demo.
- `docs/ARCHITECTURE.md` explains the package map and core invariants.
- `docs/AGENT_USAGE.md` defines the recommended workflow for coding agents.
- `docs/INSTALL.md` shows current CI and platform guidance without package
  distribution claims.
- `docs/RELEASE_READINESS.md` lists gates before publishing external
  distribution instructions.
- `schemas/` contains plan, report, and `setupproof.yml` JSON Schemas.
- `examples/` contains Node, Python, Docker Compose, monorepo, Go, and Rust
  fixtures.
- `SUPPORT.md` lists the information maintainers should include when reporting
  a setup-doc verification issue.

## License

SetupProof is licensed under the Apache License, Version 2.0 (`Apache-2.0`).
See `LICENSE` and `NOTICE`.

## Repository Checks

For normal repository validation:

```sh
make check
```

For release-oriented changes, also run:

```sh
make staticcheck
make vuln
make actionlint
```

## Repository Dogfood

The marked block below is the repository's SetupProof check.

<!-- setupproof id=repo-smoke -->
```sh
go test ./...
go vet ./...
bash scripts/check-github-action.sh
sh scripts/check-examples.sh
sh scripts/check-docs.sh
```
