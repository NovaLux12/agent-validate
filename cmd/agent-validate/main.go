// Command agent-validate validates agent.json identity cards against
// the reflectt/agent-identity-kit v1 JSON Schema. It is a single
// static binary with no runtime dependencies (other than the JSON
// file you point at, optionally retrieved via http[s]).
//
// Usage:
//
//	agent-validate [flags] <file-or-url>
//
// If the argument is a URL it is fetched (capped at 1 MiB by default)
// and the response body is treated as the agent.json payload.
// Otherwise the argument is treated as a local path. "-" reads from
// stdin, which is what you want for piping from another tool.
//
// Exit codes:
//
//	0   the document is valid (and, when --lint-warnings is pass,
//	    no warnings were emitted)
//	1   the document failed schema validation
//	2   the document failed lint-only checks (schema passed but
//	    soft rules warn)
//	3   a fetch or I/O error prevented validation
//	4   a tool/argument error
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/NovaLux12/agent-validate/pkg/agentvalidate"
)

const toolVersion = "0.3.0"

func main() {
	// Sub-command style: pick validate, lint, or all. We default to "all".
	var (
		showVersion = flag.Bool("version", false, "print version and exit")
		mode        = flag.String("mode", "all", "what to run: validate (schema only), lint (soft checks), or all (default)")
		noColor     = flag.Bool("no-color", false, "disable ANSI styling in output")
		quiet       = flag.Bool("quiet", false, "suppress per-file success messages; just exit")
		warnExit    = flag.Bool("lint-warnings-fail", false, "exit 2 when lint warnings are present (default: exit 0)")
		timeout     = flag.Duration("timeout", 30*time.Second, "total timeout for URL fetches")
		schemaOut   = flag.String("dump-schema", "", "if set, write the embedded JSON Schema to this file and exit")
		jsonOut     = flag.Bool("json", false, "output results as JSON for CI pipelines (implies --quiet)")
		graphOut    = flag.Bool("graph", false, "emit a DOT digraph of the agent card structure (coloured by validation status)")
	)
	flag.Usage = usage
	flag.Parse()

	if *showVersion {
		fmt.Fprintf(os.Stdout, "agent-validate %s\n", toolVersion)
		return
	}

	if *schemaOut != "" {
		if err := os.WriteFile(*schemaOut, agentvalidate.SchemaBytes(), 0o644); err != nil {
			fail(4, "could not write schema to %s: %v", *schemaOut, err)
		}
		if !*quiet {
			fmt.Fprintf(os.Stdout, "wrote %d bytes of embedded schema to %s\n", len(agentvalidate.SchemaBytes()), *schemaOut)
		}
		return
	}

	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "error: expected exactly one positional argument (file path, URL, or -)")
		fmt.Fprintln(os.Stderr, "")
		usage()
		os.Exit(4)
	}
	target := flag.Arg(0)

	switch *mode {
	case "validate", "lint", "all":
	default:
		fmt.Fprintf(os.Stderr, "error: --mode must be one of validate, lint, all (got %q)\n", *mode)
		os.Exit(4)
	}

	doValidate := *mode == "validate" || *mode == "all"
	doLint := *mode == "lint" || *mode == "all"

	// Set up context with signal handling so users can Ctrl-C a slow
	// URL fetch without leaking goroutines.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	data, source, err := loadSource(ctx, target, *timeout)
	if err != nil {
		fail(3, "could not load %s: %v", target, err)
	}
	// In graph mode stdout must carry nothing but DOT — a stray banner
	// would make the pipe into `dot` fail.
	if !*quiet && !*jsonOut && !*graphOut {
		fmt.Fprintf(os.Stdout, "loaded %d bytes from %s\n", len(data), source)
	}

	// ---- graph mode ----
	// --graph emits a DOT digraph of the card's structure. It runs both
	// schema validation and lint so edges can be coloured by health, but
	// does NOT gate the DOT output on pass/fail (a broken card is still
	// worth visualising — its red edges are the point). Exit code is 0
	// for a successful rendering regardless of validation status, so the
	// output is safe to pipe straight into `dot`. --graph and --json are
	// mutually exclusive output formats; graph wins if both are set.
	if *graphOut {
		graphResults, _ := agentvalidate.Validate(ctx, data)
		graphWarnings := agentvalidate.Lint(data)
		dot, err := agentvalidate.Graph(data, graphResults, graphWarnings)
		if err != nil {
			fail(4, "could not render graph: %v", err)
		}
		fmt.Fprintln(os.Stdout, dot)
		return
	}

	// ---- schema validation ----
	var schemaResults []agentvalidate.Result
	if doValidate {
		var err error
		schemaResults, err = agentvalidate.Validate(ctx, data)
		if err != nil {
			if *jsonOut {
				printJSONError(toolVersion, source, err)
			}
			fail(3, "schema validation could not run: %v", err)
		}
		if len(schemaResults) == 0 {
			if !*quiet && !*jsonOut {
				printSuccess(*noColor, "schema validation: PASS")
			}
		} else {
			if !*jsonOut {
				printFail(*noColor, "schema validation: FAIL (%d issue(s))", len(schemaResults))
				for _, r := range schemaResults {
					fmt.Fprintf(os.Stdout, "  - %s\n", r.String())
				}
			}
			// In JSON mode we collect and continue to lint,
			// so the report can show both schema errors AND
			// lint warnings in one document.
			if !*jsonOut {
				os.Exit(1)
			}
		}
	}

	// ---- soft lint ----
	var warnings []agentvalidate.Warning
	if doLint {
		warnings = agentvalidate.Lint(data)
	}

	// ---- JSON output ----
	if *jsonOut {
		report := agentvalidate.NewReport(toolVersion, source, len(data), schemaResults, warnings, doValidate)
		b, err := report.JSON()
		if err != nil {
			fail(3, "could not marshal JSON report: %v", err)
		}
		fmt.Fprintln(os.Stdout, string(b))
		// Exit codes follow the same rules as text mode.
		if doValidate && len(schemaResults) > 0 {
			os.Exit(1)
		}
		if len(warnings) > 0 && *warnExit {
			os.Exit(2)
		}
		return
	}

	if !doLint {
		return
	}

	if len(warnings) == 0 {
		if !*quiet {
			printSuccess(*noColor, "lint: clean")
		}
		return
	}

	printWarn(*noColor, "lint: %d warning(s)", len(warnings))
	for _, w := range warnings {
		fmt.Fprintf(os.Stdout, "  - %s\n", w.String())
	}
	if *warnExit {
		os.Exit(2)
	}
}

