#!/usr/bin/env bash
set -euo pipefail

summary_limit_bytes="${SETUPPROOF_SUMMARY_LIMIT_BYTES:-60000}"

main() {
  local runner_temp
  runner_temp="${RUNNER_TEMP:-${TMPDIR:-/tmp}}"
  mkdir -p "$runner_temp"

  local mode cli_version cli_path cli_sha256 files config runner timeout network
  local require_blocks fail_fast include_untracked keep_workspace no_color no_glyphs
  local report_json report_file cli status stdout_capture summary_report

  mode="$(input MODE run)"
  cli_version="$(input CLI_VERSION "")"
  cli_path="$(input CLI_PATH "")"
  cli_sha256="$(input CLI_SHA256 "")"
  files="$(input FILES README.md)"
  config="$(input CONFIG "")"
  runner="$(input RUNNER action-local)"
  timeout="$(input TIMEOUT "")"
  network="$(input NETWORK "")"
  report_json="$(input REPORT_JSON "")"
  report_file="$(input REPORT_FILE "")"

  require_blocks="$(bool_input REQUIRE_BLOCKS true)"
  fail_fast="$(bool_input FAIL_FAST false)"
  include_untracked="$(bool_input INCLUDE_UNTRACKED false)"
  keep_workspace="$(bool_input KEEP_WORKSPACE false)"
  no_color="$(bool_input NO_COLOR true)"
  no_glyphs="$(bool_input NO_GLYPHS true)"

  case "$mode" in
    run|review) ;;
    *) fail "mode must be run or review" ;;
  esac
  if [ -n "$cli_version" ]; then
    validate_cli_version "$cli_version"
  fi
  if [ -n "$cli_sha256" ]; then
    validate_sha256 "$cli_sha256"
    if [ -n "$cli_path" ]; then
      fail "cli-sha256 can only be used with cli-version downloads"
    fi
  fi

  if [ "$mode" = "review" ]; then
    if [ "$fail_fast" = "true" ] || [ "$keep_workspace" = "true" ] || [ -n "$report_json" ] || [ -n "$report_file" ]; then
      fail "review mode does not accept fail-fast, keep-workspace, report-json, or report-file"
    fi
  fi

  if [ "$mode" = "run" ] && [ -z "$report_json" ]; then
    report_json="$runner_temp/setupproof-report.json"
  fi

  resolve_cli "$cli_path" "$cli_version" "$cli_sha256" "$runner_temp"
  cli="$SETUPPROOF_CLI"
  if [ -n "$cli_version" ]; then
    verify_cli_version "$cli" "$cli_version"
  fi

  build_command "$cli" "$mode" "$files" "$config" "$runner" "$timeout" "$network" \
    "$require_blocks" "$fail_fast" "$include_untracked" "$keep_workspace" \
    "$no_color" "$no_glyphs" "$report_json" "$report_file"

  write_outputs "$report_json" "$report_file"

  stdout_capture="$runner_temp/setupproof-action-stdout.json"
  status=0
  run_setup_proof "$mode" "$stdout_capture" || status=$?

  summary_report="$report_json"
  if [ ! -s "$summary_report" ] && [ -s "$stdout_capture" ]; then
    summary_report="$stdout_capture"
  fi
  write_step_summary "$cli" "$mode" "$status" "$summary_report" "$report_json" "$files"

  exit "$status"
}

input() {
  local name="$1"
  local default_value="$2"
  local var="SETUPPROOF_INPUT_${name}"
  local value="${!var-}"
  if [ -z "$value" ]; then
    printf '%s' "$default_value"
  else
    printf '%s' "$value"
  fi
}

bool_input() {
  local name="$1"
  local default_value="$2"
  local value
  value="$(input "$name" "$default_value")"
  value="$(printf '%s' "$value" | tr '[:upper:]' '[:lower:]')"
  case "$value" in
    true|false) printf '%s' "$value" ;;
    *) fail "$name must be true or false" ;;
  esac
}

