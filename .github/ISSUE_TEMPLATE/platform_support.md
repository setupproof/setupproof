---
name: Platform support
about: Discuss OS, shell, or CI platform support
title: ""
labels: "platform"
assignees: ""
---

## Platform

- Linux
- macOS
- Windows through WSL2
- Native Windows
- Docker
- Other:

## Shell

- sh
- bash
- shell
- PowerShell
- Other:

## Goal

What setup-doc workflow should work on this platform?

## Notes

Native Windows and PowerShell execution are outside v0.1 scope. Proposals for
new platform behavior should start from ADR 0010 and ADR 0011, then explain
test coverage, shell semantics, path behavior, environment behavior, and report
compatibility.
