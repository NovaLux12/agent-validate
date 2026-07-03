# Changelog

All notable changes to `agent-validate` are documented here. The project
does **not** follow strict SemVer yet (we're pre-0.2); the format is
loose-keep-a-tidy-trail.

## 0.1.0 — 2026-07-03

First public release. Schema validation against the embedded
`reflectt/agent-identity-kit` v1 JSON Schema, soft lint rules for
common mistakes beyond what JSON Schema expresses, fetch-with-cap
remote URL support, single static binary build. Includes:

- `pkg/agentvalidate` library: `Validate`, `Lint`, `FetchURL`,
  `ResolveWellKnownURL`, `LoadedSchema`, `SchemaBytes`.
- `cmd/agent-validate` CLI with `--mode {validate,lint,all}`,
  `--lint-warnings-fail`, `--quiet`, `--no-color`, `--timeout`,
  `--version`, `--dump-schema`.
- Tests against the three upstream examples plus synthetic broken
  fixtures and an httptest-driven fetch test.
- `scripts/update-schema.sh` to refresh the embedded schema from
  upstream.

Verified against upstream examples (commit
`reflectt/agent-identity-kit` main as of 2026-07-03):

- `examples/minimal.agent.json` — schema PASS, 2 lint warnings
- `examples/kai.agent.json` — schema PASS, 1 lint warning
  (verified-claim)
- `examples/team.agents.json` — schema FAIL (correctly: this is a
  roster, not a single card)
- `examples/nova-lux.agent.json` — schema FAIL (correctly: this card
  is in Google's A2A shape, not the foragents.dev v1 shape; documenting
  the distinction in README)

## Schema fingerprint (0.1.0)

- `pkg/agentvalidate/schema/agent.schema.json` —
  sha256 `7e9c9ab7ce4e0c45b1b3baeef397198edba44249dc78e64fdc994523292312ad`
