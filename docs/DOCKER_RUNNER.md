# Docker Runner

Use the Docker runner when the documented setup path needs a pinned toolchain
image or when host-tooling drift makes local execution too noisy.

Docker is an isolation upgrade from running directly on the host. It is not a
security sandbox for hostile commands. Treat marked blocks from untrusted pull
requests the same way you treat the project test suite: run them without
repository secrets, prefer ephemeral hosted runners, and review the plan before
execution.

## How It Runs

For each target file, SetupProof copies the Git workspace to a temporary host
directory, starts a container, and bind-mounts that copied workspace at
`/workspace`.

The container starts with:

- read-only root filesystem;
- `tmpfs` at `/tmp`;
- dropped Linux capabilities;
- `no-new-privileges`;
- the host UID/GID when available;
- SetupProof-managed home, temp, and cache directories under
  `/workspace/.setupproof`.

Blocks in the same Markdown file share shell state and one container by default.
Set `isolated: true` for a block or default when each marked block should get a
fresh workspace and container.

One execution cannot mix Docker and non-Docker runners. Shared blocks in one
file must also use one Docker image; use `isolated: true` when different blocks
need different images.

## Image Choice

Pin CI images by digest once the setup path is known:

```yaml
version: 1

defaults:
  runner: docker
  image: ghcr.io/OWNER/IMAGE:TAG@sha256:...
  timeout: 5m

files:
  - README.md
```

Tags are fine for local exploration, but mutable tags can make README checks
change without a repository diff. The image must contain the shell and tools
used by the marked block.

Docker image pulls happen before the marked command starts, so account for pull
latency in the CI job timeout. The per-block timeout covers the command
execution after the container is running.

## Network

Docker is the only v0.1 runner that can enforce `network: false`:

```yaml
version: 1

defaults:
  runner: docker
  image: ghcr.io/OWNER/IMAGE:TAG@sha256:...
  network: false
```

With `network: false`, SetupProof starts the container with `--network none`.
Without it, Docker's default container networking applies.

## Environment

SetupProof does not copy the host environment wholesale. It sets a small
baseline environment and passes only variables listed in `setupproof.yml`.

```yaml
version: 1

env:
  pass:
    - name: REGISTRY_TOKEN
      secret: true
      required: true
```

`secret: true` redacts exact values from supported output sinks. Avoid printing
derived secrets, partial secrets, or private infrastructure names in marked
commands.

## When Not To Use It

Do not use the Docker runner when:

- the setup path needs privileged Docker, host networking, or the host Docker
  socket;
- the commands are interactive;
- the repository needs multiple long-running services instead of a README
  quickstart check;
- the goal is to safely execute hostile code.

Use a project-specific CI job for full environment orchestration. Use
SetupProof for the narrow promise that the documented first-run commands still
work.