resolve_cli() {
  local cli_path="$1"
  local cli_version="$2"
  local cli_sha256="$3"
  local runner_temp="$4"
  if [ -n "$cli_path" ]; then
    # Caller-provided CLI paths are a workflow trust boundary: this action
    # validates executability and optional version only, then runs that binary.
    if [ ! -f "$cli_path" ]; then
      fail "cli-path does not exist: $cli_path"
    fi
    if [ ! -x "$cli_path" ]; then
      fail "cli-path is not executable: $cli_path"
    fi
    SETUPPROOF_CLI="$cli_path"
    return 0
  fi
  if [ -z "$cli_version" ]; then
    fail "cli-path is required for source-tree use; set cli-version only when release archives are published"
  fi
  download_cli "$cli_version" "$runner_temp" "$cli_sha256"
}

download_cli() {
  local cli_version="$1"
  local runner_temp="$2"
  local cli_sha256="$3"
  local os arch archive_version archive_name checksum_name base_url work_dir
  local archive_path checksum_path extract_dir

  os="$(platform_os)"
  arch="$(platform_arch)"
  archive_version="$(strip_v "$cli_version")"
  archive_name="setupproof_${archive_version}_${os}_${arch}.tar.gz"
  checksum_name="setupproof_${archive_version}_checksums.txt"
  base_url="$(release_base_url "$cli_version")"
  work_dir="$runner_temp/setupproof-action-${archive_version}-${os}-${arch}"
  archive_path="$work_dir/$archive_name"
  checksum_path="$work_dir/$checksum_name"
  extract_dir="$work_dir/extract"

  rm -rf "$work_dir"
  mkdir -p "$work_dir" "$extract_dir"

  curl -fsSL "$base_url/$checksum_name" -o "$checksum_path"
  curl -fsSL "$base_url/$archive_name" -o "$archive_path"
  verify_checksum "$work_dir" "$archive_name" "$checksum_path"
  if [ -n "$cli_sha256" ]; then
    verify_exact_checksum "$archive_path" "$cli_sha256"
  fi

  tar -xzf "$archive_path" -C "$extract_dir"
  locate_extracted_cli "$extract_dir"
}

platform_os() {
  case "$(uname -s)" in
    Linux) printf 'linux' ;;
    Darwin) printf 'darwin' ;;
    *) fail "unsupported action runner OS: $(uname -s)" ;;
  esac
}

platform_arch() {
  case "$(uname -m)" in
    x86_64|amd64) printf 'amd64' ;;
    arm64|aarch64) printf 'arm64' ;;
    *) fail "unsupported action runner architecture: $(uname -m)" ;;
  esac
}

release_base_url() {
  local cli_version="$1"
  local server repo
  server="${GITHUB_SERVER_URL:-https://github.com}"
  repo="${GITHUB_ACTION_REPOSITORY:-${GITHUB_REPOSITORY:-}}"
  if [ -z "$repo" ]; then
    fail "GITHUB_ACTION_REPOSITORY or GITHUB_REPOSITORY is required to download the SetupProof CLI"
  fi
  printf '%s/%s/releases/download/%s' "${server%/}" "$repo" "$cli_version"
}

verify_checksum() {
  local work_dir="$1"
  local archive_name="$2"
  local checksum_path="$3"
  local selected

  selected="$(awk -v name="$archive_name" 'NF == 2 && $2 == name { print; exit }' "$checksum_path")"
  if [ -z "$selected" ]; then
    fail "checksum manifest does not contain $archive_name"
  fi
  printf '%s\n' "$selected" > "$work_dir/checksums.selected"
  if command -v sha256sum >/dev/null 2>&1; then
    if ! (cd "$work_dir" && sha256sum -c checksums.selected >/dev/null); then
      fail "checksum verification failed for $archive_name"
    fi
  elif command -v shasum >/dev/null 2>&1; then
    if ! (cd "$work_dir" && shasum -a 256 -c checksums.selected >/dev/null); then
      fail "checksum verification failed for $archive_name"
    fi
  else
    fail "sha256sum or shasum is required to verify the SetupProof archive"
  fi
}

verify_exact_checksum() {
  local archive_path="$1"
  local expected="$2"
  local actual

  if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "$archive_path" | awk '{print $1}')"
  elif command -v shasum >/dev/null 2>&1; then
    actual="$(shasum -a 256 "$archive_path" | awk '{print $1}')"
  else
    fail "sha256sum or shasum is required to verify the SetupProof archive"
  fi
  if [ "$actual" != "$expected" ]; then
    fail "archive checksum does not match cli-sha256"
  fi
}

