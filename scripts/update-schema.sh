#!/usr/bin/env bash
# scripts/update-schema.sh — refresh the embedded Agent Card JSON Schemas
# from NovaLux12/agent-identity-kit.
#
# Usage: ./scripts/update-schema.sh
#
# Fetches all versioned schemas (v1.0 + v1.1–v1.3) from main and copies
# them into pkg/agentvalidate/schema/. Prints sha256 + byte counts.
#
# Requires: bash, curl, sha256sum.

set -euo pipefail

UPSTREAM_BASE="https://raw.githubusercontent.com/NovaLux12/agent-identity-kit/main"
DEST_DIR="pkg/agentvalidate/schema"

if [[ ! -d "$DEST_DIR" ]]; then
    echo "error: $DEST_DIR not found — run from the repo root" >&2
    exit 1
fi

if ! command -v curl >/dev/null; then
    echo "error: curl required" >&2
    exit 1
fi

declare -A SOURCES=(
    ["agent.schema.json"]="$UPSTREAM_BASE/schema/agent.schema.json"
    ["agent-card.v1.1.json"]="$UPSTREAM_BASE/schema/agent-card.v1.1.json"
    ["agent-card.v1.2.json"]="$UPSTREAM_BASE/schema/agent-card.v1.2.json"
    ["agent-card.v1.3.json"]="$UPSTREAM_BASE/schema/agent-card.v1.3.json"
)

for fname in "${!SOURCES[@]}"; do
    url="${SOURCES[$fname]}"
    dest="$DEST_DIR/$fname"
    echo "fetching $url -> $dest ..."
    tmp=$(mktemp)
    trap 'rm -f "$tmp"' EXIT
    if ! curl -sSf -o "$tmp" "$url"; then
        echo "error: failed to fetch $url" >&2
        exit 1
    fi
    head -c 1 "$tmp" | grep -q '{' || {
        echo "error: fetched $fname does not look like JSON" >&2
        exit 1
    }
    hash=$(sha256sum "$tmp" | awk '{print $1}')
    bytes=$(wc -c < "$tmp")
    cp "$tmp" "$dest"
    echo "  $fname: $bytes bytes, sha256 $hash"
    rm -f "$tmp"
    trap - EXIT
done

echo ""
echo "All schemas updated in $DEST_DIR"
echo "Suggested CHANGELOG entry:"
echo "  - Agent Card schemas sync to upstream (NovaLux12/agent-identity-kit main)"