// loadSource unifies the three input shapes (file, URL, stdin).
//
// For URLs it defers to agentvalidate.FetchURL with the user's timeout.
// For "-" it reads stdin into memory (the agent.json validation model
// requires the whole document in memory anyway).
//
// Returns (data, displaySource, error). displaySource is used in the
// "loaded N bytes from …" line and is intended to be a stable label,
// not a secret or a query string.
func loadSource(ctx context.Context, target string, timeout time.Duration) ([]byte, string, error) {
	if target == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, "", fmt.Errorf("read stdin: %w", err)
		}
		return data, "stdin", nil
	}
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		data, err := agentvalidate.FetchURL(ctx, target, agentvalidate.FetchOptions{Timeout: timeout})
		if err != nil {
			return nil, "", err
		}
		return data, target, nil
	}
	// Path case. Reject path traversal characters up front so we don't
	// silently load something outside the caller's expectation.
	if strings.ContainsAny(target, "\x00") {
		return nil, "", fmt.Errorf("invalid path %q", target)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return nil, "", err
	}
	return data, target, nil
}

func usage() {
	fmt.Fprintf(os.Stderr, `agent-validate %s — validate agent.json identity cards

Usage:
  agent-validate [flags] <file|URL|->

Flags:
`, toolVersion)
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, `
Examples:
  agent-validate path/to/agent.json
  agent-validate --mode lint path/to/agent.json
  agent-validate --json path/to/agent.json | jq .summary.overall
  agent-validate --graph path/to/agent.json | dot -Tsvg > graph.svg
  agent-validate https://example.com/.well-known/agent.json
  cat agent.json | agent-validate -
  agent-validate --dump-schema schema.json

Exit codes:
  0  valid (warnings only when --lint-warnings-fail is set)
  1  schema validation failed
  2  lint warnings (only with --lint-warnings-fail)
	3  fetch / I/O error
  4  argument error
`)
}

// printJSONError outputs a JSON error report when the validator itself
// fails (e.g., schema could not compile, fetch error). This keeps the
// --json contract: always emit valid JSON, even on error.
func printJSONError(version, source string, err error) {
	report := agentvalidate.Report{
		Version:   version,
		Source:    source,
		Bytes:     0,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Schema:    agentvalidate.SchemaReport{Valid: false, Errors: []agentvalidate.Result{{Message: err.Error()}}},
		Lint:      agentvalidate.LintReport{},
		Summary:   agentvalidate.SummaryReport{SchemaPass: false, Overall: "fail"},
	}
	b, _ := report.JSON()
	fmt.Fprintln(os.Stderr, string(b))
}

func fail(code int, format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(code)
}

func printSuccess(noColor bool, format string, args ...any) {
	if noColor {
		fmt.Fprintf(os.Stdout, "✓ "+format+"\n", args...)
		return
	}
	fmt.Fprintf(os.Stdout, "\033[32m✓\033[0m "+format+"\n", args...)
}

func printWarn(noColor bool, format string, args ...any) {
	if noColor {
		fmt.Fprintf(os.Stdout, "! "+format+"\n", args...)
		return
	}
	fmt.Fprintf(os.Stdout, "\033[33m!\033[0m "+format+"\n", args...)
}

func printFail(noColor bool, format string, args ...any) {
	if noColor {
		fmt.Fprintf(os.Stdout, "✗ "+format+"\n", args...)
		return
	}
	fmt.Fprintf(os.Stdout, "\033[31m✗\033[0m "+format+"\n", args...)
}
