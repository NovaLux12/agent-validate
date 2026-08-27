package agentvalidate

import "testing"

// TestLoadedSchemaCompiles verifies the embedded schemas parse cleanly.
func TestLoadedSchemaCompiles(t *testing.T) {
	for _, ver := range SupportedVersions() {
		t.Run(ver, func(t *testing.T) {
			s, err := LoadedSchemaForVersion(ver)
			if err != nil {
				t.Fatalf("LoadedSchemaForVersion(%q) failed: %v", ver, err)
			}
			if s == nil {
				t.Fatalf("LoadedSchemaForVersion(%q) returned nil", ver)
			}
		})
	}
	// Also test the default LoadedSchema (latest).
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
	if got[0] != '{' {
		t.Fatalf("SchemaBytes() does not start with '{'; got first byte %q", got[0])
	}
}

func TestSchemaBytesForVersion(t *testing.T) {
	for _, ver := range SupportedVersions() {
		t.Run(ver, func(t *testing.T) {
			got := SchemaBytesForVersion(ver)
			if len(got) == 0 {
				t.Fatalf("SchemaBytesForVersion(%q) empty", ver)
			}
			if got[0] != '{' {
				t.Fatalf("SchemaBytesForVersion(%q) bad prefix", ver)
			}
		})
	}
}

func TestDetectVersion(t *testing.T) {
	cases := []struct {
		json string
		want string
	}{
		{`{"version":"1.0","agent":{"name":"x"}}`, "1.0"},
		{`{"version":"1.1","agent":{"name":"x"}}`, "1.1"},
		{`{"version":"1.3","agent":{"name":"x"}}`, "1.3"},
		{`{"$schema":"https://github.com/NovaLux12/agent-identity-kit/blob/main/schema/agent-card.v1.2.json","agent":{"name":"x"}}`, "1.2"},
		{`{"agent":{"name":"x"}}`, ""},
	}
	for _, tc := range cases {
		got := detectVersion([]byte(tc.json))
		if got != tc.want {
			t.Errorf("detectVersion(%s) = %q, want %q", tc.json, got, tc.want)
		}
	}
}
