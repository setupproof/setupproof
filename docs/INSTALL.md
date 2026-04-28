# Install And CI

SetupProof v0.1 runs from this source tree. Packaged installs and external
Action tag examples will be documented after release packaging exists.

The CI examples below assume a `setupproof` executable is already available on
`PATH`, except for the GitHub Actions source-tree example, which builds the CLI
from this repository and passes it to the local Action with `cli-path`.

## Source Tree

Prerequisites: Go, Git, and a POSIX shell. From this repository or a vendored
copy:

```sh
make build
./setupproof init --check
./setupproof review README.md
./setupproof --require-blocks --no-color --no-glyphs README.md
```

Use `make build VERSION=<tag>` when building a release binary from a tagged
checkout.

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

This source-tree snippet is for this repository or a repository that has
vendored the SetupProof Action files at its root. Public external Action tags
will be documented only after immutable release packaging exists.

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
      # Source-tree workflow: see docs/adr/0009-github-actions-checkout-strategy.md.
      - name: Checkout repository
        shell: bash
        run: |
          git init .
          git remote add origin "$GITHUB_SERVER_URL/$GITHUB_REPOSITORY"
          git fetch --depth=1 origin "$GITHUB_SHA"
          git checkout --detach FETCH_HEAD
      - name: Build SetupProof CLI
        shell: bash
        run: go build -o "$RUNNER_TEMP/setupproof" ./cmd/setupproof
      - name: Review marked quickstarts
        uses: ./
        with:
          cli-path: ${{ runner.temp }}/setupproof
          mode: review
          require-blocks: "true"
          files: README.md
      - name: Run marked quickstarts
        uses: ./
        with:
          cli-path: ${{ runner.temp }}/setupproof
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

Deferred until implemented and verified:

- npm package distribution.
- Homebrew, winget, Chocolatey, and Scoop packages.
- Stable external GitHub Action tags and Marketplace listing.
