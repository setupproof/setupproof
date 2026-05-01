#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)
TMP_ROOT=$(mktemp -d "${TMPDIR:-/tmp}/setupproof-demo.XXXXXX")
PAUSE=${SETUPPROOF_DEMO_PAUSE:-1}

cleanup() {
  rm -rf "$TMP_ROOT"
}
trap cleanup EXIT INT TERM

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf 'missing required command: %s\n' "$1" >&2
    exit 2
  fi
}

pause() {
  if [ "$PAUSE" != "0" ]; then
    sleep "$PAUSE"
  fi
}

run_setupproof() {
  printf '\n$ setupproof'
  for arg in "$@"; do
    printf ' %s' "$arg"
  done
  printf '\n'
  pause
  "$BIN" "$@"
  pause
}

need git
need go

BIN="$TMP_ROOT/setupproof"
DEMO_REPO="$TMP_ROOT/project"

(cd "$ROOT" && go build -o "$BIN" ./cmd/setupproof)

mkdir -p "$DEMO_REPO"
cd "$DEMO_REPO"

cat > README.md <<'EOF'
# Demo Project

This unmarked setup block is a candidate a maintainer can review first:

```sh
go test ./...
```

This marked quickstart is the block SetupProof will run:

<!-- setupproof id=quickstart -->
```sh
printf 'install dependencies\n'
sleep 2
printf 'run tests\n'
test -f README.md
```
EOF

git init -q
git add README.md
git -c user.name='SetupProof Demo' -c user.email='demo@example.invalid' commit -m initial -q

printf 'SetupProof terminal demo\n'
printf 'project: temporary Git repository\n'
pause

run_setupproof suggest README.md
run_setupproof --list README.md
run_setupproof review README.md
run_setupproof --require-blocks --no-color --no-glyphs README.md

pause
printf '\nDemo complete.\n'
