# agent-validate

Single-binary CLI to validate `agent.json` identity cards against the
[NovaLux12/agent-identity-kit](https://github.com/NovaLux12/agent-identity-kit)
v1.0–v1.3 JSON Schemas. Zero runtime dependencies, optional `--lint` advisory
checks, exit codes ready for CI. Auto-detects card version (`version` / `$schema`)
and validates against the matching bundled schema.

```text
$ agent-validate path/to/agent.json
loaded 260 bytes from path/to/agent.json
✓ schema validation: PASS
! lint: 2 warning(s)
  - NO-ENDPOINTS endpoints: no endpoints block — …
  - NO-UPDATED-AT updated_at: updated_at is missing — …
```

## Why this exists

The upstream `NovaLux12/agent-identity-kit` ships `validate.sh` that wraps
Node's `ajv-cli` or Python's `jsonschema` library. That works, but it
means:

- Installing one runtime just to lint a JSON file.
- Two layers of indirection (Shell → npm or pip → JSON parser) every
  time you want a check.
- No machine-readable output — Ajv's errors are nice for humans but
  hard to grep or post-process in a pipeline.

`agent-validate` is the same job in a single ~7 MB static binary with
no runtime. It also layers on a small set of soft **lint rules** that
flag common mistakes JSON Schema can't express (free-mail handles,
duplicate capability tags, missing `endpoints.card`, etc.).

## Install

**Homebrew** (planned):

```sh
brew install NovaLux12/tap/agent-validate
```

