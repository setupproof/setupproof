# Security

SetupProof v0.1 runs from this source tree. Packaged installs and external
Action tag examples are not published yet.

## Reporting Security Issues

Do not open a public issue for a vulnerability, secret leak, or private
repository exposure. Use a private maintainer channel until a dedicated security
advisory workflow exists.

Include:

- the SetupProof version or source commit;
- operating system and runner (`local`, `action-local`, or `docker`);
- the command you ran;
- whether `--include-untracked`, `--keep-workspace`, or Docker was used;
- a minimal marked block that reproduces the issue, with secrets removed.

## Execution Model

SetupProof runs only explicitly marked shell blocks. The default execution mode
copies tracked working-tree files into a temporary workspace and runs commands
there. That temporary workspace is not the maintainer's live working directory.

The local and action-local runners are trusted-code runners. They are not a
security sandbox for hostile commands. The Docker runner improves isolation, but
it is still intended for project setup commands you choose to run.

The Docker runner keeps its control sidecar inside the container workspace
under `.setupproof/`. Marked blocks should not delete that directory.

## Privacy

SetupProof sends no telemetry and performs no hidden update checks.

No user environment variables pass by default beyond the sanitized baseline.
Configured secret environment values are redacted before supported output
sinks. Transformed, encoded, partial, or newly generated secrets may still
appear in project command output.

## Pull Requests

For untrusted pull requests, run `setupproof review README.md` before executing
marked commands. Default CI examples should pass no repository secrets, use the
standard `pull_request` event, and prefer hosted ephemeral runners.

Native Windows execution and PowerShell fenced blocks are unsupported in v0.1.
Windows users should run SetupProof through WSL2.
