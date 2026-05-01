#!/usr/bin/env bash
set -euo pipefail

ROOT="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
VERSION="${1:-}"

if [ -z "$VERSION" ]; then
  printf 'usage: scripts/package-release.sh v0.1.0\n' >&2
  exit 2
fi

case "$VERSION" in
  v[0-9]*.[0-9]*.[0-9]*) ;;
  *)
    printf 'package-release: version must look like v0.1.0\n' >&2
    exit 2
    ;;
esac

PLAIN_VERSION="${VERSION#v}"
DIST="$ROOT/dist/$VERSION"
LDFLAGS="-X github.com/setupproof/setupproof/internal/app.Version=$PLAIN_VERSION"

rm -rf "$DIST"
mkdir -p "$DIST"

build_one() {
  local goos="$1"
  local goarch="$2"
  local dir="$DIST/setupproof_${PLAIN_VERSION}_${goos}_${goarch}"
  local archive="$DIST/setupproof_${PLAIN_VERSION}_${goos}_${goarch}.tar.gz"

  mkdir -p "$dir"
  GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 \
    go build -trimpath -ldflags "$LDFLAGS" -o "$dir/setupproof" ./cmd/setupproof
  cp "$ROOT/LICENSE" "$ROOT/NOTICE" "$dir/"
  tar -C "$dir" -czf "$archive" .
  rm -rf "$dir"
}

cd "$ROOT"
build_one linux amd64
build_one linux arm64
build_one darwin amd64
build_one darwin arm64

(
  cd "$DIST"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum ./*.tar.gz
  else
    shasum -a 256 ./*.tar.gz
  fi | awk '{ name = $2; sub(/^\.\//, "", name); print $1 "  " name }' > "setupproof_${PLAIN_VERSION}_checksums.txt"
)

printf 'wrote %s\n' "$DIST"
