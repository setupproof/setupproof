# SetupProof

[![SetupProof](https://github.com/setupproof/setupproof/actions/workflows/setupproof.yml/badge.svg)](https://github.com/setupproof/setupproof/actions/workflows/setupproof.yml)

Run the README setup path from a clean checkout before contributors copy it.

Normal CI proves code; it rarely proves the commands someone runs in the first
five minutes. SetupProof lets maintainers mark the setup block they want to
stand behind, then run that exact block locally or in CI from a copied
workspace. If it breaks, the report points back to the README line that needs
fixing.

- Mark only the trusted shell block.
- Review the plan without executing commands.
- Run the block locally or in GitHub Actions, with no telemetry and no secret
  environment passthrough unless configured.

![Terminal demo showing SetupProof reviewing and running a marked README quickstart](docs/demo/setupproof.gif)

## What It Catches

README quickstarts usually fail in small, ordinary ways: a package moves, a test
command changes, or a prerequisite stops being true. SetupProof turns the
marked block into the thing CI verifies.

````md
<!-- setupproof id=quickstart -->
```sh
npm ci
npm test
```
````

If that setup path breaks, the failure points back to the README block people
would have copied:

```text
! SetupProof failed  742ms
  1 block, 1 file

! README.md#quickstart failed  742ms
  README.md:18 runner=local timeout=120s result=failed exit=1 reason=exit-code
  next: setupproof review README.md
```

The goal is not to lint Markdown. The goal is to keep the public setup path
runnable from a fresh clone.

Good first targets are the commands a new contributor runs before they know the
project: install dependencies, run the default tests, start the local service,
or enter the package that owns the example. Those paths change quietly, and they
are exactly the failures SetupProof makes visible.

## Install

With Homebrew:

```sh
brew install setupproof/tap/setupproof
setupproof --version
```

With Go 1.22 or newer:

```sh
go install github.com/setupproof/setupproof/cmd/setupproof@v0.1.3
setupproof --version
```

From a source checkout:

```sh
make build
./setupproof --version
```

## Use It

Mark a README shell block that should stay runnable:

````md
<!-- setupproof id=quickstart -->
```sh
go test ./...
```
````

Review the plan without executing commands:

```sh
setupproof review README.md
```

Run the marked blocks:

```sh
setupproof README.md
```

Typical local output is compact and scan-friendly:

```text
+ SetupProof passed  35ms
  1 block, 1 file

+ README.md#quickstart passed  35ms
  README.md:12 runner=local timeout=120s result=passed
```

Interactive terminals wait briefly, then show a styled live line with the current phase, block count, and elapsed time.

For deterministic CI logs, keep the plain format:

```sh
setupproof --require-blocks --no-color --no-glyphs README.md
```

```text
[passed] README.md#quickstart file=README.md:12 runner=local timeout=120s result=passed
```

Unmarked shell examples stay inert.

## Adopt It

Create the default config:

```sh
setupproof init
```

That writes `setupproof.yml` with `README.md` as the default target, the local
runner, a 120 second timeout, `defaults.requireBlocks: true`, and no secret
environment passthrough. Existing files are not overwritten unless `--force` is
passed.

Create the config and a pinned GitHub Actions workflow together:

```sh
setupproof init --workflow
```

Use non-executing inspection while adding markers:

```sh
setupproof suggest README.md
setupproof --list README.md
setupproof review README.md
setupproof --dry-run --json --require-blocks README.md
```

## GitHub Actions

Pin both the Action and CLI version:

```yaml
name: SetupProof

on:
  pull_request:
  push:
    branches:
      - main

permissions:
  contents: read

jobs:
  readme-quickstart:
    runs-on: ubuntu-24.04
    timeout-minutes: 10
    steps:
      - uses: actions/checkout@v4
      - uses: setupproof/setupproof@v0.1.3
        with:
          cli-version: v0.1.3
          mode: review
          require-blocks: "true"
          files: README.md
      - uses: setupproof/setupproof@v0.1.3
        with:
          cli-version: v0.1.3
          require-blocks: "true"
          files: README.md
```

## Safety Model

- `suggest`, `review`, `doctor`, `--list`, and `--dry-run --json` do not
  execute commands.
- Execution uses a copied temporary workspace, not the live working directory.
- Local and Action runners are trusted-code runners.
- Docker improves isolation but is not a security sandbox.
- No telemetry. No update checks.
- Secrets pass only when configured.

## Demos And Docs

- `docs/demo/setupproof.gif` shows the short terminal demo used above.
- `docs/demo/setupproof.tape` regenerates the GIF with VHS.
- `docs/demo/terminal-demo.sh` regenerates a short terminal demo from source.
- `docs/demo/terminal-demo.txt` is a checked transcript of the terminal demo.
- `docs/ARCHITECTURE.md` explains the package map and core invariants.
- `docs/DOCKER_RUNNER.md` documents Docker runner tradeoffs and trust boundaries.
- `docs/INSTALL.md` covers release archives, GitHub Actions, and CI snippets.
- `docs/RECIPES.md` collects copyable `setupproof.yml` starters for common repository layouts.
- `docs/RELEASE_READINESS.md` lists release checks.
- `docs/TROUBLESHOOTING.md` maps common failure output to the next command to
  run.
- `schemas/` contains plan, report, and `setupproof.yml` JSON Schemas.
- `examples/` contains Node, Python, Docker Compose, monorepo, Go, and Rust
  fixtures.
- `SUPPORT.md` lists the information maintainers should include when reporting
  a setup-doc verification issue.

## Repository Checks

```sh
make check
```

For release-oriented changes, also run:

```sh
make staticcheck
make vuln
make actionlint
make release-check
```

## License

SetupProof is licensed under the Apache License, Version 2.0 (`Apache-2.0`).
See `LICENSE` and `NOTICE`.

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
