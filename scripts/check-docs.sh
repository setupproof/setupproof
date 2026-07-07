#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
DOC="$ROOT/docs/INSTALL.md"
NATIVE_WINDOWS_ADR="$ROOT/docs/adr/0010-native-windows-support-scope.md"
NATIVE_WINDOWS_SHELL_ADR="$ROOT/docs/adr/0011-native-windows-shell-semantics.md"
NATIVE_WINDOWS_PATH_ENV_ADR="$ROOT/docs/adr/0012-native-windows-path-and-environment-semantics.md"
PUBLIC_DOCS="$ROOT/README.md
$ROOT/LICENSE
$ROOT/CODE_OF_CONDUCT.md
$ROOT/SECURITY.md
$ROOT/CONTRIBUTING.md
$ROOT/SUPPORT.md
$ROOT/CHANGELOG.md
$ROOT/docs/ARCHITECTURE.md
$ROOT/docs/DOCKER_RUNNER.md
$ROOT/docs/INSTALL.md
$ROOT/docs/RECIPES.md
$ROOT/docs/RELEASE_READINESS.md
$ROOT/docs/SCHEMAS.md
$ROOT/docs/TROUBLESHOOTING.md
$ROOT/docs/COMPARISON.md
$ROOT/docs/DECISIONS.md
$ROOT/.github/labels.yml
$ROOT/.github/repository-metadata.yml
$ROOT/.github/pull_request_template.md
$ROOT/.github/ISSUE_TEMPLATE/contributor_task.md
$ROOT/.github/ISSUE_TEMPLATE/design_discussion.md
$ROOT/.github/ISSUE_TEMPLATE/runner_issue.md
$ROOT/.github/ISSUE_TEMPLATE/schema_contract.md
$ROOT/.github/ISSUE_TEMPLATE/platform_support.md
$ROOT/.github/ISSUE_TEMPLATE/setup-doc-failure.md
$ROOT/.github/ISSUE_TEMPLATE/bug_report.md"

fail() {
  printf 'check-docs: %s\n' "$*" >&2
  exit 1
}

assert_contains() {
  file=$1
  expected=$2
  if ! grep -Fq -- "$expected" "$file"; then
    printf 'missing expected text: %s\n--- %s ---\n' "$expected" "$file" >&2
    sed -n '1,240p' "$file" >&2
    exit 1
  fi
}

assert_not_contains() {
  file=$1
  unexpected=$2
  if grep -Fq -- "$unexpected" "$file"; then
    printf 'unexpected text: %s\n--- %s ---\n' "$unexpected" "$file" >&2
    sed -n '1,240p' "$file" >&2
    exit 1
  fi
}

[ -f "$DOC" ] || fail "missing docs/INSTALL.md"
[ -f "$NATIVE_WINDOWS_ADR" ] || fail "missing native Windows ADR"
[ -f "$NATIVE_WINDOWS_SHELL_ADR" ] || fail "missing native Windows shell ADR"
[ -f "$NATIVE_WINDOWS_PATH_ENV_ADR" ] || fail "missing native Windows path/env ADR"
[ -f "$ROOT/schemas/setupproof-config.schema.json" ] || fail "missing config schema"
for file in $PUBLIC_DOCS; do
  [ -f "$file" ] || fail "missing public doc: $file"
done

