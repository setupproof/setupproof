---
name: Setup doc verification issue
about: Report a marked quickstart that fails or reports confusing output
title: ""
labels: "support"
assignees: ""
---

## What failed?

Describe the marked block and the expected setup behavior.

## Command

```sh
setupproof --version
setupproof review README.md
setupproof --report-json setupproof-report.json --require-blocks --no-color --no-glyphs README.md
```

## Environment

- OS and architecture:
- Runner: local, action-local, or docker
- Docker image, if used:
- Config file, if used:

## Report

Attach the Markdown or JSON report only after removing secrets, private
hostnames, and full unredacted command logs.