locate_extracted_cli() {
  local extract_dir="$1"
  local candidate found
  for candidate in "$extract_dir/setupproof" "$extract_dir/bin/setupproof"; do
    if [ -f "$candidate" ] && [ -x "$candidate" ]; then
      SETUPPROOF_CLI="$candidate"
      return 0
    fi
  done
  found="$(find "$extract_dir" -type f -name setupproof -perm -111 | head -n 1 || true)"
  if [ -z "$found" ]; then
    fail "SetupProof archive did not contain an executable setupproof binary"
  fi
  SETUPPROOF_CLI="$found"
}

verify_cli_version() {
  local cli="$1"
  local expected="$2"
  local line got
  line="$("$cli" --version)"
  case "$line" in
    "setupproof "*) got="${line#setupproof }" ;;
    *) fail "unexpected setupproof --version output: $line" ;;
  esac
  if [ "$(strip_v "$got")" != "$(strip_v "$expected")" ]; then
    fail "SetupProof CLI version $got does not match pinned version $expected"
  fi
}

strip_v() {
  local value="$1"
  printf '%s' "${value#v}"
}

validate_cli_version() {
  local value="$1"
  if [ "${#value}" -gt 128 ]; then
    fail "cli-version is too long"
  fi
  case "$value" in
    v[0-9]*|[0-9]*) ;;
    *) fail "cli-version must start with an optional v followed by a digit" ;;
  esac
  case "$value" in
    *[!abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._+-]*)
      fail "cli-version contains unsupported characters"
      ;;
  esac
  case "$value" in
    *..*) fail "cli-version must not contain empty dot segments" ;;
  esac
  if ! [[ "$value" =~ ^v?[0-9]+(\.[0-9]+){0,2}([._+-][A-Za-z0-9]+)*$ ]]; then
    fail "cli-version must look like v1, v1.2, v1.2.3, or a release suffix such as v1.2.3-rc.1"
  fi
}

validate_sha256() {
  local value="$1"
  if [ "${#value}" -ne 64 ]; then
    fail "cli-sha256 must be a 64-character lowercase hex SHA-256 digest"
  fi
  case "$value" in
    *[!0123456789abcdef]*)
      fail "cli-sha256 must be a 64-character lowercase hex SHA-256 digest"
      ;;
  esac
}

declare -a SETUPPROOF_COMMAND=()

build_command() {
  local cli="$1"
  local mode="$2"
  local files="$3"
  local config="$4"
  local runner="$5"
  local timeout="$6"
  local network="$7"
  local require_blocks="$8"
  local fail_fast="$9"
  local include_untracked="${10}"
  local keep_workspace="${11}"
  local no_color="${12}"
  local no_glyphs="${13}"
  local report_json="${14}"
  local report_file="${15}"

  SETUPPROOF_COMMAND=("$cli")
  if [ "$mode" = "review" ]; then
    SETUPPROOF_COMMAND+=("review")
  else
    SETUPPROOF_COMMAND+=("--json" "--report-json" "$report_json")
    if [ -n "$report_file" ]; then
      SETUPPROOF_COMMAND+=("--report-file" "$report_file")
    fi
  fi

  SETUPPROOF_COMMAND+=("--runner=$runner")
  if [ -n "$config" ]; then
    SETUPPROOF_COMMAND+=("--config" "$config")
  fi
  if [ -n "$timeout" ]; then
    SETUPPROOF_COMMAND+=("--timeout=$timeout")
  fi
  if [ -n "$network" ]; then
    case "$network" in
      true|false) SETUPPROOF_COMMAND+=("--network=$network") ;;
      *) fail "NETWORK must be true, false, or empty" ;;
    esac
  fi
  if [ "$require_blocks" = "true" ]; then
    SETUPPROOF_COMMAND+=("--require-blocks")
  fi
  if [ "$fail_fast" = "true" ]; then
    SETUPPROOF_COMMAND+=("--fail-fast")
  fi
  if [ "$include_untracked" = "true" ]; then
    SETUPPROOF_COMMAND+=("--include-untracked")
  fi
  if [ "$keep_workspace" = "true" ]; then
    SETUPPROOF_COMMAND+=("--keep-workspace")
  fi
  if [ "$no_color" = "true" ]; then
    SETUPPROOF_COMMAND+=("--no-color")
  fi
  if [ "$no_glyphs" = "true" ]; then
    SETUPPROOF_COMMAND+=("--no-glyphs")
  fi

  append_files "$files"
}

