# JSON Schemas

SetupProof publishes JSON Schemas for the dry-run plan, execution report, and
`setupproof.yml` config contracts. Pin consumers to a versioned schema URL
instead of `main` so validation does not change underneath existing
integrations.

Current schema URLs:

- Plan JSON:
  `https://setupproof.github.io/setupproof/schemas/v1.0.0/setupproof-plan.schema.json`
- Report JSON:
  `https://setupproof.github.io/setupproof/schemas/v1.0.0/setupproof-report.schema.json`
- Config YAML rendered as JSON:
  `https://setupproof.github.io/setupproof/schemas/v1.0.0/setupproof-config.schema.json`

The same files live under `schemas/` for repository-local validation. Release
checks require the root schema files and the published `docs/schemas/v1.0.0/`
copies to match byte-for-byte before publication.

Treat `v1.0.0` as immutable. Additive clarifications can keep using the same
schema contract only when they do not change validation behavior. Tightening,
removing, or renaming fields needs a new versioned schema directory and new
`$id` values.