**Direct download** — grab the binary for your OS from
[Releases](https://github.com/NovaLux12/agent-validate/releases):

```sh
curl -L https://github.com/NovaLux12/agent-validate/releases/latest/download/agent-validate_linux_amd64.tar.gz | tar xz
sudo mv agent-validate /usr/local/bin/
```

**`go install`:**

```sh
go install github.com/NovaLux12/agent-validate/cmd/agent-validate@latest
```

**Build from source:**

```sh
git clone https://github.com/NovaLux12/agent-validate.git
cd agent-validate
go build -ldflags="-s -w" -o agent-validate ./cmd/agent-validate
```

## Usage

```text
agent-validate [flags] <file-or-URL|->

Flags:
  -json                   output results as JSON for CI pipelines (implies --quiet)
  -mode string             what to run: validate (schema only), lint, or all (default "all")
  -lint-warnings-fail      exit 2 when lint warnings are present (default: exit 0)
  -no-color                disable ANSI styling in output
  -quiet                   suppress per-file success messages
  -timeout duration        total timeout for URL fetches (default 30s)
  -version                 print version and exit
  -dump-schema string      if set, write the embedded JSON Schema to this file and exit
```

The positional argument can be:

- **a local file path** — e.g. `./my-agent.json`
- **an http(s) URL** — e.g. `https://example.com/.well-known/agent.json`
- **`-`** — read JSON from stdin, useful for shell pipelines

### Examples

```sh
# Local file, schema + lint (default)
agent-validate ./my-agent.json

# Just schema
agent-validate --mode validate ./my-agent.json

# JSON output for CI pipelines
agent-validate --json ./my-agent.json | jq '.summary.overall'

# Lint and treat warnings as failures (good for CI)
agent-validate --lint-warnings-fail ./my-agent.json

# Fetch + validate a published card
agent-validate https://example.com/.well-known/agent.json

# Pipe from another tool
curl -s https://example.com/.well-known/agent.json | agent-validate -

# Dump the embedded schema to disk
agent-validate -dump-schema ./schema.json

# Visualise the card structure as a Graphviz digraph
agent-validate --graph ./my-agent.json | dot -Tsvg > graph.svg
```

## Exit codes

| code | meaning                                                      |
| ---: | :----------------------------------------------------------- |
|    0 | valid (warnings only exit non-zero with `--lint-warnings-fail`) |
|    1 | schema validation failed                                      |
|    2 | lint warnings present (only with `--lint-warnings-fail`)     |
|    3 | fetch / I/O error                                            |
|    4 | argument error                                               |

## Output format

### Text (default)

Schema validation failures render as:

```text
✗ schema validation: FAIL (2 issue(s))
  - agent/handle: regexp pattern ... mismatch on string: ... (got: "@")
  - additional properties are not allowed (got: {"foo":...})
```

Each error is one line, with a stable `path: message` shape so you can
grep, awk, or pipe into other tools.

Lint warnings render as:

```text
! lint: 1 warning(s)
  - H003 agent.handle: handle domain is a personal email provider — …
```

Each warning carries a **stable code** (e.g. `H003`, `CAP-DUP`,
`NO-ENDPOINTS`) suitable for filtering in CI scripts.

### JSON (`--json`)

For CI pipelines and programmatic consumers, pass `--json` to emit a
structured JSON report instead of text:

```json
{
  "version": "0.3.0",
  "source": "agent.json",
  "bytes": 260,
  "timestamp": "2026-07-03T22:25:48Z",
  "schema": {"valid": true, "errors": []},
  "lint": {"warnings": [...]},
  "summary": {"schema_pass": true, "lint_warnings": 2, "overall": "warn"}
}
```

The `summary.overall` field is `"pass"`, `"warn"`, or `"fail"` —
check it with `jq` in CI scripts:

```sh
result=$(agent-validate --json ./agent.json | jq -r '.summary.overall')
[ "$result" = "pass" ] || exit 1
```

`--json` implies `--quiet` (no text-mode status lines). Exit codes
are the same as text mode.

### Graph (`--graph`)

Pass `--graph` to emit a DOT-format digraph of the card's structure,
ready to render with Graphviz:

```sh
agent-validate --graph agent.json | dot -Tsvg > graph.svg
```

The graph shows the root agent identity (name, handle, version), then
the card's actual structure — owner, platform, capabilities, protocols,
endpoints, trust, voice, and links — and colours everything by health:

- **green** — present and consistent
- **amber** — a soft lint warning applies (advisory)
- **red** — a schema validation failure, or a critical field missing
  (e.g. no `endpoints` block, stale `updated_at`)

A small inline legend is included in the rendered output. `--graph`
runs both schema validation and lint so colours are accurate, but does
**not** gate the DOT on pass/fail — a broken card is still worth
visualising (the red edges are the point). It exits 0 on a successful
render. `--graph --no-color` is a no-op, and `--graph --json` yields a
graph (graph wins; the two are mutually exclusive formats).

The graph feature mirrors the v1 schema exactly: capability tags are
flat strings and endpoints are top-level. It deliberately does **not**
invent per-capability→endpoint edges that the schema doesn't express.

## Codes

| code              | meaning                                                                |
| ----------------- | ---------------------------------------------------------------------- |
| `JSON`            | input could not be parsed as JSON                                      |
| `EMPTY`           | document is empty                                                      |
| `H002`            | handle does not look like `@name@domain` — likely a schema failure too |
| `H003`            | handle uses a free-mail domain (gmail.com, etc.)                       |
| `DESC-TOO-LONG`   | description exceeds 280 chars                                           |
| `CONFUSABLE`      | `agent.name` contains characters that look like Latin (Cyrillic/Greek) |
| `VERIFIED-CLAIM`  | `owner.verified=true` — make sure a registry actually vouches           |
| `CAP-DUP`         | same capability tag appears more than once                             |
| `CAP-WHITESPACE`  | capability tag contains whitespace                                     |
| `CAP-EXCESSIVE`   | more than 30 capabilities — list is too diluted to be useful           |
| `NO-ENDPOINTS`    | no `endpoints` block — directories need a canonical URL                |
| `NO-CARD-URL`     | `endpoints.card` missing                                               |
| `CARD-URL-NOT-HTTP` | `endpoints.card` is not an http(s) URL                               |
| `CARD-URL-UNUSUAL`| `endpoints.card` doesn't end with `/.well-known/agent.json`            |
| `TRUST-VERIFIED`  | `trust.level=verified` is the strongest claim — verify before publishing |
| `TRUST-UNBACKED`  | `trust.level=verified` but `verified_by` is empty                      |
| `NO-UPDATED-AT`   | `updated_at` is missing — directories need it to detect staleness      |

## Use as a library

```go
import "github.com/NovaLux12/agent-validate/pkg/agentvalidate"

results, err := agentvalidate.Validate(ctx, data) // schema
warnings := agentvalidate.Lint(data)             // advisory

body, err := agentvalidate.FetchURL(ctx, url, agentvalidate.FetchOptions{})
```

See [pkg/agentvalidate](pkg/agentvalidate) for the public API.

## Use in CI

```yaml
# .github/workflows/agent-validate.yml
name: agent.json validation
on: [push, pull_request]
jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.26'
      - run: go install github.com/NovaLux12/agent-validate/cmd/agent-validate@latest
      - run: agent-validate --lint-warnings-fail ./agent.json
```

Or with the prebuilt binary:

```yaml
- uses: actions/checkout@v4
- run: |
    curl -L https://github.com/NovaLux12/agent-validate/releases/latest/download/agent-validate_linux_amd64.tar.gz \
      | tar xz -C /usr/local/bin agent-validate
- run: agent-validate --lint-warnings-fail ./agent.json
```

## About the spec

`agent-validate` validates against **NovaLux12/agent-identity-kit v1.0–v1.3**.
It bundles all four schemas (`agent.schema.json` for v1.0 and
`agent-card.v1.1.json` / `v1.2.json` / `v1.3.json` for the fork lineage)
and auto-selects the correct one from the card's `version` field (or
`$schema` URL hint, falling back to the latest v1.3 which is
backward-compatible with all prior versions). See `SupportedVersions()`
in the Go API for the bundled list.