append_files() {
  local files="$1"
  local file count
  count=0
  while IFS= read -r file || [ -n "$file" ]; do
    file="${file%$'\r'}"
    if [ -z "$file" ]; then
      continue
    fi
    SETUPPROOF_COMMAND+=("$file")
    count=$((count + 1))
  done <<< "$files"
  if [ "$count" -eq 0 ]; then
    fail "files input must contain at least one Markdown path"
  fi
}

run_setup_proof() {
  local mode="$1"
  local stdout_capture="$2"
  local token status
  token="$(stop_token)"

  printf '::stop-commands::%s\n' "$token"
  set +e
  if [ "$mode" = "run" ]; then
    "${SETUPPROOF_COMMAND[@]}" > "$stdout_capture"
  else
    "${SETUPPROOF_COMMAND[@]}"
  fi
  status=$?
  set -e
  printf '::%s::\n' "$token"
  return "$status"
}

stop_token() {
  if command -v uuidgen >/dev/null 2>&1; then
    uuidgen | tr '[:upper:]' '[:lower:]'
  elif command -v od >/dev/null 2>&1 && [ -r /dev/urandom ]; then
    printf 'setupproof-%s\n' "$(od -An -N16 -tx1 /dev/urandom | tr -d ' \n')"
  elif command -v openssl >/dev/null 2>&1; then
    printf 'setupproof-%s\n' "$(openssl rand -hex 16)"
  else
    fail "could not generate a random stop-commands token"
  fi
}

write_outputs() {
  local report_json="$1"
  local report_file="$2"
  if [ -z "${GITHUB_OUTPUT:-}" ]; then
    return 0
  fi
  write_output_value "report-json" "$report_json"
  write_output_value "report-file" "$report_file"
}

write_output_value() {
  local name="$1"
  local value="$2"
  case "$value" in
    *$'\n'*|*$'\r'*)
      fail "$name output path must not contain newlines"
      ;;
  esac
  printf '%s=%s\n' "$name" "$value" >> "$GITHUB_OUTPUT"
}

