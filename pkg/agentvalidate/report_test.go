package agentvalidate

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNewReportPass(t *testing.T) {
	r := NewReport("0.1.0", "agent.json", 260, nil, nil, true)
	if !r.Summary.SchemaPass {
		t.Errorf("expected schema_pass=true")
	}
	if r.Summary.LintWarnings != 0 {
		t.Errorf("expected 0 lint warnings, got %d", r.Summary.LintWarnings)
	}
	if r.Summary.Overall != "pass" {
		t.Errorf("expected overall=pass, got %s", r.Summary.Overall)
	}
	if !r.Schema.Valid {
		t.Errorf("expected schema.valid=true")
	}
	if len(r.Schema.Errors) != 0 {
		t.Errorf("expected 0 schema errors, got %d", len(r.Schema.Errors))
	}
}

func TestNewReportSchemaFail(t *testing.T) {
	errs := []Result{
		{PropertyPath: "agent.handle", Message: "pattern mismatch"},
	}
	r := NewReport("0.1.0", "agent.json", 100, errs, nil, true)
	if r.Summary.SchemaPass {
		t.Errorf("expected schema_pass=false")
	}
	if r.Summary.Overall != "fail" {
		t.Errorf("expected overall=fail, got %s", r.Summary.Overall)
	}
	if r.Schema.Valid {
		t.Errorf("schema.valid should be false when there are errors")
	}
}

func TestNewReportLintOnly(t *testing.T) {
	warnings := []Warning{
		{Code: "NO-ENDPOINTS", Path: "endpoints", Message: "no endpoints"},
	}
	r := NewReport("0.1.0", "agent.json", 260, nil, warnings, false)
	if !r.Summary.SchemaPass {
		t.Errorf("expected schema_pass=true when schema not run")
	}
	if r.Summary.LintWarnings != 1 {
		t.Errorf("expected 1 lint warning, got %d", r.Summary.LintWarnings)
	}
	if r.Summary.Overall != "warn" {
		t.Errorf("expected overall=warn, got %s", r.Summary.Overall)
	}
}

func TestNewReportSchemaPassWithWarnings(t *testing.T) {
	warnings := []Warning{
		{Code: "NO-ENDPOINTS", Path: "endpoints", Message: "no endpoints"},
		{Code: "NO-UPDATED-AT", Path: "updated_at", Message: "missing"},
	}
	r := NewReport("0.1.0", "agent.json", 260, nil, warnings, true)
	if !r.Summary.SchemaPass {
		t.Errorf("expected schema_pass=true")
	}
	if r.Summary.LintWarnings != 2 {
		t.Errorf("expected 2 lint warnings, got %d", r.Summary.LintWarnings)
	}
	if r.Summary.Overall != "warn" {
		t.Errorf("expected overall=warn, got %s", r.Summary.Overall)
	}
}

func TestReportJSON(t *testing.T) {
	r := NewReport("0.1.0", "agent.json", 260, nil, nil, true)
	b, err := r.JSON()
	if err != nil {
		t.Fatalf("JSON() failed: %v", err)
	}
	s := string(b)
	// Check key JSON fields are present.
	for _, want := range []string{`"version"`, `"source"`, `"schema"`, `"lint"`, `"summary"`, `"overall"`} {
		if !strings.Contains(s, want) {
			t.Errorf("JSON output missing %s in: %s", want, s)
		}
	}
}

func TestReportJSONCompact(t *testing.T) {
	r := NewReport("0.1.0", "agent.json", 260, nil, nil, true)
	b, err := r.JSONCompact()
	if err != nil {
		t.Fatalf("JSONCompact() failed: %v", err)
	}
	// Compact JSON should not contain newlines.
	if strings.Contains(string(b), "\n") {
		t.Errorf("JSONCompact() contains newlines: %s", b)
	}
}

func TestReportString(t *testing.T) {
	r := NewReport("0.1.0", "agent.json", 260, nil, nil, true)
	s := r.String()
	if !strings.Contains(s, `"version": "0.1.0"`) {
		t.Errorf("String() missing version: %s", s)
	}
}

func TestReportJSONRoundTrip(t *testing.T) {
	r := NewReport("0.1.0", "agent.json", 260,
		[]Result{{PropertyPath: "agent.handle", Message: "bad"}},
		[]Warning{{Code: "H001", Path: "agent.handle", Message: "warn"}},
		true)
	b, err := r.JSON()
	if err != nil {
		t.Fatalf("JSON() failed: %v", err)
	}
	// Re-parse and check key fields survive the round trip.
	var back map[string]any
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("JSON round-trip unmarshal failed: %v", err)
	}
	if back["version"] != "0.1.0" {
		t.Errorf("version mismatch: %v", back["version"])
	}
	if back["source"] != "agent.json" {
		t.Errorf("source mismatch: %v", back["source"])
	}
}

func TestNewReportSchemaSkippedPasses(t *testing.T) {
	// When ranSchema=false, overall should be "warn" if there are
	// warnings, or "pass" if there aren't — never "fail".
	r := NewReport("0.1.0", "agent.json", 260, nil, nil, false)
	if r.Summary.Overall != "pass" {
		t.Errorf("expected pass when schema skipped and no warnings, got %s", r.Summary.Overall)
	}
}

func TestReportTimestampFormat(t *testing.T) {
	r := NewReport("0.1.0", "agent.json", 260, nil, nil, true)
	// Timestamp should be valid RFC3339.
	if r.Timestamp == "" {
		t.Fatalf("timestamp is empty")
	}
	// RFC3339 requires at least 20 chars (YYYY-MM-DDTHH:MM:SSZ).
	if len(r.Timestamp) < 20 {
		t.Errorf("timestamp too short for RFC3339: %q", r.Timestamp)
	}
	// Should end with Z (UTC).
	if !strings.HasSuffix(r.Timestamp, "Z") {
		t.Errorf("timestamp should end with Z (UTC), got %q", r.Timestamp)
	}
}
