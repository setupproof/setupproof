# Troubleshooting

Use non-executing commands first when a setup-doc check is confusing:

```sh
setupproof review README.md
setupproof doctor README.md
setupproof --dry-run --json --require-blocks README.md
```

Then run the marked blocks once the plan matches the setup path you expect:

```sh
setupproof --require-blocks --no-color --no-glyphs README.md
```

## No Marked Blocks Found

`No marked blocks found.` means SetupProof did not find an explicitly marked
shell fence in the selected files.

Check that the command is reading the file that contains the setup commands. If
you use `setupproof.yml`, run without positional files to use the configured
targets, or pass every Markdown file explicitly.

Use `--require-blocks` in CI so a renamed file, removed marker, or typo in a
marker fails loudly instead of producing an empty run.

## Command Failed

A line such as this is an ordinary setup command failure:

```text
[failed] README.md#quickstart file=README.md:18 runner=local timeout=120s result=failed exit=1 reason=exit-code
```

Run `setupproof review README.md` to confirm the exact source command, shell,
runner, and timeout. Then run the command manually from a clean checkout if the
failure needs project-specific debugging.

## Timeout

Timeout failures report `result=timeout` and keep the source file and block ID
in the output. Increase the timeout only for setup steps that are expected to be
slow:

````md
<!-- setupproof id=quickstart timeout=5m -->
```sh
go test ./...
```
````

Prefer fixing unexpectedly slow setup commands before raising the timeout in CI.

## Interactive Command

SetupProof closes stdin and does not allocate a TTY. Commands such as `read`,
interactive package prompts, or login flows fail before they can hang the run.

Move interactive steps outside the marked block, or replace them with
non-interactive flags that are safe for CI.

## Missing Shell Or Unsupported Platform

Marked shell fences need a supported shell. In v0.1, use `sh`, `bash`, or
`shell` fences.

Native Windows execution and PowerShell fences are unsupported in v0.1. On
Windows, run SetupProof through WSL2.

## Docker Runner Problems

Run `setupproof doctor README.md` when a Docker-backed block fails before the
project command starts.

Common causes:

- Docker is not installed or the daemon is not reachable.
- The configured image cannot be pulled.
- The first image pull is slow enough to hit a short timeout.
- The image tag is mutable or the digest is missing.

Prefer digest-pinned images for CI. Docker improves isolation from host tooling
drift, but it is not a security sandbox for hostile commands.

## Missing Environment Variables

Required variables configured in `setupproof.yml` fail before execution when
they are missing:

```yaml
version: 1

env:
  pass:
    - name: REGISTRY_TOKEN
      secret: true
      required: true
```

Run `setupproof doctor README.md` to check required variables without executing
marked blocks. Do not paste secret values into public issues, pull requests, or
logs.

## What To Include In A Support Issue

Include:

- `setupproof --version`
- operating system and architecture
- runner (`local`, `action-local`, or `docker`)
- the command you ran
- the relevant file path and block ID
- `setupproof review README.md` output for the block
- a redacted Markdown or JSON report when it is safe to share

Do not include secrets, private hostnames, or full unredacted command logs in
public issues.