write_step_summary() {
  local cli="$1"
  local mode="$2"
  local status="$3"
  local summary_report="$4"
  local report_json="$5"
  local files="$6"
  local summary_tmp
  local summary_input

  if [ -z "${GITHUB_STEP_SUMMARY:-}" ]; then
    return 0
  fi

  summary_tmp="${RUNNER_TEMP:-${TMPDIR:-/tmp}}/setupproof-step-summary.md"
  if [ "${SETUPPROOF_DISABLE_PYTHON_SUMMARY:-}" != "1" ] && command -v python3 >/dev/null 2>&1; then
    SETUPPROOF_SUMMARY_FILES="$files" SETUPPROOF_SUMMARY_LIMIT_BYTES="$summary_limit_bytes" \
      python3 - "$mode" "$status" "$summary_report" "$report_json" > "$summary_tmp" <<'PY'
import json
import os
import sys

mode = sys.argv[1]
status = int(sys.argv[2])
summary_report = sys.argv[3]
report_json = sys.argv[4]
files = os.environ.get("SETUPPROOF_SUMMARY_FILES", "")
limit = int(os.environ.get("SETUPPROOF_SUMMARY_LIMIT_BYTES", "60000"))

def md(value, max_len=160):
    text = str(value if value is not None else "")
    text = text.replace("\r", "").replace("\n", "<br>").replace("|", "\\|")
    if len(text) > max_len:
        return text[: max_len - 3] + "..."
    return text

lines = ["## SetupProof", ""]

if mode == "review":
    lines.extend([
        "- mode: review",
        f"- exit code: {status}",
        "- report JSON: not produced in review mode",
        "",
        "Review mode is non-executing.",
    ])
else:
    report = None
    if summary_report and os.path.exists(summary_report) and os.path.getsize(summary_report) > 0:
        try:
            with open(summary_report, "r", encoding="utf-8") as handle:
                report = json.load(handle)
        except Exception:
            report = None

    if not report:
        lines.extend([
            "- result: unavailable",
            f"- exit code: {status}",
            f"- report JSON: `{md(report_json, 240)}`",
            "",
            "SetupProof did not produce a readable JSON report.",
        ])
    else:
        blocks = report.get("blocks") or []
        warnings = report.get("warnings") or []
        failed = [b for b in blocks if b.get("result") != "passed"]
        selected = failed[:15] if failed else blocks[:15]
        lines.extend([
            f"- result: {md(report.get('result', 'unknown'))}",
            f"- exit code: {report.get('exitCode', status)}",
            f"- duration ms: {report.get('durationMs', 0)}",
            f"- report JSON: `{md(report_json, 240)}`",
            f"- files: {md(', '.join(report.get('files') or []), 240)}",
            "",
        ])
        if warnings:
            lines.append("### Warnings")
            lines.append("")
            for warning in warnings[:10]:
                lines.append(f"- {md(warning, 240)}")
            if len(warnings) > 10:
                lines.append(f"- ... {len(warnings) - 10} more")
            lines.append("")
        if blocks:
            title = "Failing Blocks" if failed else "Blocks"
            lines.extend([
                f"### {title}",
                "",
                "| Block | Result | Exit | Reason |",
                "| --- | --- | ---: | --- |",
            ])
            for block in selected:
                block_id = block.get("qualifiedId") or f"{block.get('file', '')}#{block.get('id', '')}"
                lines.append(
                    f"| {md(block_id)} | {md(block.get('result', 'unknown'), 80)} | "
                    f"{block.get('exitCode', 0)} | {md(block.get('reason', ''), 120)} |"
                )
            omitted = len((failed if failed else blocks)) - len(selected)
            if omitted > 0:
                lines.append(f"| ... {omitted} more |  |  |  |")
            lines.append("")
        else:
            lines.append("No marked blocks were reported.")
            lines.append("")

if files and mode != "run":
    rendered_files = ", ".join([line for line in files.splitlines() if line])
    lines.extend(["", f"- files: {md(rendered_files, 240)}"])

text = "\n".join(lines).rstrip() + "\n"
encoded = text.encode("utf-8")
if len(encoded) > limit:
    suffix = "\n\n_Summary truncated by SetupProof Action._\n"
    cut = max(0, limit - len(suffix.encode("utf-8")))
    text = encoded[:cut].decode("utf-8", errors="ignore").rstrip() + suffix
sys.stdout.write(text)
PY
  else
    summary_input="/dev/null"
    if [ -n "$summary_report" ] && [ -s "$summary_report" ]; then
      summary_input="$summary_report"
    fi
    if ! "$cli" report --format=github-step-summary --mode "$mode" --status "$status" --report-json "$report_json" --files "$files" < "$summary_input" > "$summary_tmp"; then
      write_minimal_step_summary "$mode" "$status" "$report_json" > "$summary_tmp"
    fi
  fi

  if [ "$(wc -c < "$summary_tmp")" -gt "$summary_limit_bytes" ]; then
    local truncated
    local suffix
    local suffix_bytes
    local head_bytes
    truncated="${summary_tmp}.truncated"
    suffix=$'\n\n_Summary truncated by SetupProof Action._\n'
    suffix_bytes="$(printf '%s' "$suffix" | wc -c | tr -d ' ')"
    head_bytes=$((summary_limit_bytes - suffix_bytes))
    if [ "$head_bytes" -lt 0 ]; then
      head_bytes=0
    fi
    head -c "$head_bytes" "$summary_tmp" > "$truncated"
    printf '%s' "$suffix" >> "$truncated"
    mv "$truncated" "$summary_tmp"
  fi

  cat "$summary_tmp" >> "$GITHUB_STEP_SUMMARY"
}

write_minimal_step_summary() {
  local mode="$1"
  local status="$2"
  local report_json="$3"
  printf '## SetupProof\n\n'
  printf -- '- mode: %s\n' "$mode"
  printf -- '- exit code: %s\n' "$status"
  if [ -n "$report_json" ]; then
    printf -- '- report JSON: `%s`\n' "$report_json"
  fi
}

fail() {
  printf 'SetupProof Action: %s\n' "$*" >&2
  exit 2
}

main "$@"
