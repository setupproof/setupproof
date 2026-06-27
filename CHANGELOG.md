# Changelog

## 0.1.2 - 2026-06-27

- Changed `setupproof init --workflow` to generate the released Action
  workflow for normal repositories.
- Pinned generated workflows to the current Action tag and matching
  `cli-version`.
- Updated onboarding docs and tests for external repository setup.

## 0.1.1 - 2026-06-26

- Added a failing Go fixture that demonstrates README drift detection.
- Improved GitHub Action step summaries with failure location, next command,
  source, and output-tail context.
- Added npm packed-tarball release tooling and smoke checks.
- Documented Docker runner tradeoffs and trust boundaries.
- Updated release checks for the current Action dependency set.

## 0.1.0 - 2026-05-01

- Initial public release.
- Marked Markdown block discovery with HTML-comment and info-string marker
  forms.
- Config-driven dry-run plan JSON.
- Local, action-local, and Docker execution runners.
- Terminal, Markdown, and JSON execution reports.
- GitHub Action integration with versioned CLI archive download.
- Go module install path and release archives for Linux and macOS.
- Example fixtures, CI/platform guidance, and reproducible terminal demo.
