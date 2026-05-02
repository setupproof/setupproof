# Configuration Recipes

Copyable `setupproof.yml` starters for common repository layouts. Each recipe
sticks to fields documented in `schemas/setupproof-config.schema.json` and
explains the tradeoff in one or two sentences.

If your repository does not need a `setupproof.yml` at all, run
`setupproof review README.md` and `setupproof --require-blocks --no-color
--no-glyphs README.md` directly. SetupProof falls back to repository-root
`README.md` when no config and no command-line files are passed.

## One README At The Repository Root

```yaml
version: 1

files:
  - README.md
```

This is the smallest viable config and the right starting point for a
single-package repository where the marked blocks live next to the install
instructions. The tradeoff is that any future docs split (`docs/cli.md`,
`docs/api.md`) needs a config update before SetupProof will inspect those
files on a no-argument run.

## Docs Split Across README And `docs/*.md`

```yaml
version: 1

defaults:
  requireBlocks: true

files:
  - README.md
  - docs/install.md
  - docs/quickstart.md
```

Use this when setup instructions live both in the top-level README and in
focused docs pages. Listing each file explicitly keeps SetupProof's no-argument
behavior predictable - the tool inspects exactly the listed files in order.
`requireBlocks: true` makes a typo in a marker fail loudly instead of silently
producing an empty plan.

## Examples That Should Be Reviewed But Not Run

```yaml
version: 1

defaults:
  requireBlocks: false

files:
  - README.md
  - docs/recipes.md
```

Set `requireBlocks: false` for repositories whose docs pages mostly contain
prose plus illustrative commands that should not be executed unattended. With
this recipe, run `setupproof review` (or in CI, `setupproof
--list`) to inspect what SetupProof would discover without actually running
any block. This keeps the v0.1 review path useful for archives and copy-this-
verbatim guides, at the cost of losing the safety net that `requireBlocks:
true` provides for active install docs.

## Projects That Require Docker For Setup Verification

```yaml
version: 1

defaults:
  runner: docker
  image: ghcr.io/OWNER/IMAGE:TAG
  timeout: 5m

files:
  - README.md

env:
  pass:
    - name: DOCKER_HOST
    - name: REGISTRY_TOKEN
      secret: true
      required: true
```

Use the Docker runner when the documented setup needs a pinned base image or
when host-tooling drift makes local execution unreliable. Pin the image by
digest (`ghcr.io/OWNER/IMAGE:TAG@sha256:...`) once you have a known-
good build; the schema accepts any non-whitespace string. `env.pass` with
`secret: true` redacts the value from supported output sinks, and `required:
true` fails before execution rather than mid-run when the variable is missing.
The tradeoff is that the Docker runner adds image-pull latency on the first
run and requires Docker on the host or runner.
