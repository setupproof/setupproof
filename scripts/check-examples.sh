#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
TMP_ROOT=${TMPDIR:-/tmp}/setupproof-example-tests-$$

trap 'rm -rf "$TMP_ROOT"' EXIT
mkdir -p "$TMP_ROOT"

fail() {
  printf 'check-examples: %s\n' "$*" >&2
  exit 1
}

assert_contains() {
  file=$1
  expected=$2
  if ! grep -Fq -- "$expected" "$file"; then
    printf 'missing expected text: %s\n--- %s ---\n' "$expected" "$file" >&2
    sed -n '1,220p' "$file" >&2
    exit 1
  fi
}

init_git_repo() {
  dir=$1
  (
    cd "$dir"
    git init >/dev/null
    git add .
    git -c user.name='SetupProof Tests' -c user.email='setupproof@example.invalid' commit -m initial >/dev/null
  )
}

EXAMPLE_MARKDOWN='
examples/node-npm/README.md
examples/python-pip/README.md
examples/docker-compose/README.md
examples/monorepo/docs/web.md
examples/monorepo/docs/api.md
examples/go/README.md
examples/rust/README.md
'

BIN="$TMP_ROOT/setupproof"
(
  cd "$ROOT"
  go build -o "$BIN" ./cmd/setupproof
)

(
  cd "$ROOT"
  "$BIN" --list $EXAMPLE_MARKDOWN > "$TMP_ROOT/list.txt"
  "$BIN" review $EXAMPLE_MARKDOWN > "$TMP_ROOT/review.txt"
  "$BIN" --dry-run --json --require-blocks $EXAMPLE_MARKDOWN > "$TMP_ROOT/plan.json"
)

for id in node-npm-test python-pip-test docker-compose-smoke web-package api-service go-test rust-cargo-test; do
  assert_contains "$TMP_ROOT/list.txt" "id=$id"
  assert_contains "$TMP_ROOT/review.txt" "#$id"
  assert_contains "$TMP_ROOT/plan.json" "\"id\":\"$id\""
done

assert_contains "$ROOT/examples/docker-compose/compose.yaml" "@sha256:"
assert_contains "$ROOT/examples/docker-compose/README.md" "@sha256:"

GO_PLAN_EXAMPLE="$TMP_ROOT/go-plan-example"
mkdir -p "$GO_PLAN_EXAMPLE"
cp -R "$ROOT/examples/go/." "$GO_PLAN_EXAMPLE"
init_git_repo "$GO_PLAN_EXAMPLE"
(
  cd "$GO_PLAN_EXAMPLE"
  "$BIN" --dry-run --json --require-blocks > "$TMP_ROOT/no-config-plan.json"
)
assert_contains "$TMP_ROOT/no-config-plan.json" '"files":["README.md"]'
assert_contains "$TMP_ROOT/no-config-plan.json" '"id":"go-test"'

MONOREPO_PLAN_EXAMPLE="$TMP_ROOT/monorepo-plan-example"
mkdir -p "$MONOREPO_PLAN_EXAMPLE"
cp -R "$ROOT/examples/monorepo/." "$MONOREPO_PLAN_EXAMPLE"
init_git_repo "$MONOREPO_PLAN_EXAMPLE"
(
  cd "$MONOREPO_PLAN_EXAMPLE"
  "$BIN" --dry-run --json > "$TMP_ROOT/config-plan.json"
)
assert_contains "$TMP_ROOT/config-plan.json" '"configPath":"setupproof.yml"'
assert_contains "$TMP_ROOT/config-plan.json" '"requireBlocks":true'
assert_contains "$TMP_ROOT/config-plan.json" '"files":["docs/web.md","docs/api.md"]'

DOCKER_COMPOSE_PLAN_EXAMPLE="$TMP_ROOT/docker-compose-plan-example"
mkdir -p "$DOCKER_COMPOSE_PLAN_EXAMPLE"
cp -R "$ROOT/examples/docker-compose/." "$DOCKER_COMPOSE_PLAN_EXAMPLE"
init_git_repo "$DOCKER_COMPOSE_PLAN_EXAMPLE"
(
  cd "$DOCKER_COMPOSE_PLAN_EXAMPLE"
  "$BIN" --dry-run --json --require-blocks > "$TMP_ROOT/docker-compose-plan.json"
)
assert_contains "$TMP_ROOT/docker-compose-plan.json" '"configPath":"setupproof.yml"'
assert_contains "$TMP_ROOT/docker-compose-plan.json" '"files":["README.md"]'
assert_contains "$TMP_ROOT/docker-compose-plan.json" '"allow":["DOCKER_HOST"]'

GO_EXAMPLE="$TMP_ROOT/go-example"
mkdir -p "$GO_EXAMPLE"
cp -R "$ROOT/examples/go/." "$GO_EXAMPLE"
init_git_repo "$GO_EXAMPLE"
(
  cd "$GO_EXAMPLE"
  "$BIN" --require-blocks --no-color --no-glyphs > "$TMP_ROOT/go-execution.txt"
)
assert_contains "$TMP_ROOT/go-execution.txt" 'result=passed'
assert_contains "$TMP_ROOT/go-execution.txt" 'README.md#go-test'

if command -v python3 >/dev/null 2>&1 && python3 -m venv "$TMP_ROOT/python-venv-probe" >/dev/null 2>&1; then
  PYTHON_EXAMPLE="$TMP_ROOT/python-example"
  mkdir -p "$PYTHON_EXAMPLE"
  cp -R "$ROOT/examples/python-pip/." "$PYTHON_EXAMPLE"
  init_git_repo "$PYTHON_EXAMPLE"
  (
    cd "$PYTHON_EXAMPLE"
    "$BIN" --require-blocks --no-color --no-glyphs > "$TMP_ROOT/python-execution.txt"
  )
  assert_contains "$TMP_ROOT/python-execution.txt" 'result=passed'
  assert_contains "$TMP_ROOT/python-execution.txt" 'README.md#python-pip-test'
else
  printf 'python example execution skipped: python3 venv support not found\n' >&2
fi

if command -v npm >/dev/null 2>&1; then
  MONOREPO_EXAMPLE="$TMP_ROOT/monorepo-example"
  mkdir -p "$MONOREPO_EXAMPLE"
  cp -R "$ROOT/examples/monorepo/." "$MONOREPO_EXAMPLE"
  init_git_repo "$MONOREPO_EXAMPLE"
  (
    cd "$MONOREPO_EXAMPLE"
    "$BIN" --no-color --no-glyphs > "$TMP_ROOT/monorepo-execution.txt"
  )
  assert_contains "$TMP_ROOT/monorepo-execution.txt" 'result=passed'
  assert_contains "$TMP_ROOT/monorepo-execution.txt" 'docs/web.md#web-package'
  assert_contains "$TMP_ROOT/monorepo-execution.txt" 'docs/api.md#api-service'
else
  printf 'configured execution smoke skipped: npm not found\n' >&2
fi

printf 'example checks passed\n'
