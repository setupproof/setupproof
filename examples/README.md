# SetupProof Examples

These are small example projects with marked quickstart blocks. They are meant
to show realistic SetupProof inputs, not distribution or release instructions
for SetupProof itself.

Examples included:

| Example | Purpose | `check-examples.sh` coverage |
| --- | --- | --- |
| `node-npm` | npm script test with no package install step. | list, review, plan |
| `python-pip` | pip requirements check plus Python unit tests, with package index access disabled. | list, review, plan, execution when `python3 -m venv` works |
| `docker-compose` | Docker Compose smoke run with an explicit digest-pinned image pull. | list, review, plan |
| `monorepo` | Configured multi-file example using `setupproof.yml`. | list, review, plan, execution when `npm` is available |
| `go` | No-config path that uses root `README.md`. | list, review, plan, execution |
| `rust` | Cargo test example with no third-party crates. | list, review, plan |

Manual runs need the matching project toolchain. The `python-pip` example needs
`python3`, and the `docker-compose` example needs Docker Compose.

Smoke coverage lives in `scripts/check-examples.sh`. It validates all example
Markdown with `--list`, `review`, and `--dry-run --json`, then executes the Go
no-config example and the configured monorepo example from temporary Git
repositories.
