// Package agentvalidate validates agent.json identity cards against the
// reflectt/agent-identity-kit v1 JSON Schema. It exposes both strict
// schema validation and a set of soft lint rules that catch common
// mistakes beyond what JSON Schema can express.
//
// The package has two public entry points:
//
//	Validate(data []byte) ([]Result, error)
//	Lint(data []byte) []Warning
//
// See cmd/agent-validate for the CLI that wires them together.
package agentvalidate

import (
	"bytes"
	_ "embed"
	"fmt"

	"github.com/qri-io/jsonschema"
)

// schemaJSON is the Agent Card v1 JSON Schema, embedded at compile time.
// Source: https://github.com/reflectt/agent-identity-kit/blob/main/schema/agent.schema.json
//
//go:embed schema/agent.schema.json
var schemaJSON []byte

// LoadedSchema returns the parsed JSON Schema as a *jsonschema.Schema.
// The schema is parsed lazily on first call and cached for reuse.
//
// Returned errors indicate a problem with the embedded schema itself
// (a build-time bug, not a user-facing data error). If this function
// ever fails it should fail loud: nothing about the validator is safe
// if its own schema cannot be parsed.
func LoadedSchema() (*jsonschema.Schema, error) {
	cached := new(jsonschema.Schema)
	if err := cached.UnmarshalJSON(schemaJSON); err != nil {
		return nil, fmt.Errorf("agentvalidate: failed to compile embedded schema: %w", err)
	}
	return cached, nil
}

// SchemaBytes exposes the raw JSON Schema bytes. Useful for callers
// that want to serve the schema on an HTTP endpoint, write it to disk,
// or hash it for comparison.
func SchemaBytes() []byte {
	out := make([]byte, len(schemaJSON))
	copy(out, schemaJSON)
	return out
}

// schemaAlias is a tiny compile-time assertion that the embedded schema
// is non-empty. A zero-length embed would silently produce a "valid
// against empty schema" pass on every input, which would be the worst
// possible failure mode for this tool.
var schemaAlias = bytes.Equal([]byte{}, schemaJSON)