v1.1 added `kind`/`scope`/`operator`/`owner:null` (autonomous agents),
v1.2 added web-of-trust `trust.vouched_by[]` and signed-revocation
`trust.revocation_url`, v1.3 added capability-marketplace `offers[]` /
`seeks[]`. Cards using `x_novalux12_*` extensions also validate via
`additionalProperties: true` in v1.1+.

Compare with the v1.0 flat shape this tool targets (embedded in this
binary, see `pkg/agentvalidate/schema/agent.schema.json`):

```json
{
  "version": "1.0",
  "agent": {"name": "...", "handle": "@name@domain", "description": "..."},
  "owner": {"name": "...", "url": "...", "contact": "...", "verified": false},
  "platform": {"runtime": "...", "model": "...", "version": "..."},
  "capabilities": ["...", "..."],
  "protocols": {"mcp": false, "a2a": false, "agent-card": "1.0"},
  "endpoints": {"card": "https://.../", "inbox": "...", "status": "..."},
  "trust": {"level": "new", "verified_by": [], "attestations": []},
  "links": {"website": "...", "repo": "...", "social": []}
}
```

### A note on Google's A2A Agent Card

Google's [A2A protocol](https://github.com/google/A2A) defines a
similar-looking but **different** Agent Card shape (`name`,
`description`, `version`, `supportedInterfaces`, `capabilities`,
`securitySchemes`, `defaultInputModes`, `defaultOutputModes`,
`skills`). Cards written for one will fail validation against the
other. This tool targets the **foragents.dev** v1 shape only.

If you need both, run two validators; or open an issue requesting
A2A schema support here. (TODO: not yet implemented in 0.1.)

## Update the embedded schemas

`pkg/agentvalidate/schema/` bundles verbatim copies of
`agent.schema.json` (v1.0) and `agent-card.v1.1.json` / `v1.2.json` /
`v1.3.json` from `NovaLux12/agent-identity-kit`. To refresh against a
newer upstream:

```sh
./scripts/update-schema.sh
```

See [scripts/update-schema.sh](scripts/update-schema.sh) — the script
fetches the latest from `NovaLux12/agent-identity-kit` and computes a
sha256 fingerprint that goes into `CHANGELOG.md` so audits are easy.

## Why another validator?

Both the upstream `validate.sh` and this binary are intentionally
small. The upstream one is right for first-touching-the-spec use; this
one is right for CI and for callers who want a stable Go library to
embed. The two should converge over time as the spec matures.

## Related

- [NovaLux12/agent-identity-kit](https://github.com/NovaLux12/agent-identity-kit)
  — the spec itself.
- [Google A2A Protocol](https://github.com/google/A2A) — the other
  Agent Card shape you may also need to support.
- [NovaLux12/agent-card](https://github.com/NovaLux12/agent-card) —
  example published agent card.

## License

MIT — see [LICENSE](LICENSE).
