#!/usr/bin/env bash
set -euo pipefail

ROOT="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
ACTION_SCRIPT="$ROOT/scripts/github-action.sh"
TMP_ROOT="${TMPDIR:-/tmp}/setupproof-action-tests-$$"

trap 'rm -rf "$TMP_ROOT"' EXIT
mkdir -p "$TMP_ROOT"

fail() {
  printf 'check-github-action: %s\n' "$*" >&2
  exit 1
}

assert_contains() {
  local file="$1"
  local expected="$2"
  if ! grep -Fq -- "$expected" "$file"; then
    printf 'missing expected text: %s\n--- %s ---\n' "$expected" "$file" >&2
    sed -n '1,220p' "$file" >&2
    exit 1
  fi
}

assert_not_contains() {
  local file="$1"
  local unexpected="$2"
  if grep -Fq -- "$unexpected" "$file"; then
    printf 'unexpected text: %s\n--- %s ---\n' "$unexpected" "$file" >&2
    sed -n '1,220p' "$file" >&2
    exit 1
  fi
}

make_fake_cli() {
  local dir="$1"
  mkdir -p "$dir"
  cat > "$dir/setupproof" <<'SH'
#!/usr/bin/env bash
set -euo pipefail

if [ "${1:-}" = "--version" ]; then
  printf 'setupproof %s\n' "${FAKE_CLI_VERSION:-0.1.0}"
  exit 0
fi

if [ "${1:-}" = "report" ]; then
  shift
  mode="run"
  status="0"
  report_json=""
  files=""
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --format)
        shift 2
        ;;
      --format=*)
        shift
        ;;
      --mode)
        mode="$2"
        shift 2
        ;;
      --mode=*)
        mode="${1#--mode=}"
        shift
        ;;
      --status)
        status="$2"
        shift 2
        ;;
      --status=*)
        status="${1#--status=}"
        shift
        ;;
      --report-json)
        report_json="$2"
        shift 2
        ;;
      --report-json=*)
        report_json="${1#--report-json=}"
        shift
        ;;
      --files)
        files="$2"
        shift 2
        ;;
      --files=*)
        files="${1#--files=}"
        shift
        ;;
      *)
        shift
        ;;
    esac
  done
  report="$(cat)"
  printf '## SetupProof\n\n'
  if [ "$mode" = "review" ]; then
    printf -- '- mode: review\n'
    printf -- '- exit code: %s\n' "$status"
    printf -- '- report JSON: not produced in review mode\n\n'
    printf 'Review mode is non-executing.\n'
    if [ -n "$files" ]; then
      printf '\n- files: %s\n' "$(printf '%s' "$files" | tr '\n' ',' | sed 's/,$//')"
    fi
    exit 0
  fi
  if [ -z "$report" ]; then
    printf -- '- result: unavailable\n'
    printf -- '- exit code: %s\n' "$status"
    printf -- '- report JSON: `%s`\n\n' "$report_json"
    printf 'SetupProof did not produce a readable JSON report.\n'
    exit 0
  fi
  result="$(printf '%s' "$report" | sed -n 's/.*"result":"\([^"]*\)".*/\1/p')"
  exit_code="$(printf '%s' "$report" | sed -n 's/.*"exitCode":\([0-9][0-9]*\).*/\1/p')"
  printf -- '- result: %s\n' "${result:-unknown}"
  printf -- '- exit code: %s\n' "${exit_code:-$status}"
  printf -- '- report JSON: `%s`\n\n' "$report_json"
  printf '### Failing Blocks\n\n'
  printf '| Block | Result | Exit | Reason |\n'
  printf '| --- | --- | ---: | --- |\n'
  { printf '%s' "$report" | grep -o 'README.md#b[0-9][0-9]*' || true; } | head -n 15 | while IFS= read -r block_id; do
    printf '| %s | failed | 1 | exit-code |\n' "$block_id"
  done
  exit 0
fi

: > "${FAKE_ARGS_FILE:?}"
for arg in "$@"; do
  printf '%s\n' "$arg" >> "$FAKE_ARGS_FILE"
done

printf '::error::raw command output must be stopped\n' >&2

report_path=""
previous=""
for arg in "$@"; do
  if [ "$previous" = "--report-json" ]; then
    report_path="$arg"
    previous=""
    continue
  fi
  case "$arg" in
    --report-json=*) report_path="${arg#--report-json=}" ;;
    --report-json) previous="--report-json" ;;
  esac
