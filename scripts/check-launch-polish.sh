#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
DEMO_OUT=${TMPDIR:-/tmp}/setupproof-demo-check-$$.out

cleanup() {
  rm -f "$DEMO_OUT"
}
trap cleanup EXIT INT TERM

fail() {
  printf 'check-launch-polish: %s\n' "$*" >&2
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

README="$ROOT/README.md"
SUPPORT="$ROOT/SUPPORT.md"
AGENT_USAGE="$ROOT/docs/AGENT_USAGE.md"
LLMS="$ROOT/llms.txt"
DEMO_README="$ROOT/docs/demo/README.md"
DEMO_SCRIPT="$ROOT/docs/demo/terminal-demo.sh"
DEMO_TRANSCRIPT="$ROOT/docs/demo/terminal-demo.txt"
LABELS="$ROOT/.github/labels.yml"
METADATA="$ROOT/.github/repository-metadata.yml"
BUG_TEMPLATE="$ROOT/.github/ISSUE_TEMPLATE/bug_report.md"
SUPPORT_TEMPLATE="$ROOT/.github/ISSUE_TEMPLATE/setup-doc-failure.md"
CONTRIBUTOR_TEMPLATE="$ROOT/.github/ISSUE_TEMPLATE/contributor_task.md"
RUNNER_TEMPLATE="$ROOT/.github/ISSUE_TEMPLATE/runner_issue.md"
PR_TEMPLATE="$ROOT/.github/pull_request_template.md"
ACTION="$ROOT/action.yml"

[ -f "$README" ] || fail "missing README.md"
[ -f "$SUPPORT" ] || fail "missing SUPPORT.md"
[ -f "$AGENT_USAGE" ] || fail "missing docs/AGENT_USAGE.md"
[ -f "$LLMS" ] || fail "missing llms.txt"
[ -f "$DEMO_README" ] || fail "missing docs/demo/README.md"
[ -f "$DEMO_SCRIPT" ] || fail "missing docs/demo/terminal-demo.sh"
[ -f "$DEMO_TRANSCRIPT" ] || fail "missing docs/demo/terminal-demo.txt"
[ -f "$LABELS" ] || fail "missing .github/labels.yml"
[ -f "$METADATA" ] || fail "missing .github/repository-metadata.yml"
[ -f "$BUG_TEMPLATE" ] || fail "missing bug report template"
[ -f "$SUPPORT_TEMPLATE" ] || fail "missing setup doc failure template"
[ -f "$CONTRIBUTOR_TEMPLATE" ] || fail "missing contributor task template"
[ -f "$RUNNER_TEMPLATE" ] || fail "missing runner issue template"
[ -f "$PR_TEMPLATE" ] || fail "missing pull request template"
[ -f "$ACTION" ] || fail "missing action.yml"

assert_contains "$README" "Test marked README quickstarts from a clean workspace"
assert_contains "$README" "## Install"
assert_contains "$README" "## Use It"
assert_contains "$README" "go install github.com/setupproof/setupproof/cmd/setupproof@v0.1.0"
assert_contains "$README" "setupproof review README.md"
assert_contains "$README" "setupproof init"
assert_contains "$README" "setupproof/setupproof@v0.1.0"
assert_contains "$README" "## For Agents"
assert_contains "$README" "Apache-2.0"
assert_contains "$README" "docs/demo/terminal-demo.sh"
assert_contains "$README" "docs/demo/terminal-demo.txt"
assert_contains "$README" "make check"
assert_contains "$README" "make release-archives"

assert_contains "$SUPPORT" "SetupProof v0.1.0 can run from the Go module install path"
assert_contains "$SUPPORT" "setupproof review README.md"
assert_contains "$SUPPORT" "Native Windows execution and PowerShell fenced blocks are unsupported in v0.1."

assert_contains "$AGENT_USAGE" "marked shell blocks are maintainer-approved verification targets"
assert_contains "$AGENT_USAGE" "Never execute unmarked Markdown shell blocks"
assert_contains "$AGENT_USAGE" "setupproof --list README.md"
assert_contains "$LLMS" "SetupProof verifies explicitly marked README shell quickstarts"
assert_contains "$LLMS" "Plan JSON Schema"
assert_contains "$LLMS" "Exit codes:"

assert_contains "$DEMO_README" "12-20 second terminal recording"
assert_contains "$DEMO_README" "terminal-demo.txt"
assert_contains "$DEMO_SCRIPT" "SETUPPROOF_DEMO_PAUSE"
assert_contains "$DEMO_SCRIPT" "go build -o"
assert_contains "$DEMO_SCRIPT" "temporary Git repository"
assert_contains "$DEMO_TRANSCRIPT" "SetupProof terminal demo"
assert_contains "$DEMO_TRANSCRIPT" "result=passed"
assert_contains "$LABELS" "good first issue"
assert_contains "$LABELS" "runner"
assert_not_contains "$LABELS" "launch"
assert_contains "$BUG_TEMPLATE" 'labels: "bug"'
assert_contains "$SUPPORT_TEMPLATE" 'labels: "support"'
assert_contains "$CONTRIBUTOR_TEMPLATE" "make fmt-check"
assert_contains "$RUNNER_TEMPLATE" "make fmt-check"
assert_contains "$PR_TEMPLATE" "make fmt-check"
assert_contains "$METADATA" "github-actions"
assert_contains "$ACTION" "without re-verifying its provenance"

for file in "$README" "$SUPPORT" "$AGENT_USAGE" "$LLMS" "$DEMO_README" "$DEMO_SCRIPT" "$DEMO_TRANSCRIPT"; do
  assert_not_contains "$file" "@""v1"
  assert_not_contains "$file" "npx setup""proof"
  assert_not_contains "$file" "npm install -g setup""proof"
  assert_not_contains "$file" "npm i -g setup""proof"
  assert_not_contains "$file" "brew install setup""proof"
  assert_not_contains "$file" "winget install setup""proof"
  assert_not_contains "$file" "choco install setup""proof"
  assert_not_contains "$file" "scoop install setup""proof"
  assert_not_contains "$file" "codex"
  assert_not_contains "$file" "chatgpt"
  assert_not_contains "$file" "openai"
  assert_not_contains "$file" "gpt-"
  assert_not_contains "$file" "generated by"
done

sh -n "$DEMO_SCRIPT"
(cd "$ROOT" && SETUPPROOF_DEMO_PAUSE=0 "$DEMO_SCRIPT" >"$DEMO_OUT" 2>&1) || {
  sed -n '1,240p' "$DEMO_OUT" >&2
  fail "demo failed"
}
grep -Fq 'result=passed' "$DEMO_OUT" || fail "demo did not pass"

printf 'launch polish checks passed\n'
