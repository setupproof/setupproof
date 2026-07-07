# Configuration Recipes

Use a config when no-argument runs should inspect more than the root README, or
when a setup path needs runner, timeout, or environment defaults. Each recipe
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

## Monorepo With Package-Local Quickstarts

```yaml
version: 1

defaults:
  requireBlocks: true

files:
  - docs/web.md
  - docs/api.md
```

Use this when the repository root owns CI, but the setup paths live next to
package or service docs. Keep the file list explicit so each package-local
README or docs page is reviewed in a stable order. In a layout like
`examples/monorepo`, `docs/web.md` can mark a block that runs
`npm --prefix packages/web test`, while `docs/api.md` can mark a block that
runs `go test ./services/api/...`.

To generate both the config and a pinned GitHub Actions workflow for those
targets:

```sh
setupproof init --workflow docs/web.md docs/api.md
```

The generated workflow maps the same target list into the Action input:

```yaml
- uses: setupproof/setupproof@v0.1.3
  with:
    cli-version: v0.1.3
    mode: review
    require-blocks: "true"
    files: |
      docs/web.md
      docs/api.md
- uses: setupproof/setupproof@v0.1.3
  with:
    cli-version: v0.1.3
    require-blocks: "true"
    files: |
      docs/web.md
      docs/api.md
```

The tradeoff is maintenance: every package docs split needs a matching
`files:` update. That explicit list is still safer than globbing broad docs
trees, because unmarked examples stay inert and newly added setup docs require
an intentional CI decision.

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
this recipe, run `setupproof review` (or `setupproof --list` in CI) to inspect
what SetupProof would discover without executing any block. This keeps the
v0.1 review path useful for archives and copy-this-verbatim guides, at the cost
of losing the safety net that `requireBlocks: true` provides for active install
docs.

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
run and requires Docker on the host or runner. See `docs/DOCKER_RUNNER.md` for
the full trust boundary and runtime model.