done

if [ -n "$report_path" ]; then
  mkdir -p "$(dirname "$report_path")"
  blocks="${FAKE_BLOCKS:-1}"
  result="${FAKE_RESULT:-passed}"
  exit_code="${FAKE_EXIT_CODE:-0}"
  {
    printf '{"kind":"report","schemaVersion":"1.0.0","setupproofVersion":"%s",' "${FAKE_CLI_VERSION:-0.1.0}"
    printf '"startedAt":"2026-04-24T00:00:00Z","durationMs":12,'
    printf '"result":"%s","exitCode":%s,' "$result" "$exit_code"
    printf '"invocation":{"args":[]},"workspace":{"mode":"temporary","source":"tracked-plus-modified","includedUntracked":false},'
    printf '"runner":{"kind":"action-local","workspace":"temporary","networkPolicy":"host","networkEnforced":false},'
    printf '"files":["README.md"],"warnings":[],"blocks":['
    i=1
    while [ "$i" -le "$blocks" ]; do
      if [ "$i" -gt 1 ]; then
        printf ','
      fi
      block_result="passed"
      block_exit=0
      reason=""
      if [ "$result" != "passed" ]; then
        block_result="failed"
        block_exit=1
        reason="exit-code"
      fi
      printf '{"id":"b%s","qualifiedId":"README.md#b%s","file":"README.md","line":%s,' "$i" "$i" "$i"
      printf '"language":"sh","shell":"sh","source":"true","strict":true,"stdin":"closed","tty":false,'
      printf '"stateMode":"shared","isolated":false,"runner":"action-local","timeout":"120s","timeoutMs":120000,'
      printf '"result":"%s","exitCode":%s,"reason":"%s","durationMs":1,' "$block_result" "$block_exit" "$reason"
      printf '"stdoutTail":"","stderrTail":"","truncated":{"stdout":false,"stderr":false}}'
      i=$((i + 1))
    done
    printf ']}'
  } > "$report_path"
fi

exit "${FAKE_EXIT_CODE:-0}"
SH
  chmod +x "$dir/setupproof"
}

run_action() {
  local dir="$1"
  shift
  mkdir -p "$dir/runner"
  : > "$dir/output"
  : > "$dir/summary"
  set +e
  (
    export RUNNER_TEMP="$dir/runner"
    export GITHUB_OUTPUT="$dir/output"
    export GITHUB_STEP_SUMMARY="$dir/summary"
    "$@" "$ACTION_SCRIPT"
  ) > "$dir/stdout" 2> "$dir/stderr"
  local status=$?
  set -e
  printf '%s' "$status" > "$dir/status"
}

platform_os() {
  case "$(uname -s)" in
    Linux) printf 'linux' ;;
    Darwin) printf 'darwin' ;;
    *) fail "unsupported test OS: $(uname -s)" ;;
  esac
}

platform_arch() {
  case "$(uname -m)" in
    x86_64|amd64) printf 'amd64' ;;
    arm64|aarch64) printf 'arm64' ;;
    *) fail "unsupported test architecture: $(uname -m)" ;;
  esac
}

sha256_file() {
  local file="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file" | awk '{print $1}'
  else
    shasum -a 256 "$file" | awk '{print $1}'
  fi
}

