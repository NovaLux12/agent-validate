#!/usr/bin/env bash
# scripts/update-schema.sh — refresh the embedded Agent Card v1 JSON Schema
# from the upstream reflectt/agent-identity-kit repository.
#
# Usage: ./scripts/update-schema.sh
#
# What it does:
#   1. Fetches the latest version of schema/agent.schema.json from main.
#   2. Computes a sha256 of the new file.
#   3. Copies it into pkg/agentvalidate/schema/.
#   4. Prints a one-line summary suitable for CHANGELOG.md.
#
# Requires: bash, curl, sha256sum.
#
# This script does NOT modify any source code — after running it, a
# human review pass is still required before committing the new
# schema, because the upstream may have added/removed keywords that
# the validator needs to handle differently.

set -euo pipefail

UPSTREAM_BASE="https://raw.githubusercontent.com/reflectt/agent-identity-kit/main"
SOURCE_URL="$UPSTREAM_BASE/schema/agent.schema.json"
DEST="pkg/agentvalidate/schema/agent.schema.json"

if [[ ! -f "$DEST" ]]; then
    echo "error: $DEST not found — run from the repo root" >&2
    exit 1
fi

if ! command -v curl >/dev/null; then
    echo "error: curl required" >&2
    exit 1
fi

echo "fetching $SOURCE_URL ..."
TMP=$(mktemp)
trap 'rm -f "$TMP"' EXIT
if ! curl -sSf -o "$TMP" "$SOURCE_URL"; then
    echo "error: failed to fetch upstream schema" >&2
    exit 1
fi

# Sanity check: the file must be a non-empty JSON object starting with {
head -c 1 "$TMP" | grep -q '{' || {
    echo "error: fetched file does not look like a JSON schema" >&2
    exit 1
}

HASH=$(sha256sum "$TMP" | awk '{print $1}')
BYTES=$(wc -c < "$TMP")

cp "$TMP" "$DEST"

echo ""
echo "Updated $DEST"
echo "  bytes: $BYTES"
echo "  sha256: $HASH"
echo ""
echo "Suggested CHANGELOG entry:"
echo "  - Agent Card schema sync to upstream (sha256 $HASH, $BYTES bytes)"
