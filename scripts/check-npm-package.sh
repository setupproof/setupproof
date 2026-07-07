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
DIST="$ROOT/dist/$VERSION"
CHECKSUM_MANIFEST="$DIST/setupproof_${PLAIN_VERSION}_checksums.txt"
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
[ -f "$CHECKSUM_MANIFEST" ] || fail "missing checksum manifest: $CHECKSUM_MANIFEST"

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

node - <<'NODE' "$PKG_ROOT" "$PLAIN_VERSION" "$CHECKSUM_MANIFEST"
const fs = require("fs");
const path = require("path");

const [pkgRoot, version, checksumManifest] = process.argv.slice(2);
const manifestPath = path.join(pkgRoot, "vendor", "manifest.json");
const manifest = JSON.parse(fs.readFileSync(manifestPath, "utf8"));

function fail(message) {
  throw new Error(message);
}

function assertRelativePath(value) {
  if (typeof value !== "string" || value.length === 0) {
    fail("binary path must be a non-empty string");
  }
  if (path.isAbsolute(value) || value.split(/[\\/]+/).includes("..")) {
    fail(`binary path must stay inside the package: ${value}`);
  }
}

const expected = new Map([
  ["linux/x64", {
    path: "vendor/linux-x64/setupproof",
    archive: `setupproof_${version}_linux_amd64.tar.gz`,
  }],
  ["linux/arm64", {
    path: "vendor/linux-arm64/setupproof",
    archive: `setupproof_${version}_linux_arm64.tar.gz`,
  }],
  ["darwin/x64", {
    path: "vendor/darwin-x64/setupproof",
    archive: `setupproof_${version}_darwin_amd64.tar.gz`,
  }],
  ["darwin/arm64", {
    path: "vendor/darwin-arm64/setupproof",
    archive: `setupproof_${version}_darwin_arm64.tar.gz`,
  }],
]);

const checksums = new Map();
for (const line of fs.readFileSync(checksumManifest, "utf8").trim().split(/\n+/)) {
  const match = line.match(/^([0-9a-f]{64})\s+(\S+)$/i);
  if (!match) {
    fail(`invalid checksum manifest line: ${line}`);
  }
  checksums.set(match[2], match[1].toLowerCase());
}

if (manifest.version !== version) {
  fail(`manifest version ${manifest.version} does not match ${version}`);
}
if (!Array.isArray(manifest.binaries)) {
  fail("manifest binaries must be an array");
}
if (manifest.binaries.length !== expected.size) {
  fail(`manifest must list ${expected.size} binaries, found ${manifest.binaries.length}`);
}

const seen = new Set();
const declaredFiles = new Set(["vendor/manifest.json"]);

for (const entry of manifest.binaries) {
  const key = `${entry.platform}/${entry.arch}`;
  const want = expected.get(key);
  if (!want) {
    fail(`unsupported binary entry: ${key}`);
  }
  if (seen.has(key)) {
    fail(`duplicate binary entry: ${key}`);
  }
  seen.add(key);

  assertRelativePath(entry.path);
  if (entry.path !== want.path) {
    fail(`${key} path ${entry.path} does not match ${want.path}`);
  }
  if (entry.archive !== want.archive) {
    fail(`${key} archive ${entry.archive} does not match ${want.archive}`);
  }
  if (!/^[0-9a-f]{64}$/i.test(entry.sha256 || "")) {
    fail(`${key} sha256 must be a 64-character hex digest`);
  }
  const checksum = checksums.get(entry.archive);
  if (!checksum) {
    fail(`${key} archive is missing from the release checksum manifest`);
  }
  if (checksum !== entry.sha256.toLowerCase()) {
    fail(`${key} sha256 does not match the release checksum manifest`);
  }

  const binaryPath = path.join(pkgRoot, entry.path);
  const stat = fs.statSync(binaryPath);
  if (!stat.isFile()) {
    fail(`${entry.path} is not a file`);
  }
  if ((stat.mode & 0o111) === 0) {
    fail(`${entry.path} is not executable`);
  }
  declaredFiles.add(entry.path);
}

for (const key of expected.keys()) {
  if (!seen.has(key)) {
    fail(`missing binary entry: ${key}`);
  }
}

function walk(dir) {
  for (const name of fs.readdirSync(dir)) {
    const fullPath = path.join(dir, name);
    const relative = path.relative(pkgRoot, fullPath).split(path.sep).join("/");
    const stat = fs.statSync(fullPath);
    if (stat.isDirectory()) {
      walk(fullPath);
    } else if (!declaredFiles.has(relative)) {
      fail(`undeclared vendor file: ${relative}`);
    }
  }
}

walk(path.join(pkgRoot, "vendor"));
NODE

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