test_local_cli_input_mapping() {
  local dir="$TMP_ROOT/input-mapping"
  local fake="$dir/fake"
  mkdir -p "$dir"
  make_fake_cli "$fake"

  run_action "$dir" env \
    FAKE_ARGS_FILE="$dir/args" \
    SETUPPROOF_INPUT_CLI_PATH="$fake/setupproof" \
    SETUPPROOF_INPUT_CLI_VERSION="v0.1.0" \
    SETUPPROOF_INPUT_FILES=$'README.md\ndocs/start.md' \
    SETUPPROOF_INPUT_MODE="run" \
    SETUPPROOF_INPUT_CONFIG="setupproof.yml" \
    SETUPPROOF_INPUT_RUNNER="action-local" \
    SETUPPROOF_INPUT_TIMEOUT="90s" \
    SETUPPROOF_INPUT_NETWORK="true" \
    SETUPPROOF_INPUT_REQUIRE_BLOCKS="true" \
    SETUPPROOF_INPUT_FAIL_FAST="true" \
    SETUPPROOF_INPUT_INCLUDE_UNTRACKED="false" \
    SETUPPROOF_INPUT_KEEP_WORKSPACE="false" \
    SETUPPROOF_INPUT_NO_COLOR="true" \
    SETUPPROOF_INPUT_NO_GLYPHS="true" \
    SETUPPROOF_INPUT_REPORT_JSON="$dir/report.json" \
    SETUPPROOF_INPUT_REPORT_FILE="$dir/report.md"

  [ "$(cat "$dir/status")" = "0" ] || fail "input mapping action failed"
  cat > "$dir/expected-args" <<EOF
--json
--report-json
$dir/report.json
--report-file
$dir/report.md
--runner=action-local
--config
setupproof.yml
--timeout=90s
--network=true
--require-blocks
--fail-fast
--no-color
--no-glyphs
README.md
docs/start.md
EOF
  diff -u "$dir/expected-args" "$dir/args"
  assert_contains "$dir/output" "report-json=$dir/report.json"
  assert_contains "$dir/output" "report-file=$dir/report.md"
  assert_contains "$dir/summary" "result: passed"
  assert_contains "$dir/stdout" "::stop-commands::"
  assert_contains "$dir/stderr" "::error::raw command output must be stopped"
}

test_failure_propagates_and_wraps_logs() {
  local dir="$TMP_ROOT/failure"
  local fake="$dir/fake"
  mkdir -p "$dir"
  make_fake_cli "$fake"

  run_action "$dir" env \
    FAKE_ARGS_FILE="$dir/args" \
    FAKE_EXIT_CODE="1" \
    FAKE_RESULT="failed" \
    SETUPPROOF_INPUT_CLI_PATH="$fake/setupproof" \
    SETUPPROOF_INPUT_FILES="README.md"

  [ "$(cat "$dir/status")" = "1" ] || fail "action did not propagate SetupProof exit code"
  assert_contains "$dir/summary" "result: failed"
  assert_contains "$dir/stdout" "::stop-commands::"
  assert_contains "$dir/stdout" "::"
}

test_review_mode_mapping() {
  local dir="$TMP_ROOT/review"
  local fake="$dir/fake"
  mkdir -p "$dir"
  make_fake_cli "$fake"

  run_action "$dir" env \
    FAKE_ARGS_FILE="$dir/args" \
    SETUPPROOF_INPUT_CLI_PATH="$fake/setupproof" \
    SETUPPROOF_INPUT_MODE="review" \
    SETUPPROOF_INPUT_FILES="README.md"

  [ "$(cat "$dir/status")" = "0" ] || fail "review mode failed"
  cat > "$dir/expected-args" <<'EOF'
review
--runner=action-local
--require-blocks
--no-color
--no-glyphs
README.md
EOF
  diff -u "$dir/expected-args" "$dir/args"
  assert_not_contains "$dir/args" "--json"
  assert_not_contains "$dir/args" "--report-json"
  assert_contains "$dir/summary" "mode: review"
  assert_contains "$dir/summary" "Review mode is non-executing."
}

test_requires_cli_path_or_explicit_version() {
  local dir="$TMP_ROOT/missing-cli"
  mkdir -p "$dir"

  run_action "$dir" env \
    SETUPPROOF_INPUT_FILES="README.md"

  [ "$(cat "$dir/status")" = "2" ] || fail "action without cli-path or cli-version did not fail setup"
  assert_contains "$dir/stderr" "cli-path is required for source-tree use"
}

