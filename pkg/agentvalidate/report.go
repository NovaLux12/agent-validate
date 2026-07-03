package agentvalidate

import (
	"encoding/json"
	"fmt"
	"time"
)

// Report is the structured output of a full validation run (schema +
// lint). It is designed to be marshalled to JSON for CI pipelines and
// programmatic consumers; the human-readable text output lives in the
// CLI layer.
//
// All fields are populated regardless of pass/fail so consumers can
// always rely on the shape being present.
type Report struct {
	// Version is the agent-validate release that produced this report.
	Version string `json:"version"`
	// Source is the display label for where the data came from (file
	// path, URL, or "stdin").
	Source string `json:"source"`
	// Bytes is the size of the validated document in bytes.
	Bytes int `json:"bytes"`
	// Timestamp is when the validation ran, in RFC3339.
	Timestamp string `json:"timestamp"`
	// Schema contains schema validation results.
	Schema SchemaReport `json:"schema"`
	// Lint contains lint results.
	Lint LintReport `json:"lint"`
	// Summary is the roll-up for quick CI decisions.
	Summary SummaryReport `json:"summary"`
}

// SchemaReport holds the schema validation portion of a Report.
type SchemaReport struct {
	// Valid is true if the document passed JSON Schema validation.
	Valid bool `json:"valid"`
	// Errors lists individual schema failures (empty when Valid).
	Errors []Result `json:"errors"`
}

// MarshalJSON ensures Errors is emitted as [] rather than null when
// there are no errors — cleaner for JSON consumers.
func (s SchemaReport) MarshalJSON() ([]byte, error) {
	type alias SchemaReport
	if s.Errors == nil {
		s.Errors = []Result{}
	}
	return json.Marshal(alias(s))
}

// LintReport holds the lint portion of a Report.
type LintReport struct {
	// Warnings lists individual lint findings (may be non-empty even
	// when schema validation passed).
	Warnings []Warning `json:"warnings"`
}

// MarshalJSON ensures Warnings is emitted as [] rather than null when
// there are no warnings — cleaner for JSON consumers.
func (l LintReport) MarshalJSON() ([]byte, error) {
	type alias LintReport
	if l.Warnings == nil {
		l.Warnings = []Warning{}
	}
	return json.Marshal(alias(l))
}

// SummaryReport is the top-level pass/fail roll-up. CI scripts can
// check .summary.overall == "pass" without inspecting individual
// sections.
type SummaryReport struct {
	// SchemaPass mirrors SchemaReport.Valid.
	SchemaPass bool `json:"schema_pass"`
	// LintWarnings is the count of lint warnings.
	LintWarnings int `json:"lint_warnings"`
	// Overall is "pass", "fail", or "warn":
	//   pass — schema valid, no lint warnings (or lint not run)
	//   fail — schema validation failed
	//   warn — schema valid but lint warnings present
	Overall string `json:"overall"`
}

// NewReport constructs a Report from the components a CLI run already
// has. It computes the Summary automatically. If schemaErrors is nil
// the schema is considered valid (callers that skip schema validation
// should pass nil and set ranSchema=false).
func NewReport(version, source string, dataBytes int, schemaErrors []Result, warnings []Warning, ranSchema bool) Report {
	r := Report{
		Version:   version,
		Source:    source,
		Bytes:     dataBytes,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Schema: SchemaReport{
			Valid:  len(schemaErrors) == 0,
			Errors: schemaErrors,
		},
		Lint: LintReport{
			Warnings: warnings,
		},
	}

	if !ranSchema {
		// Schema check skipped — don't claim pass or fail.
		r.Summary.SchemaPass = true
	} else {
		r.Summary.SchemaPass = len(schemaErrors) == 0
	}
	r.Summary.LintWarnings = len(warnings)

	switch {
	case ranSchema && !r.Summary.SchemaPass:
		r.Summary.Overall = "fail"
	case len(warnings) > 0:
		r.Summary.Overall = "warn"
	default:
		r.Summary.Overall = "pass"
	}

	return r
}

// JSON renders the Report to indented JSON bytes.
func (r Report) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// JSONCompact renders the Report to compact JSON (no whitespace) for
// piping into jq or other line-oriented tools.
func (r Report) JSONCompact() ([]byte, error) {
	return json.Marshal(r)
}

// String implements fmt.Stringer, rendering the Report as indented JSON.
// If marshalling fails (which should be impossible for this struct)
// it returns a fallback string with the error.
func (r Report) String() string {
	b, err := r.JSON()
	if err != nil {
		return fmt.Sprintf(`{"error": "could not marshal report: %s"}`, err)
	}
	return string(b)
}
