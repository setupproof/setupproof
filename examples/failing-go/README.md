# Failing Go Example

This fixture shows what SetupProof reports when a marked README block has
drifted from the project layout. It is intentionally checked in as a
failing example so maintainers can verify SetupProof's failure output
end-to-end, and so the docs have a concrete reference for "what does a
real failure look like".

The marked block targets `./internal/greeter`, but the package actually
lives under `./pkg/greeter` (a common drift pattern: a contributor moves
a package and forgets to update the README). Running the block from a
clean checkout fails with an exit-1 from `go test`.

<!-- setupproof id=failing-go-test -->
```sh
go test ./internal/greeter
```

## Expected output

When SetupProof runs this README from a clean checkout, the marked block
fails because the imported path does not exist. The exact line offset
varies as this README evolves, but the result line is stable:

```text
[failed] README.md#failing-go-test file=README.md:<line> runner=local timeout=120s result=failed exit=1
```

The corresponding `go test` failure that drives `exit=1` is the standard
"no Go files" / "package … is not in std" error.

## When a maintainer would use this

- Verifying SetupProof's failure path after a release-candidate change
  to the runner or reporter.
- Demonstrating to new contributors what a drifted README looks like
  through the SetupProof lens, without having to break a real example.
- Smoke coverage in `scripts/check-examples.sh` (`failing-go-test`
  appears in `--list` / `review` / `--dry-run --json` output, and the
  example's actual execution is asserted to produce `result=failed`).

`./pkg/greeter` is the real package and has a passing unit test. The
drift is purely in the README path.
