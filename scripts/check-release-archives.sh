#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
VERSION="${1:-}"

if [ -z "$VERSION" ]; then
  printf 'usage: scripts/check-release-archives.sh v0.1.0\n' >&2
  exit 2
fi

case "$VERSION" in
  v[0-9]*.[0-9]*.[0-9]*) ;;
  [0-9]*.[0-9]*.[0-9]*) VERSION="v$VERSION" ;;
  *)
    printf 'check-release-archives: version must look like v0.1.0\n' >&2
    exit 2
    ;;
esac

PLAIN_VERSION="${VERSION#v}"
DIST="$ROOT/dist/$VERSION"
MANIFEST="$DIST/setupproof_${PLAIN_VERSION}_checksums.txt"

fail() {
  printf 'check-release-archives: %s\n' "$*" >&2
  exit 1
}

[ -d "$DIST" ] || fail "missing release directory: $DIST"
[ -f "$MANIFEST" ] || fail "missing checksum manifest: $MANIFEST"

archive_count=$(find "$DIST" -maxdepth 1 -type f -name '*.tar.gz' | wc -l | tr -d ' ')
[ "$archive_count" = "4" ] || fail "expected 4 archives, found $archive_count"

manifest_count=$(wc -l < "$MANIFEST" | tr -d ' ')
[ "$manifest_count" = "$archive_count" ] || fail "manifest lists $manifest_count entries for $archive_count archives"

if command -v sha256sum >/dev/null 2>&1; then
  (cd "$DIST" && sha256sum -c "$(basename "$MANIFEST")")
elif command -v shasum >/dev/null 2>&1; then
  (cd "$DIST" && shasum -a 256 -c "$(basename "$MANIFEST")")
else
  fail "sha256sum or shasum is required"
fi

for archive in "$DIST"/*.tar.gz; do
  name=$(basename "$archive")
  count=$(awk -v name="$name" '$2 == name { count++ } END { print count + 0 }' "$MANIFEST")
  [ "$count" = "1" ] || fail "manifest must list $name exactly once"

  contents=$(tar -tzf "$archive")
  printf '%s\n' "$contents" | grep -Fx './setupproof' >/dev/null || fail "$name missing setupproof"
  printf '%s\n' "$contents" | grep -Fx './LICENSE' >/dev/null || fail "$name missing LICENSE"
  printf '%s\n' "$contents" | grep -Fx './NOTICE' >/dev/null || fail "$name missing NOTICE"
done

host_os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$host_os" in
  darwin|linux) ;;
  *) fail "unsupported host OS for binary smoke test: $host_os" ;;
esac

host_arch=$(uname -m)
case "$host_arch" in
  x86_64|amd64) host_arch=amd64 ;;
  arm64|aarch64) host_arch=arm64 ;;
  *) fail "unsupported host architecture for binary smoke test: $host_arch" ;;
esac

host_archive="$DIST/setupproof_${PLAIN_VERSION}_${host_os}_${host_arch}.tar.gz"
[ -f "$host_archive" ] || fail "missing host archive: $(basename "$host_archive")"

tmp="${TMPDIR:-/tmp}/setupproof-release-check-$$"
rm -rf "$tmp"
mkdir -p "$tmp"
trap 'rm -rf "$tmp"' EXIT INT TERM

tar -xzf "$host_archive" -C "$tmp"
[ -x "$tmp/setupproof" ] || fail "host archive setupproof is not executable"

version_output=$("$tmp/setupproof" --version)
[ "$version_output" = "setupproof $PLAIN_VERSION" ] || fail "unexpected version output: $version_output"

printf 'release archive checks passed\n'
