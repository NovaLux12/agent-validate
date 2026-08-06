## Unreleased

### Docs

- **README: explicitly document the v1.0-only scope.** The validator was
  silently pinned to the v1.0 flat Agent Card schema while the
  `agent-identity-kit` fork diverged to v1.1/v1.2/v1.2.1/v1.3, so a card
  written against the newer lineage failed confusingly (`version: "1.3"`
  not in enum `["1.0"]`) with no explanation. "About the spec" now states
  the v1.0-only boundary up front, points v1.1+ card authors at the
  fork's `validate.sh`, and links the multi-schema request in issue #4.

## 0.3.0 — 2026-08-05

Add `--graph` output: a DOT digraph of an agent card's structure for
visualisation with Graphviz.

**Added:**

- **`--graph` flag.** Emits a DOT-format digraph on stdout describing the
  card's structure — root agent identity, owner, platform, capabilities,
  protocols, endpoints, trust, voice, and links — coloured by health:
  green (present/consistent), amber (lint warning applies), red (schema
  failure or critical field missing). Pipes straight into `dot`:
  ```sh
  agent-validate --graph agent.json | dot -Tsvg > graph.svg
  ```
  Zero new dependencies (DOT emitted via `fmt.Fprintf`).
- **`agentvalidate.Graph`** function in the public package API. Takes the
  raw document plus schema results and lint warnings, returns DOT text.
- **`--graph` semantics:** runs both schema validation and lint so edges
  are coloured by status, but does *not* gate the DOT output on pass/fail
  (a broken card is still worth visualising). Exit code is 0 for a
  successful render. `--graph --no-color` is a no-op (graph colour is
  not terminal output). `--graph` and `--json` are mutually exclusive
  output formats; graph wins if both are set.

**Tests:** +9 graph test functions covering DOT structure and brace
balance, green/amber/red colouring, missing endpoints, missing
updated_at, schema-error bleed, invalid-JSON handling, and
protocol emission (9 new; total 52).

# Changelog

All notable changes to `agent-validate` are documented here. The project
does **not** follow strict SemVer yet (we're pre-0.2); the format is
loose-keep-a-tidy-trail.

## 0.2.0 — 2026-07-03

Add JSON output mode for CI pipelines and programmatic consumers.

**Added:**

- **`--json` flag.** When set, the CLI emits a structured JSON report
  instead of text output. The report includes schema validation
  results, lint warnings, and a summary with an `overall` field
  (`"pass"`, `"warn"`, `"fail"`) for quick CI decisions. `--json`
  implies `--quiet` (no text-mode status lines). Exit codes are
  unchanged. Example:
  ```json
  {
    "summary": {"schema_pass": true, "lint_warnings": 2, "overall": "warn"}
  }
  ```
- **`agentvalidate.Report`** type in the public package API. Consumers
  using agent-validate as a Go library can now construct structured
  reports programmatically via `NewReport(...)` and marshal with
  `Report.JSON()` or `Report.JSONCompact()`.

**Tests:** 19 → 31 (added 8 report tests + 3 CLI integration tests for
`--json` mode: pass case, schema-fail case, text-suppression case).

## 0.1.1 — 2026-07-03

Quick post-launch fixes from the M3 verifier pass.

**Fixed:**

- **fetch.go redirect off-by-one.** `CheckRedirect` was rejecting the
  Nth redirect when it should have rejected the (N+1)th. With
  `MaxRedirects: 5` the validator actually allowed only 4 redirects.
  Changed `len(via) >= N` to `len(via) > N` and updated the inline
  comment so the next reader doesn't repeat the same mistake.
- **fetch.go `User-Agent` now includes the real version** (was a
  hardcoded `/dev` placeholder). Added `Version` constant in the
  package and updated the default UA. Verified via
  `TestFetchURLUserAgentIncludesVersion`.
- **fetch.go `ResolveWellKnownURL` strips URL userinfo.** A
  `https://user:pass@host` input no longer leaks credentials into the
  output path. Verified via a new case in `TestResolveWellKnownURL`.
- **lint.go H002 now actually catches uppercase handles.** The
  H002 regex was a verbatim copy of the schema pattern, so it never
  fired on a schema-valid uppercase handle. Added
  `handleCanonicalRe` (lowercase-only) and rewired H002 to fire only
  when the handle passes the schema pattern but fails the canonical
  form. The schema validator catches the truly malformed; H002 now
  catches the "valid but unconventional" cases the schema misses.
- **lint.go description length now uses rune count, not byte count.**
  A 100-rune CJK description (300 bytes) no longer falsely triggers
  `DESC-TOO-LONG`. Verified via `TestLintDescriptionCJKRuneCount`.

**Removed:**

- `lintVoice` and `lintProtocols` were no-op stubs. The functions
  computed values and discarded them with `_ = …`, making the file
  read like an unimplemented policy rather than a deliberate "no
  checks" decision. Removed both functions and their call sites.

**Tests:** 13 → 19 (added `TestFetchURLRedirectLimit`,
`TestFetchURLUserAgentIncludesVersion`,
`TestLintUppercaseHandle`, `TestLintNoH002OnLowercase`,
`TestLintDescriptionCJKRuneCount`, plus a userinfo case in
`TestResolveWellKnownURL`).

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
