// Package agentvalidate validates agent.json identity cards against the
// NovaLux12/agent-identity-kit JSON Schemas (v1.0–v1.3).
package agentvalidate

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/qri-io/jsonschema"
)

// Embedded schemas — one per spec version. v1.0 is the original
// foragents.dev flat schema; v1.1+ are from NovaLux12/agent-identity-kit
// and are backward-compatible (every v1.0 card validates against v1.3).

//go:embed schema/agent.schema.json
var schemaV10 []byte

//go:embed schema/agent-card.v1.1.json
var schemaV11 []byte

//go:embed schema/agent-card.v1.2.json
var schemaV12 []byte

//go:embed schema/agent-card.v1.3.json
var schemaV13 []byte

// LatestVersion is the newest bundled schema version.
const LatestVersion = "1.3"

// schemaByVersion maps version strings to raw schema bytes.
var schemaByVersion = map[string][]byte{
	"1.0": schemaV10,
	"1.1": schemaV11,
	"1.2": schemaV12,
	"1.3": schemaV13,
}

// orderedVersions lists versions from newest to oldest for fallback.
var orderedVersions = []string{"1.3", "1.2", "1.1", "1.0"}

// SchemaBytesForVersion returns the raw JSON Schema bytes for the given
// version. If version is unknown or empty, the latest schema is returned.
func SchemaBytesForVersion(version string) []byte {
	if b, ok := schemaByVersion[version]; ok {
		out := make([]byte, len(b))
		copy(out, b)
		return out
	}
	out := make([]byte, len(schemaV13))
	copy(out, schemaV13)
	return out
}

// SupportedVersions returns the list of bundled schema versions.
func SupportedVersions() []string {
	out := make([]string, len(orderedVersions))
	copy(out, orderedVersions)
	// Return ascending for display.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// detectVersion inspects the raw JSON to find the card's declared
// version. It checks `version` and `$schema` fields. Returns empty
// string if neither is present or parse fails.
func detectVersion(data []byte) string {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		return ""
	}
	// Prefer explicit `version` field.
	if raw, ok := doc["version"]; ok {
		var v string
		if err := json.Unmarshal(raw, &v); err == nil {
			v = strings.TrimSpace(v)
			if _, ok := schemaByVersion[v]; ok {
				return v
			}
			// Unknown version string — let caller fallback to latest.
			return v
		}
	}
	// Try $schema URL hint: look for v1.1, v1.2, v1.3 substrings.
	if raw, ok := doc["$schema"]; ok {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			for _, ver := range orderedVersions {
				if strings.Contains(s, "v"+ver) || strings.Contains(s, "/v"+ver+".") {
					return ver
				}
			}
			// Generic v1 URL without minor — treat as v1.0.
			if strings.Contains(s, "foragents.dev") {
				return "1.0"
			}
		}
	}
	return ""
}

// LoadedSchema returns the parsed JSON Schema for the latest version
// (currently v1.3). This is the default schema for callers that do
// not need version-specific selection.
//
// For version-aware validation use LoadedSchemaForVersion.
func LoadedSchema() (*jsonschema.Schema, error) {
	return LoadedSchemaForVersion(LatestVersion)
}

// LoadedSchemaForVersion returns the parsed JSON Schema for the given
// version string. Unknown versions fall back to the latest schema.
func LoadedSchemaForVersion(version string) (*jsonschema.Schema, error) {
	raw, ok := schemaByVersion[version]
	if !ok {
		raw = schemaV13
	}
	s := new(jsonschema.Schema)
	if err := s.UnmarshalJSON(raw); err != nil {
		return nil, fmt.Errorf("agentvalidate: failed to compile embedded schema %q: %w", version, err)
	}
	return s, nil
}

// SchemaBytes exposes the raw JSON Schema bytes for the latest version.
// Useful for callers that want to serve the schema on an HTTP endpoint,
// write it to disk, or hash it for comparison.
//
// For version-specific bytes use SchemaBytesForVersion.
func SchemaBytes() []byte {
	out := make([]byte, len(schemaV13))
	copy(out, schemaV13)
	return out
}

// SchemaBytesV10 exposes the raw v1.0 schema bytes (legacy accessor).
func SchemaBytesV10() []byte {
	out := make([]byte, len(schemaV10))
	copy(out, schemaV10)
	return out
}

// schemaAlias is a tiny compile-time assertion that the embedded schemas
// are non-empty. A zero-length embed would silently produce a "valid
// against empty schema" pass on every input.
var schemaAlias = bytes.Equal([]byte{}, schemaV13) && bytes.Equal([]byte{}, schemaV10)
