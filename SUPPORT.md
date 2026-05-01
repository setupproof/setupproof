# Support

SetupProof v0.1.0 can run from the Go module install path, release archives, a
source checkout, or the pinned GitHub Action.

For help with a SetupProof run, include:

- the command you ran;
- `setupproof --version`;
- operating system and architecture;
- runner (`local`, `action-local`, or `docker`);
- the relevant marked block ID and file path;
- `setupproof review README.md` output for the block;
- the Markdown or JSON report when it is safe to share.

Do not include secrets, private hostnames, or unredacted command logs in public
support requests. SetupProof redacts configured secrets, but transformed or
partial secrets may still appear in project command output.

For untrusted pull requests, run `setupproof review README.md` before executing
marked commands. Default CI examples should pass no repository secrets, prefer
hosted ephemeral runners, and use the standard `pull_request` event rather
than privileged target-branch pull request workflows.

Native Windows execution and PowerShell fenced blocks are unsupported in v0.1.
Windows users should run SetupProof through WSL2.
