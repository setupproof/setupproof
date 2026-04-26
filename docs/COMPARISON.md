# Comparison

SetupProof is for maintainers who want README setup commands to fail in CI
before new contributors copy stale instructions.

## SetupProof

- Executes only shell blocks explicitly marked by the maintainer.
- Runs marked blocks from a copied temporary workspace by default.
- Reports file, line, block ID, runner, timeout, result, and redacted output.
- Keeps non-executing inspection commands available for review and CI adoption.
- Sends no telemetry or hidden update checks.

## Notebook-Style Markdown Runners

Notebook-style tools are useful when a Markdown file is the command interface
itself. SetupProof is narrower: unmarked prose examples stay inert, and marked
quickstarts are treated as verification targets rather than a general notebook.

## Custom CI Scripts

Project-specific scripts are flexible and usually the right choice for complex
test orchestration. SetupProof focuses on the boundary between documentation and
CI: it finds the documented quickstart block, runs that exact marked source, and
records a report tied back to the Markdown location.

## When Not To Use SetupProof

- The commands are not safe to run in CI.
- The setup path requires interactive prompts.
- The repository needs a full end-to-end environment test instead of README
  command verification.
- The project cannot tolerate running contributor-controlled shell commands.
