package agentvalidate

import "testing"

// TestLoadedSchemaCompiles verifies the embedded schema parses cleanly.
// A failure here is a build-time bug — never a user-facing data issue.
func TestLoadedSchemaCompiles(t *testing.T) {
	s, err := LoadedSchema()
	if err != nil {
		t.Fatalf("LoadedSchema() failed: %v", err)
	}
	if s == nil {
		t.Fatalf("LoadedSchema() returned nil schema without error")
	}
}

func TestSchemaBytes(t *testing.T) {
	got := SchemaBytes()
	if len(got) == 0 {
		t.Fatalf("SchemaBytes() returned empty data — embed failed")
	}
	// The agent-identity-kit schema starts with a { and contains "Agent Card" in $title.
	if got[0] != '{' {
		t.Fatalf("SchemaBytes() does not start with '{'; got first byte %q", got[0])
	}
}
