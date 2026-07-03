package agentvalidate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestValidateAgainstExamples verifies the three upstream examples
// pass schema validation. A regression here means the embedded schema
// or the validator's interpretation has drifted from the upstream
// repo — exactly the kind of silent break this test exists to catch.
func TestValidateAgainstExamples(t *testing.T) {
	cases := []struct {
		name      string
		file      string
		wantValid bool
		wantErrs  []string // substrings expected in any error
	}{
		{
			name:      "minimal",
			file:      "../../examples/minimal.agent.json",
			wantValid: true,
		},
		{
			name:      "kai-full",
			file:      "../../examples/kai.agent.json",
			wantValid: true,
		},
		{
			// team.agents.json is a roster, not a single Agent Card,
			// so it is intentionally NOT a single-card document.
			// We expect schema to reject it.
			name:      "team-roster-rejected",
			file:      "../../examples/team.agents.json",
			wantValid: false,
			wantErrs:  []string{"required", "version", "agent", "owner"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(".", tc.file))
			if err != nil {
				t.Fatalf("read %s: %v", tc.file, err)
			}
			results, err := Validate(context.Background(), data)
			if err != nil {
				t.Fatalf("Validate() error: %v", err)
			}
			if tc.wantValid {
				if len(results) != 0 {
					t.Errorf("expected valid, got %d errors: %v", len(results), results)
				}
				return
			}
			if len(results) == 0 {
				t.Fatalf("expected invalid, got no errors")
			}
			// Verify the right things failed: at least one of our
			// wanted substrings must appear in some error message.
			combined := ""
			for _, r := range results {
				combined += r.String() + "\n"
			}
			matched := false
			for _, want := range tc.wantErrs {
				if strings.Contains(combined, want) {
					matched = true
					break
				}
			}
			if !matched {
				t.Errorf("expected at least one of %v in errors, got:\n%s", tc.wantErrs, combined)
			}
		})
	}
}

func TestValidateBadHandle(t *testing.T) {
	// Take kai's full example and break the handle so we know our
	// only change is the one we're testing.
	src, err := os.ReadFile(filepath.Join(".", "../../examples/kai.agent.json"))
	if err != nil {
		t.Fatalf("read kai: %v", err)
	}
	// Minimal in-place JSON mutation rather than depending on a
	// fixture file; keeps the test self-contained.
	bad := strings.Replace(string(src), `"@kai@reflectt.ai"`, `"this-is-not-a-handle"`, 1)
	if bad == string(src) {
		t.Fatalf("setup failed: handle substitution did not change input")
	}

	results, err := Validate(context.Background(), []byte(bad))
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("expected validation failure on malformed handle, got pass")
	}
	found := false
	for _, r := range results {
		if r.PropertyPath == "agent/handle" && strings.Contains(r.Message, "pattern") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected an agent/handle pattern error, got: %v", results)
	}
}

func TestValidateRejectsInvalidJSON(t *testing.T) {
	// The schema library raises an error on unparseable JSON rather
	// than producing zero-result "passes" — that's the right
	// behaviour for a validator (we DO want to surface this clearly
	// rather than treat it as a silent pass). This test pins the
	// contract: caller gets an error.
	_, err := Validate(context.Background(), []byte(`{ this isn't json `))
	if err == nil {
		t.Fatalf("expected error on non-JSON input, got nil")
	}
	if !strings.Contains(err.Error(), "JSON") {
		t.Errorf("expected JSON-related error message, got: %v", err)
	}
}
