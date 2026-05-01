# Docker Compose Example

This example assumes Docker Compose is installed. It uses
`busybox:1.36.1@sha256:73aaf090f3d85aa34ee199857f03fa3a95c8ede2ffd4cc2cdb5b94e566b11662`
so the example cannot drift if the tag moves. If that image is not already
local, the explicit `docker compose pull app` command uses the network to fetch
it. No secrets are required.

If Docker is exposed through a non-default socket, such as Colima on macOS, set
`DOCKER_HOST` before running SetupProof. This example's `setupproof.yml` allows
that variable through to the runner.

<!-- setupproof id=docker-compose-smoke timeout=180s -->
```sh
compose() {
  if docker compose version >/dev/null 2>&1; then
    docker compose "$@"
  else
    docker-compose "$@"
  fi
}

compose -f compose.yaml config
compose -f compose.yaml pull app
trap 'compose -f compose.yaml down --volumes' EXIT
compose -f compose.yaml up --abort-on-container-exit --exit-code-from app app
```
