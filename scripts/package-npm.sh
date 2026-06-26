#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
VERSION="${1:-}"

if [ -z "$VERSION" ]; then
  printf 'usage: scripts/package-npm.sh v0.1.0\n' >&2
  exit 2
fi

case "$VERSION" in
  v[0-9]*.[0-9]*.[0-9]*) ;;
  [0-9]*.[0-9]*.[0-9]*) VERSION="v$VERSION" ;;
  *)
    printf 'package-npm: version must look like v0.1.0\n' >&2
    exit 2
    ;;
esac

PLAIN_VERSION="${VERSION#v}"
DIST="$ROOT/dist/$VERSION"
MANIFEST="$DIST/setupproof_${PLAIN_VERSION}_checksums.txt"
TEMPLATE="$ROOT/packaging/npm"
PKG_ROOT="$DIST/npm/package"

fail() {
  printf 'package-npm: %s\n' "$*" >&2
  exit 1
}

[ -d "$DIST" ] || fail "missing release directory: $DIST"
[ -f "$MANIFEST" ] || fail "missing checksum manifest: $MANIFEST"
[ -f "$TEMPLATE/package.json.in" ] || fail "missing npm package template"

rm -rf "$PKG_ROOT"
mkdir -p "$PKG_ROOT/bin" "$PKG_ROOT/vendor"

sed "s/__VERSION__/$PLAIN_VERSION/g" "$TEMPLATE/package.json.in" > "$PKG_ROOT/package.json"
sed "s/__VERSION__/$PLAIN_VERSION/g" "$TEMPLATE/README.md.in" > "$PKG_ROOT/README.md"
cp "$TEMPLATE/bin/setupproof" "$PKG_ROOT/bin/setupproof"
cp "$ROOT/LICENSE" "$ROOT/NOTICE" "$PKG_ROOT/"
chmod 755 "$PKG_ROOT/bin/setupproof"

checksum_for() {
  name=$1
  awk -v name="$name" '$2 == name { print $1; found = 1 } END { if (!found) exit 1 }' "$MANIFEST"
}

copy_binary() {
  goos=$1
  goarch=$2
  npm_platform=$3
  npm_arch=$4
  archive="setupproof_${PLAIN_VERSION}_${goos}_${goarch}.tar.gz"
  archive_path="$DIST/$archive"
  target_dir="$PKG_ROOT/vendor/${npm_platform}-${npm_arch}"
  extract_dir="$DIST/npm/extract-${npm_platform}-${npm_arch}"

  [ -f "$archive_path" ] || fail "missing release archive: $archive"
  rm -rf "$extract_dir"
  mkdir -p "$extract_dir" "$target_dir"
  tar -xzf "$archive_path" -C "$extract_dir"
  [ -x "$extract_dir/setupproof" ] || fail "$archive did not contain executable setupproof"
  cp "$extract_dir/setupproof" "$target_dir/setupproof"
  chmod 755 "$target_dir/setupproof"
  rm -rf "$extract_dir"
}

copy_binary linux amd64 linux x64
copy_binary linux arm64 linux arm64
copy_binary darwin amd64 darwin x64
copy_binary darwin arm64 darwin arm64

linux_x64_archive="setupproof_${PLAIN_VERSION}_linux_amd64.tar.gz"
linux_arm64_archive="setupproof_${PLAIN_VERSION}_linux_arm64.tar.gz"
darwin_x64_archive="setupproof_${PLAIN_VERSION}_darwin_amd64.tar.gz"
darwin_arm64_archive="setupproof_${PLAIN_VERSION}_darwin_arm64.tar.gz"

linux_x64_sha=$(checksum_for "$linux_x64_archive") || fail "missing checksum for $linux_x64_archive"
linux_arm64_sha=$(checksum_for "$linux_arm64_archive") || fail "missing checksum for $linux_arm64_archive"
darwin_x64_sha=$(checksum_for "$darwin_x64_archive") || fail "missing checksum for $darwin_x64_archive"
darwin_arm64_sha=$(checksum_for "$darwin_arm64_archive") || fail "missing checksum for $darwin_arm64_archive"

cat > "$PKG_ROOT/vendor/manifest.json" <<EOF
{
  "version": "$PLAIN_VERSION",
  "binaries": [
    {
      "platform": "linux",
      "arch": "x64",
      "path": "vendor/linux-x64/setupproof",
      "archive": "$linux_x64_archive",
      "sha256": "$linux_x64_sha"
    },
    {
      "platform": "linux",
      "arch": "arm64",
      "path": "vendor/linux-arm64/setupproof",
      "archive": "$linux_arm64_archive",
      "sha256": "$linux_arm64_sha"
    },
    {
      "platform": "darwin",
      "arch": "x64",
      "path": "vendor/darwin-x64/setupproof",
      "archive": "$darwin_x64_archive",
      "sha256": "$darwin_x64_sha"
    },
    {
      "platform": "darwin",
      "arch": "arm64",
      "path": "vendor/darwin-arm64/setupproof",
      "archive": "$darwin_arm64_archive",
      "sha256": "$darwin_arm64_sha"
    }
  ]
}
EOF

printf 'wrote %s\n' "$PKG_ROOT"
