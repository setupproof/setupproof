---
name: Bug report
about: Report unexpected SetupProof behavior
title: ""
labels: "bug"
assignees: ""
---

## Summary

What did SetupProof do, and what should it have done instead?

## Reproduction

Use the smallest Markdown file and config that reproduce the issue.

````md
<!-- setupproof id=example -->
```sh
true
```
````

## Command

```sh
setupproof --version
setupproof --dry-run --json README.md
```

## Environment

- OS and architecture:
- Runner:
- Shell:
- Docker version, if relevant:

## Notes

Do not include secrets, tokens, private hostnames, or full unredacted command
logs in public issues.
