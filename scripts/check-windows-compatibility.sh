#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
cd "$ROOT"

go test ./internal/platform
go test -run 'TestBuildRejectsNativeWindowsShellLanguages|TestBuildRejectsUnsupportedMarkedLanguage' ./internal/planning
go test -run 'TestInstallDocCISnippetsAndDeferredClaims|TestInitWorkflowPrintsConservativeWorkflowOnly' ./internal/cli
sh scripts/check-docs.sh

printf 'windows compatibility checks passed\n'
