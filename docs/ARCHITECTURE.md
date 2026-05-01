# Architecture

SetupProof has one main path:

```text
Markdown -> Plan -> Workspace Copy -> Runner -> Report
```

The split is intentional. Discovery and planning are non-executing. Execution
starts only after a plan exists.

## Packages

- `cmd/setupproof` contains the executable entry point.
- `internal/cli` parses command-line arguments and connects commands to the
  rest of the system.
- `internal/markdown` discovers fenced shell blocks marked with SetupProof
  metadata.
- `internal/config` parses `setupproof.yml`.
- `internal/planning` combines CLI flags, config, target Markdown files, and
  marker metadata into a plan.
- `internal/runner` copies the Git workspace and executes planned blocks with
  the local, action-local, or Docker runner.
- `internal/report` renders terminal output, Markdown reports, and JSON
  reports.
- `internal/adoption` implements `init`, `suggest`, `review`, and `doctor`.
- `internal/project` resolves repository-root-relative files and protects path
  boundaries.
- `internal/diag` emits plan warnings and validation errors.
- `schemas/` defines the public JSON contracts for config, plans, and reports.
- `scripts/` holds repository checks used by CI and dogfooding.

## Invariants

- Unmarked Markdown blocks never execute as SetupProof targets.
- `suggest`, `review`, `doctor`, `--list`, and `--dry-run --json` do not
  execute project commands.
- Execution uses a copied temporary workspace, not the live working directory.
- The default workspace source is tracked files plus tracked modifications.
  Untracked files are copied only when requested.
- Report JSON is machine-facing output. Do not mix human progress text into
  `--json` stdout.
- Plan JSON and report JSON are separate contracts. Planning output must not
  contain execution results.
- Secret environment values pass only when configured and marked values must be
  redacted from supported output sinks.
- Local and action-local are trusted-code runners, not security sandboxes.
- Docker improves isolation but is not a hostile-code security boundary.
- Native Windows and PowerShell execution are outside v0.1 scope.

## Execution Flow

1. `internal/cli` parses the command and builds a `planning.Request`.
2. `internal/planning` resolves targets, loads config, discovers marked blocks,
   validates options, and returns a plan.
3. Non-executing commands render the plan or adoption guidance and stop there.
4. Executing commands ask `internal/runner` to prepare a Git workspace copy.
5. The runner executes marked blocks in file order, preserving shell state
   between shared blocks in the same file.
6. `internal/report` finalizes the result and writes terminal, JSON, or
   Markdown output.

## Change Boundaries

Changes to Markdown discovery should usually include tests in
`internal/markdown` and dry-run coverage in `internal/cli`.

Changes to config, marker options, runner behavior, or JSON output should update
schemas and schema contract tests.

The config and marker parsers are intentionally small. Config files use
repository paths, simple scalars, lists, and two/four/six-space indentation;
tabs, anchors, flow style, document separators, and multi-line scalars are out
of scope. Markdown markers are either one-line HTML comments or info-string
markers immediately after the language token.

Changes to workspace copy, process cleanup, stream capture, redaction, timeout
handling, or Docker behavior should include focused runner tests and a race-test
run.

Changes to public docs should keep release-channel claims tied to channels that
are actually published and tested.
