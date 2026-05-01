# Install And CI

SetupProof v0.1.0 ships as a Go module, GitHub release archives, and a
versioned composite GitHub Action. npm, Homebrew, winget, Chocolatey, and Scoop
packages are not published yet.

## Go Install

Prerequisites: Go 1.22 or newer, Git, and a POSIX shell.

```sh
go install github.com/setupproof/setupproof/cmd/setupproof@v0.1.0
setupproof --version
setupproof review README.md
setupproof --require-blocks --no-color --no-glyphs README.md
```

## Release Archives

The GitHub release publishes Linux and macOS archives for `amd64` and `arm64`.
Each archive contains the `setupproof` binary plus license files. Verify the
archive with the matching checksum manifest before running it.

Archive names:

- `setupproof_0.1.0_linux_amd64.tar.gz`
- `setupproof_0.1.0_linux_arm64.tar.gz`
- `setupproof_0.1.0_darwin_amd64.tar.gz`
- `setupproof_0.1.0_darwin_arm64.tar.gz`
- `setupproof_0.1.0_checksums.txt`

## Source Checkout

From this repository:

```sh
make build
./setupproof init --check
./setupproof review README.md
./setupproof --require-blocks --no-color --no-glyphs README.md
```

Use `make build VERSION=<tag>` when building a release binary from a tagged
checkout. Use `make release-archives VERSION=0.1.0` to build the release
archive set locally.

For a new project, `setupproof init` writes only `setupproof.yml` by default.
Use `setupproof init --workflow` only from this source tree, or from a
repository root that vendors the SetupProof Action files, when you want the
source-tree workflow written too. Both writes refuse to overwrite existing
files unless `--force` is passed.

`setupproof.yml` intentionally supports a small YAML subset in v0.1: two-space
indentation, no tabs, string/number/boolean scalars, lists, and `x-` extension
keys. The parser rejects unsupported YAML forms with precise errors instead of
silently guessing.

## GitHub Actions

Use the standard `pull_request` event for pull request checks, and do not pass
repository secrets to the job by default. The run step executes marked commands
from the pull request, so treat it like running the project test suite against
untrusted code.

Pin the Action tag and the downloaded CLI version together.

<!-- ci-snippet:github-actions -->
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
      - uses: setupproof/setupproof@v0.1.0
        with:
          cli-version: v0.1.0
          mode: review
          require-blocks: "true"
          files: README.md
      - uses: setupproof/setupproof@v0.1.0
        with:
          cli-version: v0.1.0
          require-blocks: "true"
          files: README.md
```

## GitLab CI

Use an executor image or runner environment that already provides
`setupproof` on `PATH`. This snippet runs the CLI directly; it does not invoke
the GitHub Action.

<!-- ci-snippet:gitlab-ci -->
```yaml
stages:
  - test

setupproof:
  stage: test
  script:
    - setupproof review README.md
    - setupproof --report-json setupproof-report.json --require-blocks --no-color --no-glyphs README.md
  artifacts:
    when: always
    paths:
      - setupproof-report.json
```

## CircleCI

Use an executor image that already provides `setupproof` on `PATH`. This
snippet runs the CLI directly; it does not invoke the GitHub Action.

<!-- ci-snippet:circleci -->
```yaml
version: 2.1

jobs:
  setupproof:
    docker:
      - image: your-ci-image-with-setupproof
    steps:
      - checkout
      - run:
          name: Review marked quickstarts
          command: setupproof review README.md
      - run:
          name: Run marked quickstarts
          command: setupproof --report-json setupproof-report.json --require-blocks --no-color --no-glyphs README.md
      - store_artifacts:
          path: setupproof-report.json

workflows:
  setupproof:
    jobs:
      - setupproof
```

## Generic Shell CI

Use this shape for CI systems that run shell scripts directly. The job
environment must already provide `setupproof` on `PATH`.

<!-- ci-snippet:generic-shell -->
```sh
#!/bin/sh
set -eu

setupproof review README.md
setupproof --report-json setupproof-report.json --require-blocks --no-color --no-glyphs README.md
```

## Platform Support

v0.1 platform support:

- Linux and Ubuntu are the primary local and CI targets.
- GitHub-hosted Ubuntu runners are the primary GitHub Actions environment.
- macOS is best effort. Shell behavior can differ from Linux, so run
  `setupproof doctor README.md` before relying on macOS-only CI.
- Docker runner support requires Docker and is an isolation upgrade, not a
  security sandbox for hostile commands.
- Git submodules and linked worktrees are treated as their own repository
  boundary when their `.git` metadata is a file. `setupproof doctor` reports
  that layout so path resolution surprises are visible before execution.
- Native Windows execution is unsupported in v0.1. Windows users should run
  SetupProof through WSL2.
- PowerShell fenced blocks are unsupported in v0.1. Use `sh`, `bash`, or
  `shell` fenced blocks for marked quickstarts.

For CI on untrusted pull requests, pass no secrets by default, prefer hosted
ephemeral runners, and run `setupproof review README.md` before executing marked
blocks.

## Distribution Status

Published for v0.1.0:

- Go install from the public module path.
- Linux and macOS release archives with checksums.
- Versioned GitHub Action usage with `cli-version: v0.1.0`.

Deferred until implemented and verified:

- npm package distribution.
- Homebrew, winget, Chocolatey, and Scoop packages.
- GitHub Marketplace listing.
