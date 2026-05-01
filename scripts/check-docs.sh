#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
DOC="$ROOT/docs/INSTALL.md"
PUBLIC_DOCS="$ROOT/README.md
$ROOT/LICENSE
$ROOT/CODE_OF_CONDUCT.md
$ROOT/SECURITY.md
$ROOT/CONTRIBUTING.md
$ROOT/SUPPORT.md
$ROOT/CHANGELOG.md
$ROOT/docs/ARCHITECTURE.md
$ROOT/docs/INSTALL.md
$ROOT/docs/RELEASE_READINESS.md
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
assert_contains "$DOC" "go install github.com/setupproof/setupproof/cmd/setupproof@v0.1.0"
assert_contains "$DOC" "setupproof_0.1.0_checksums.txt"
assert_contains "$DOC" "setupproof review README.md"
assert_contains "$DOC" "setupproof --report-json setupproof-report.json --require-blocks --no-color --no-glyphs README.md"
assert_contains "$DOC" "require-blocks: \"true\""
assert_contains "$ROOT/LICENSE" "Apache License"
assert_contains "$ROOT/NOTICE" "Licensed under the Apache License, Version 2.0."
assert_contains "$ROOT/README.md" 'Apache License, Version 2.0 (`Apache-2.0`)'
assert_contains "$ROOT/README.md" "setupproof/setupproof@v0.1.0"
assert_contains "$ROOT/examples/node-npm/package.json" '"license": "Apache-2.0"'
assert_contains "$ROOT/examples/monorepo/package.json" '"license": "Apache-2.0"'
assert_contains "$ROOT/examples/monorepo/packages/web/package.json" '"license": "Apache-2.0"'
assert_contains "$ROOT/examples/rust/Cargo.toml" 'license = "Apache-2.0"'
assert_contains "$ROOT/README.md" "docs/ARCHITECTURE.md"
assert_contains "$ROOT/CONTRIBUTING.md" "Markdown -> Plan -> Workspace Copy -> Runner -> Report"
assert_contains "$ROOT/CONTRIBUTING.md" "make dogfood"
assert_contains "$ROOT/CONTRIBUTING.md" "CODE_OF_CONDUCT.md"
assert_contains "$ROOT/.github/pull_request_template.md" "make check"
assert_contains "$ROOT/.github/pull_request_template.md" "make fmt-check"
assert_contains "$ROOT/.github/ISSUE_TEMPLATE/bug_report.md" 'labels: "bug"'
assert_contains "$ROOT/.github/ISSUE_TEMPLATE/setup-doc-failure.md" 'labels: "support"'
assert_contains "$ROOT/.github/ISSUE_TEMPLATE/contributor_task.md" "make fmt-check"
assert_contains "$ROOT/.github/ISSUE_TEMPLATE/runner_issue.md" "make fmt-check"
assert_contains "$ROOT/docs/ARCHITECTURE.md" "Unmarked Markdown blocks never execute"
assert_contains "$ROOT/docs/ARCHITECTURE.md" "Report JSON is machine-facing output"
assert_contains "$ROOT/docs/RELEASE_READINESS.md" "Moving major Action tags"
assert_contains "$ROOT/docs/RELEASE_READINESS.md" "make release-archives"
assert_contains "$ROOT/docs/RELEASE_READINESS.md" "CODE_OF_CONDUCT.md"
assert_contains "$ROOT/docs/RELEASE_READINESS.md" "The SetupProof workflow is green on"
assert_contains "$ROOT/.github/repository-metadata.yml" "github-actions"
assert_contains "$ROOT/.github/labels.yml" "good first issue"
assert_contains "$ROOT/.github/labels.yml" "runner"
assert_not_contains "$ROOT/.github/labels.yml" "launch"

for file in $PUBLIC_DOCS; do
  assert_not_contains "$file" "pull_request_target"
  assert_not_contains "$file" "\${{ secrets."
  assert_not_contains "$file" "@""v1"
  assert_not_contains "$file" "npx setup""proof"
  assert_not_contains "$file" "npm install -g setup""proof"
  assert_not_contains "$file" "npm i -g setup""proof"
  assert_not_contains "$file" "brew install setup""proof"
  assert_not_contains "$file" "winget install setup""proof"
  assert_not_contains "$file" "choco install setup""proof"
  assert_not_contains "$file" "scoop install setup""proof"
done

(cd "$ROOT" && go test -run TestInstallDocCISnippetsAndDeferredClaims ./internal/cli)

printf 'docs checks passed\n'
