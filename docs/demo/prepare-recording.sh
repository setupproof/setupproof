#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)

export PS1='$ '

TMP_ROOT=$(mktemp -d "${TMPDIR:-/tmp}/setupproof-recording.XXXXXX")
BIN="$TMP_ROOT/setupproof"
DEMO_REPO="$TMP_ROOT/project"

cleanup_demo() {
  rm -rf "$TMP_ROOT"
}
trap cleanup_demo EXIT INT TERM

(cd "$ROOT" && go build -o "$BIN" ./cmd/setupproof)

mkdir -p "$DEMO_REPO"
cd "$DEMO_REPO"

cat > README.md <<'EOF'
# Demo service

This is the quickstart people copy:

<!-- setupproof id=quickstart -->
```sh
printf 'install dependencies\n'
printf 'run tests\n'
test -f README.md
```
EOF

git init -q
git add README.md
git -c user.name='Demo Maintainer' -c user.email='demo@example.invalid' \
  commit -m initial -q

alias setupproof="$BIN"