assert_contains "$DOC" "<!-- ci-snippet:github-actions -->"
assert_contains "$DOC" "<!-- ci-snippet:gitlab-ci -->"
assert_contains "$DOC" "<!-- ci-snippet:circleci -->"
assert_contains "$DOC" "<!-- ci-snippet:generic-shell -->"
assert_contains "$DOC" "Native Windows execution is unsupported in v0.1"
assert_contains "$DOC" "PowerShell fenced blocks are unsupported in v0.1"
assert_contains "$DOC" "WSL2"
assert_contains "$DOC" "ADR 0010 records the native Windows support boundary"
assert_contains "$DOC" "ADR 0011 records the current shell decision"
assert_contains "$DOC" "ADR 0012 records the path and environment"
assert_contains "$NATIVE_WINDOWS_ADR" "Native Windows execution remains unsupported in v0.1"
assert_contains "$NATIVE_WINDOWS_ADR" "Windows CI coverage runs the relevant CLI, runner, report, and docs tests"
assert_contains "$NATIVE_WINDOWS_ADR" 'PowerShell and `cmd` fences are unsupported'
assert_contains "$NATIVE_WINDOWS_SHELL_ADR" "Native Windows shell execution is not supported in v0.1"
assert_contains "$NATIVE_WINDOWS_SHELL_ADR" '`powershell`, `pwsh`, and `cmd` fences are unsupported'
assert_contains "$NATIVE_WINDOWS_SHELL_ADR" 'The local and Action runners must not reinterpret `shell`'
assert_contains "$NATIVE_WINDOWS_PATH_ENV_ADR" "Native Windows path and environment behavior remains unsupported in v0.1"
assert_contains "$NATIVE_WINDOWS_PATH_ENV_ADR" "Absolute Git workspace paths, including native Windows drive-letter and UNC"
assert_contains "$NATIVE_WINDOWS_PATH_ENV_ADR" "Native Windows case-insensitive environment-name handling is not supported"
assert_contains "$ROOT/docs/DECISIONS.md" "ADR 0010 records the support boundary"
assert_contains "$ROOT/docs/DECISIONS.md" "ADR 0011 records the current shell decision"
assert_contains "$ROOT/docs/DECISIONS.md" "ADR 0012 records the current path and environment"
assert_contains "$ROOT/docs/TROUBLESHOOTING.md" "ADR 0010 records the native Windows"
assert_contains "$ROOT/docs/TROUBLESHOOTING.md" 'ADR 0011 records that `shell` remains POSIX'
assert_contains "$ROOT/docs/TROUBLESHOOTING.md" "ADR 0012 records that native Windows path and environment"
assert_contains "$ROOT/.github/ISSUE_TEMPLATE/platform_support.md" "start from ADR 0010, ADR 0011, and ADR 0012"
assert_contains "$DOC" "brew install setupproof/tap/setupproof"
assert_contains "$DOC" "go install github.com/setupproof/setupproof/cmd/setupproof@v0.1.3"
assert_contains "$DOC" "setupproof_0.1.3_checksums.txt"
assert_contains "$DOC" "setupproof review README.md"
assert_contains "$DOC" "setupproof --report-json setupproof-report.json --require-blocks --no-color --no-glyphs README.md"
assert_contains "$DOC" "require-blocks: \"true\""
assert_contains "$ROOT/LICENSE" "Apache License"
assert_contains "$ROOT/NOTICE" "Licensed under the Apache License, Version 2.0."
assert_contains "$ROOT/README.md" 'Apache License, Version 2.0 (`Apache-2.0`)'
assert_contains "$ROOT/README.md" "setupproof/setupproof@v0.1.3"
assert_contains "$ROOT/examples/node-npm/package.json" '"license": "Apache-2.0"'
assert_contains "$ROOT/examples/monorepo/package.json" '"license": "Apache-2.0"'
assert_contains "$ROOT/examples/monorepo/packages/web/package.json" '"license": "Apache-2.0"'
assert_contains "$ROOT/examples/rust/Cargo.toml" 'license = "Apache-2.0"'
assert_contains "$ROOT/README.md" "docs/ARCHITECTURE.md"
assert_contains "$ROOT/README.md" "docs/DOCKER_RUNNER.md"
assert_contains "$ROOT/README.md" "docs/RECIPES.md"
assert_contains "$ROOT/README.md" "docs/TROUBLESHOOTING.md"
assert_contains "$ROOT/README.md" "docs/SCHEMAS.md"
assert_contains "$ROOT/docs/DOCKER_RUNNER.md" "security sandbox"
assert_contains "$ROOT/docs/DOCKER_RUNNER.md" "network: false"
assert_contains "$ROOT/docs/DOCKER_RUNNER.md" "digest"
assert_contains "$ROOT/docs/DOCKER_RUNNER.md" "env:"
assert_contains "$ROOT/docs/RECIPES.md" "schemas/setupproof-config.schema.json"
assert_contains "$ROOT/docs/RECIPES.md" "version: 1"
assert_contains "$ROOT/docs/RECIPES.md" "requireBlocks: false"
assert_contains "$ROOT/docs/RECIPES.md" "Small Go Module"
assert_contains "$ROOT/docs/RECIPES.md" "examples/go"
assert_contains "$ROOT/docs/RECIPES.md" "go-test"
assert_contains "$ROOT/docs/RECIPES.md" "Small Node/npm Package"
assert_contains "$ROOT/docs/RECIPES.md" "examples/node-npm"
assert_contains "$ROOT/docs/RECIPES.md" "node-npm-test"
assert_contains "$ROOT/docs/RECIPES.md" "no secret environment passthrough"
assert_contains "$ROOT/docs/RECIPES.md" "Monorepo With Package-Local Quickstarts"
assert_contains "$ROOT/docs/RECIPES.md" "setupproof init --workflow docs/web.md docs/api.md"
assert_contains "$ROOT/docs/RECIPES.md" "files: |"
assert_contains "$ROOT/docs/RECIPES.md" "docs/web.md"
assert_contains "$ROOT/docs/RECIPES.md" "docs/api.md"
assert_contains "$ROOT/docs/RECIPES.md" "runner: docker"
assert_contains "$ROOT/CONTRIBUTING.md" "Markdown -> Plan -> Workspace Copy -> Runner -> Report"
assert_contains "$ROOT/CONTRIBUTING.md" "Claiming Issues"
assert_contains "$ROOT/CONTRIBUTING.md" "linked pull request"
assert_contains "$ROOT/CONTRIBUTING.md" "make dogfood"
assert_contains "$ROOT/CONTRIBUTING.md" "CODE_OF_CONDUCT.md"
assert_contains "$ROOT/SUPPORT.md" "docs/TROUBLESHOOTING.md"
assert_contains "$ROOT/docs/TROUBLESHOOTING.md" "setupproof doctor README.md"
assert_contains "$ROOT/docs/TROUBLESHOOTING.md" "No Marked Blocks Found"
assert_contains "$ROOT/docs/TROUBLESHOOTING.md" "Docker Runner Problems"
assert_contains "$ROOT/docs/TROUBLESHOOTING.md" "Missing Environment Variables"
assert_contains "$ROOT/.github/pull_request_template.md" "make check"
assert_contains "$ROOT/.github/pull_request_template.md" "make fmt-check"
assert_contains "$ROOT/.github/pull_request_template.md" "make release-check"
assert_contains "$ROOT/.github/ISSUE_TEMPLATE/bug_report.md" 'labels: "bug"'
assert_contains "$ROOT/.github/ISSUE_TEMPLATE/setup-doc-failure.md" 'labels: "support"'
assert_contains "$ROOT/.github/ISSUE_TEMPLATE/contributor_task.md" "make fmt-check"
assert_contains "$ROOT/.github/ISSUE_TEMPLATE/runner_issue.md" "make fmt-check"
assert_contains "$ROOT/docs/ARCHITECTURE.md" "Unmarked Markdown blocks never execute"
assert_contains "$ROOT/docs/ARCHITECTURE.md" "Report JSON is machine-facing output"
assert_contains "$ROOT/docs/RELEASE_READINESS.md" "Moving major Action tags"
assert_contains "$ROOT/docs/RELEASE_READINESS.md" "make release-archives"
assert_contains "$ROOT/docs/RELEASE_READINESS.md" "make npm-check"
assert_contains "$ROOT/docs/RELEASE_READINESS.md" "make schemas"
assert_contains "$ROOT/docs/RELEASE_READINESS.md" "Schema publication gates"
assert_contains "$ROOT/docs/RELEASE_READINESS.md" "make release-check"
assert_contains "$ROOT/docs/RELEASE_READINESS.md" "scripts/check-release-archives.sh"
assert_contains "$ROOT/docs/RELEASE_READINESS.md" "scripts/check-npm-package.sh"
assert_contains "$ROOT/docs/RELEASE_READINESS.md" "packed-tarball smoke"
assert_contains "$ROOT/docs/RELEASE_READINESS.md" "patched Go toolchain"
assert_contains "$ROOT/docs/RELEASE_READINESS.md" "CODE_OF_CONDUCT.md"
assert_contains "$ROOT/docs/RELEASE_READINESS.md" "The SetupProof workflow is green on"
assert_contains "$ROOT/.github/repository-metadata.yml" "github-actions"
assert_contains "$ROOT/.github/labels.yml" "good first issue"
assert_contains "$ROOT/.github/labels.yml" "runner"
assert_not_contains "$ROOT/.github/labels.yml" "launch"
assert_contains "$ROOT/docs/SCHEMAS.md" "schemas/v1.0.0/setupproof-plan.schema.json"
assert_contains "$ROOT/docs/SCHEMAS.md" "schemas/v1.0.0/setupproof-report.schema.json"
assert_contains "$ROOT/docs/SCHEMAS.md" "schemas/v1.0.0/setupproof-config.schema.json"
assert_contains "$ROOT/docs/SCHEMAS.md" "byte-for-byte"

for file in $PUBLIC_DOCS; do
  assert_not_contains "$file" "pull_request_target"
  assert_not_contains "$file" "\${{ secrets."
  assert_not_contains "$file" "@""v1"
  assert_not_contains "$file" "npx setup""proof"
  assert_not_contains "$file" "npm install -g setup""proof"
  assert_not_contains "$file" "npm i -g setup""proof"
  assert_not_contains "$file" "winget install setup""proof"
  assert_not_contains "$file" "choco install setup""proof"
  assert_not_contains "$file" "scoop install setup""proof"
done

(cd "$ROOT" && go test -run TestInstallDocCISnippetsAndDeferredClaims ./internal/cli)

printf 'docs checks passed\n'
