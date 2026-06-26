#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
VERSION="${1:-}"

if [ -z "$VERSION" ]; then
  printf 'usage: scripts/check-npm-package.sh v0.1.0\n' >&2
  exit 2
fi

case "$VERSION" in
  v[0-9]*.[0-9]*.[0-9]*) ;;
  [0-9]*.[0-9]*.[0-9]*) VERSION="v$VERSION" ;;
  *)
    printf 'check-npm-package: version must look like v0.1.0\n' >&2
    exit 2
    ;;
esac

PLAIN_VERSION="${VERSION#v}"
PKG_ROOT="$ROOT/dist/$VERSION/npm/package"

fail() {
  printf 'check-npm-package: %s\n' "$*" >&2
  exit 1
}

command -v node >/dev/null 2>&1 || fail "node is required"
command -v npm >/dev/null 2>&1 || fail "npm is required"

[ -d "$PKG_ROOT" ] || "$ROOT/scripts/package-npm.sh" "$VERSION" >/dev/null
[ -f "$PKG_ROOT/package.json" ] || fail "missing generated package.json"
[ -f "$PKG_ROOT/vendor/manifest.json" ] || fail "missing generated binary manifest"

node -e '
const fs = require("fs");
const pkg = JSON.parse(fs.readFileSync(process.argv[1], "utf8"));
if (pkg.scripts && pkg.scripts.postinstall) {
  throw new Error("postinstall scripts are not allowed");
}
if (pkg.dependencies && Object.keys(pkg.dependencies).length > 0) {
  throw new Error("production dependencies are not allowed");
}
' "$PKG_ROOT/package.json"

version_output=$("$PKG_ROOT/bin/setupproof" --version)
[ "$version_output" = "setupproof $PLAIN_VERSION" ] || fail "unexpected wrapper version output: $version_output"

tmp="${TMPDIR:-/tmp}/setupproof-npm-check-$$"
rm -rf "$tmp"
mkdir -p "$tmp/project"
trap 'rm -rf "$tmp"' EXIT INT TERM

npm pack "$PKG_ROOT" --dry-run --json >/dev/null
npm pack "$PKG_ROOT" --pack-destination "$tmp" --json >/dev/null
tarball=$(find "$tmp" -maxdepth 1 -type f -name 'setupproof-*.tgz' | head -n 1)
[ -n "$tarball" ] || fail "npm pack did not write a tarball"

(
  cd "$tmp/project"
  npm init -y >/dev/null
  npm install --no-audit --no-fund "$tarball" >/dev/null
  output=$(./node_modules/.bin/setupproof --version)
  [ "$output" = "setupproof $PLAIN_VERSION" ] || fail "unexpected installed version output: $output"
)

printf 'npm package checks passed\n'
