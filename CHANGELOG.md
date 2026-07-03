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
