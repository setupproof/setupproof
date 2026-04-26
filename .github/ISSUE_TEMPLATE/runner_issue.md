---
name: Runner issue
about: Report or propose execution, timeout, workspace, or Docker behavior
title: ""
labels: "runner"
assignees: ""
---

## Runner

- local
- action-local
- docker

## Behavior

What happened, and what should have happened?

## Minimal Markdown

````md
<!-- setupproof id=example -->
```sh
true
```
````

## Environment

- OS and architecture:
- Shell:
- Docker version, if relevant:
- `setupproof --version`:

## Checks

Runner changes should usually include:

```sh
make fmt-check
make test
make race
make dogfood
```