test_download_and_checksum_handling() {
  local dir="$TMP_ROOT/download"
  local release_root="$dir/releases"
  local release_dir="$release_root/owner/repo/releases/download/v0.1.0"
  local payload="$dir/payload"
  local os arch archive checksum archive_name checksum_name
  mkdir -p "$release_dir" "$payload"
  make_fake_cli "$payload"

  os="$(platform_os)"
  arch="$(platform_arch)"
  archive_name="setupproof_0.1.0_${os}_${arch}.tar.gz"
  checksum_name="setupproof_0.1.0_checksums.txt"
  archive="$release_dir/$archive_name"
  checksum="$release_dir/$checksum_name"
  tar -czf "$archive" -C "$payload" setupproof
  printf '%s  %s\n' "$(sha256_file "$archive")" "$archive_name" > "$checksum"

  run_action "$dir/good" env \
    FAKE_ARGS_FILE="$dir/good/args" \
    FAKE_CLI_VERSION="0.1.0" \
    GITHUB_SERVER_URL="file://$release_root" \
    GITHUB_ACTION_REPOSITORY="owner/repo" \
    SETUPPROOF_INPUT_CLI_VERSION="v0.1.0" \
    SETUPPROOF_INPUT_CLI_SHA256="$(sha256_file "$archive")" \
    SETUPPROOF_INPUT_FILES="README.md"

  [ "$(cat "$dir/good/status")" = "0" ] || fail "downloaded action path failed"
  [ -s "$dir/good/args" ] || fail "downloaded CLI did not execute after checksum verification"

  printf '%064d  %s\n' 0 "$archive_name" > "$checksum"
  run_action "$dir/bad" env \
    FAKE_ARGS_FILE="$dir/bad/args" \
    FAKE_CLI_VERSION="0.1.0" \
    GITHUB_SERVER_URL="file://$release_root" \
    GITHUB_ACTION_REPOSITORY="owner/repo" \
    SETUPPROOF_INPUT_CLI_VERSION="v0.1.0" \
    SETUPPROOF_INPUT_FILES="README.md"

  [ "$(cat "$dir/bad/status")" = "2" ] || fail "bad checksum did not fail action setup"
  if [ -e "$dir/bad/args" ] && [ -s "$dir/bad/args" ]; then
    fail "CLI executed despite checksum failure"
  fi

  printf '%s  %s\n' "$(sha256_file "$archive")" "$archive_name" > "$checksum"
  run_action "$dir/bad-pinned" env \
    FAKE_ARGS_FILE="$dir/bad-pinned/args" \
    FAKE_CLI_VERSION="0.1.0" \
    GITHUB_SERVER_URL="file://$release_root" \
    GITHUB_ACTION_REPOSITORY="owner/repo" \
    SETUPPROOF_INPUT_CLI_VERSION="v0.1.0" \
    SETUPPROOF_INPUT_CLI_SHA256="$(printf '%064d' 0)" \
    SETUPPROOF_INPUT_FILES="README.md"

  [ "$(cat "$dir/bad-pinned/status")" = "2" ] || fail "bad pinned checksum did not fail action setup"
  assert_contains "$dir/bad-pinned/stderr" "archive checksum does not match cli-sha256"
}

test_rejects_unsafe_cli_version() {
  local dir="$TMP_ROOT/unsafe-version"
  mkdir -p "$dir"

  run_action "$dir/slash" env \
    SETUPPROOF_INPUT_CLI_VERSION="0.1.0/evil" \
    SETUPPROOF_INPUT_FILES="README.md"

  [ "$(cat "$dir/slash/status")" = "2" ] || fail "unsafe cli-version with slash did not fail action setup"
  assert_contains "$dir/slash/stderr" "cli-version contains unsupported characters"

  run_action "$dir/double-dot" env \
    SETUPPROOF_INPUT_CLI_VERSION="1.0..0" \
    SETUPPROOF_INPUT_FILES="README.md"
  [ "$(cat "$dir/double-dot/status")" = "2" ] || fail "double-dot cli-version did not fail action setup"
  assert_contains "$dir/double-dot/stderr" "cli-version must not contain empty dot segments"

  run_action "$dir/leading-dot" env \
    SETUPPROOF_INPUT_CLI_VERSION=".1.0.0" \
    SETUPPROOF_INPUT_FILES="README.md"
  [ "$(cat "$dir/leading-dot/status")" = "2" ] || fail "leading-dot cli-version did not fail action setup"
  assert_contains "$dir/leading-dot/stderr" "cli-version must start"

  run_action "$dir/leading-dash" env \
    SETUPPROOF_INPUT_CLI_VERSION="--evil" \
    SETUPPROOF_INPUT_FILES="README.md"
  [ "$(cat "$dir/leading-dash/status")" = "2" ] || fail "leading-dash cli-version did not fail action setup"
  assert_contains "$dir/leading-dash/stderr" "cli-version must start"

  long_version="1$(printf '%0130d' 0)"
  run_action "$dir/long" env \
    SETUPPROOF_INPUT_CLI_VERSION="$long_version" \
    SETUPPROOF_INPUT_FILES="README.md"
  [ "$(cat "$dir/long/status")" = "2" ] || fail "overlong cli-version did not fail action setup"
  assert_contains "$dir/long/stderr" "cli-version is too long"
}

