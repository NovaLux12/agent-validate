package agentvalidate

import (
	"context"
	"fmt"
	"strings"

	"github.com/qri-io/jsonschema"
)

// Result describes a single schema validation failure. It wraps the
// underlying jsonschema.KeyError with a stable, human-readable shape
// suitable for direct CLI output and for callers that want to render
// errors themselves (e.g., a web UI).
type Result struct {
	// PropertyPath is the dotted JSON pointer to the offending property,
	// e.g. "agent.handle" or "capabilities.2". Empty for top-level errors.
	PropertyPath string
	// Message is a short description of why validation failed.
	Message string
	// Invalid is a string rendering of the bad value, when the engine
	// can provide one. May be empty for structural failures.
	Invalid string
}

// IsZero reports whether this Result carries no information — used to
// represent the absence of an error.
func (r Result) IsZero() bool {
	return r.PropertyPath == "" && r.Message == "" && r.Invalid == ""
}

// String renders the Result as a single short line, suitable for a
// machine-readable CI output or a grep target.
//
// Format: <path>: <message> (got: <invalid>)
//
// If PropertyPath is empty it is elided; the colon and parentheses are
// included only when those fields are present.
func (r Result) String() string {
	var b strings.Builder
	if r.PropertyPath != "" {
		b.WriteString(r.PropertyPath)
		b.WriteString(": ")
	}
	b.WriteString(r.Message)
	if r.Invalid != "" {
		b.WriteString(" (got: ")
		b.WriteString(r.Invalid)
		b.WriteString(")")
	}
	return b.String()
}

// Validate checks a JSON-encoded agent.json against the embedded
// Agent Card v1 schema. It returns a list of Result; an empty slice
// means the document is structurally valid.
//
// Validate does NOT run the soft lint rules — those live in Lint().
// The split is deliberate: schema validation is mandatory (exit-code
// matter), lint is advisory (warning matter).
//
// Validate returns an error only for problems with the input itself
// (not valid JSON, schema failed to compile, etc.). All schema-driven
// failures are returned as Results.
func Validate(ctx context.Context, data []byte) ([]Result, error) {
	schema, err := LoadedSchema()
	if err != nil {
		return nil, fmt.Errorf("agentvalidate: %w", err)
	}

	keyErrs, err := schema.ValidateBytes(ctx, data)
	if err != nil {
		return nil, fmt.Errorf("agentvalidate: schema validation failed: %w", err)
	}

	if len(keyErrs) == 0 {
		return nil, nil
	}

	out := make([]Result, 0, len(keyErrs))
	for _, ke := range keyErrs {
		path := strings.TrimPrefix(ke.PropertyPath, "/")
		out = append(out, Result{
			PropertyPath: path,
			Message:      ke.Message,
			Invalid:      invalidValueOrEmpty(ke.InvalidValue),
		})
	}
	return out, nil
}

// invalidValueOrEmpty renders a validator-supplied bad value for our
// CLI output. Nil invalid values are rendered as empty so the output
// stays clean.
func invalidValueOrEmpty(v any) string {
	if v == nil {
		return ""
	}
	return jsonschema.InvalidValueString(v)
}
