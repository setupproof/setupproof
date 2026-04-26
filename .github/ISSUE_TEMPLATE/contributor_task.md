---
name: Contributor task
about: Propose or claim a small focused contribution
title: ""
labels: "good first issue"
assignees: ""
---

## Task

What should change?

## Area

- Docs
- Examples
- Tests
- Parser
- Planning
- Runner
- Reports
- Action

## Expected Checks

```sh
make fmt-check
make test
make vet
make dogfood
```

## Notes

Keep the change focused. Avoid behavior changes unless the expected behavior is
already documented or covered by an accepted ADR.