test_summary_is_bounded() {
  local dir="$TMP_ROOT/summary"
  local fake="$dir/fake"
  mkdir -p "$dir"
  make_fake_cli "$fake"

  run_action "$dir" env \
    FAKE_ARGS_FILE="$dir/args" \
    FAKE_BLOCKS="500" \
    FAKE_EXIT_CODE="1" \
    FAKE_RESULT="failed" \
    SETUPPROOF_SUMMARY_LIMIT_BYTES="4096" \
    SETUPPROOF_INPUT_CLI_PATH="$fake/setupproof" \
    SETUPPROOF_INPUT_FILES="README.md"

  [ "$(cat "$dir/status")" = "1" ] || fail "bounded summary run should return fake failure"
  size="$(wc -c < "$dir/summary")"
  [ "$size" -le 4608 ] || fail "summary exceeded bounded budget: $size bytes"
  assert_contains "$dir/summary" "SetupProof"
  assert_contains "$dir/summary" "Failing Blocks"
}

test_summary_fallback_without_python_keeps_block_details() {
  local dir="$TMP_ROOT/summary-no-python"
  local fake="$dir/fake"
  mkdir -p "$dir"
  make_fake_cli "$fake"

  run_action "$dir" env \
    FAKE_ARGS_FILE="$dir/args" \
    FAKE_EXIT_CODE="1" \
    FAKE_RESULT="failed" \
    SETUPPROOF_DISABLE_PYTHON_SUMMARY="1" \
    SETUPPROOF_INPUT_CLI_PATH="$fake/setupproof" \
    SETUPPROOF_INPUT_FILES="README.md"

  [ "$(cat "$dir/status")" = "1" ] || fail "fallback summary run should return fake failure"
  assert_contains "$dir/summary" "Failing Blocks"
  assert_contains "$dir/summary" "README.md#b1"
  assert_contains "$dir/summary" "exit-code"
}

test_no_secret_default_workflow_shape() {
  assert_contains "$ROOT/action.yml" "default: action-local"
  assert_contains "$ROOT/action.yml" "default: \"true\""
  assert_contains "$ROOT/action.yml" "published release"
  assert_contains "$ROOT/action.yml" "trusted path"
  assert_contains "$ROOT/action.yml" "runs this binary as provided"
  assert_not_contains "$ROOT/action.yml" "default: v0.1.0"
  assert_not_contains "$ROOT/action.yml" "secrets."
  assert_not_contains "$ROOT/action.yml" "github-token"
  assert_not_contains "$ROOT/action.yml" "@""v1"

  assert_contains "$ROOT/.github/workflows/setupproof.yml" "pull_request:"
  assert_contains "$ROOT/.github/workflows/setupproof.yml" "permissions:"
  assert_contains "$ROOT/.github/workflows/setupproof.yml" "contents: read"
	  assert_contains "$ROOT/.github/workflows/setupproof.yml" "require-blocks: \"true\""
	  assert_contains "$ROOT/.github/workflows/setupproof.yml" "runs-on: ubuntu-24.04"
	  assert_contains "$ROOT/.github/workflows/setupproof.yml" "timeout-minutes: 10"
	  assert_contains "$ROOT/.github/workflows/setupproof.yml" "docs/adr/0009-github-actions-checkout-strategy.md"
  assert_not_contains "$ROOT/.github/workflows/setupproof.yml" "pull_request_target"
  assert_not_contains "$ROOT/.github/workflows/setupproof.yml" "secrets."
  assert_not_contains "$ROOT/.github/workflows/setupproof.yml" "github.token"
  assert_not_contains "$ROOT/.github/workflows/setupproof.yml" "@""v1"
}

test_local_cli_input_mapping
test_failure_propagates_and_wraps_logs
test_review_mode_mapping
test_requires_cli_path_or_explicit_version
test_download_and_checksum_handling
test_rejects_unsafe_cli_version
test_summary_is_bounded
test_summary_fallback_without_python_keeps_block_details
test_no_secret_default_workflow_shape

printf 'github action checks passed\n'
