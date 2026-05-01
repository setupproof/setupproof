## Summary

What changed, and why?

## Scope

- Docs
- Examples
- Tests
- Parser
- Planning
- Runner
- Reports
- Action

## Checks

Run the smallest relevant set, then expand for release-facing changes.

```sh
make fmt-check
make test
make vet
make dogfood
```

For release-facing, runner, report, schema, Action, or public docs changes:

```sh
make fmt-check
make check
make staticcheck
make vuln
make actionlint
make release-check
```

## Release Notes

- No package-manager install claims unless that package exists.
- No external Action tag examples unless immutable release tags exist.
- Native Windows and PowerShell support stay out of v0.1 unless the ADRs change.
- Plan/report JSON changes include schema and contract-test updates.
- Public examples do not include secrets, private hostnames, or private paths.
