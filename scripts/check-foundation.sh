#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
cd "$ROOT"

fail() {
  printf 'check-foundation: %s\n' "$*" >&2
  exit 1
}

required_files="
docs/adr/0000-runtime-and-npm-package-strategy.md
docs/adr/0001-github-action-packaging-strategy.md
docs/adr/0002-shell-execution-semantics.md
docs/adr/0003-workspace-copy-semantics.md
docs/adr/0004-baseline-environment-semantics.md
docs/adr/0005-config-file-name-discovery.md
docs/adr/0006-json-report-plan-schema-boundaries.md
docs/adr/0007-no-argument-behavior.md
docs/adr/0008-marketplace-strategy.md
"

for file in $required_files; do
  if [ ! -f "$file" ]; then
    fail "missing required file: $file"
  fi
done

for adr in docs/adr/*.md; do
  if ! grep -q '^Status: Accepted$' "$adr"; then
    fail "ADR is not accepted: $adr"
  fi
done

git -C "$ROOT" ls-files | while IFS= read -r path; do
  case "$path" in
    .github/*|.gitignore)
      ;;
    .*/*)
      fail "tracked local-only artifact: $path"
      ;;
  esac
  case "$path" in
    *.md)
      case "$path" in
        README.md|CHANGELOG.md|CODE_OF_CONDUCT.md|CONTRIBUTING.md|SECURITY.md|SUPPORT.md|docs/*|examples/*|.github/*|internal/*/testdata/*.md)
          ;;
        *)
          fail "tracked Markdown file outside public doc locations: $path"
          ;;
      esac
      ;;
  esac
  name=${path##*/}
  case "$name" in
    *review*.md|*notes*.md|*private*.md|*internal*.md)
      case "$path" in
        .github/*|docs/*|examples/*)
          ;;
        *)
          fail "tracked local-only artifact: $path"
          ;;
      esac
      ;;
  esac
done

printf 'foundation checks passed\n'
