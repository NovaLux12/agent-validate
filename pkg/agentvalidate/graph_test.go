package agentvalidate

import (
	"strings"
	"testing"
)

const healthyDoc = `{
  "version": "1.0",
  "agent": {
    "name": "Nova",
    "handle": "@nova@example.com",
    "description": "test agent"
  },
  "owner": { "name": "Jack" },
  "capabilities": ["code-generation", "web-search"],
  "protocols": { "mcp": true, "a2a": false, "http": true },
  "endpoints": {
    "card": "https://example.com/.well-known/agent.json",
    "inbox": "https://example.com/inbox",
    "status": "https://example.com/status"
  },
  "updated_at": "2026-08-05T00:00:00Z"
}`

const missingDoc = `{
  "version": "1.0",
  "agent": {
    "name": "Bare",
    "handle": "@bare@example.com",
    "description": "missing a lot"
  },
  "owner": { "name": "Jane" },
  "capabilities": ["file-operations"]
}`

func mustGraph(t *testing.T, doc string, results []Result, warnings []Warning) string {
	t.Helper()
	dot, err := Graph([]byte(doc), results, warnings)
	if err != nil {
		t.Fatalf("Graph error: %v", err)
	}
	return dot
}

// agentLine extracts the full DOT node statement for the named node id.
func agentLine(dot, id string) string {
	for _, line := range strings.Split(dot, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, `"`+id+`" [`) {
			return line
		}
	}
	return ""
}

func TestGraphEmitsValidDOTStructure(t *testing.T) {
	dot := mustGraph(t, healthyDoc, nil, nil)
	for _, want := range []string{
		"digraph agent_card {",
		`"agent" [`,
		`"owner" [`,
		`"cap_0" [`,
		`"ep_card" [`,
		"cluster_legend",
		"}",
	} {
		if !strings.Contains(dot, want) {
			t.Errorf("graph missing %q", want)
		}
	}
	if got, want := strings.Count(dot, "{"), strings.Count(dot, "}"); got != want {
		t.Errorf("unbalanced braces: %d open vs %d close", got, want)
	}
}

func TestGraphHealthyColoursGreenRoot(t *testing.T) {
	dot := mustGraph(t, healthyDoc, nil, nil)
	// Every structural node in a healthy card colours green. (The inline
	// legend always carries one red "error" sample, so we scope the check
	// to real nodes rather than a blanket substring.)
	for _, id := range []string{"agent", "owner", "cap_0", "ep_card", "ep_inbox", "ep_status"} {
		if line := agentLine(dot, id); !strings.Contains(line, `color="#2e7d32"`) {
			t.Errorf("node %s should be green in a healthy card, got: %s", id, line)
		}
	}
}

func TestGraphMissingEndpointsAreRed(t *testing.T) {
	dot := mustGraph(t, missingDoc, nil, nil)
	if line := agentLine(dot, "endpoints"); !strings.Contains(line, "endpoints\\n(missing)") || !strings.Contains(line, `color="#c0392b"`) {
		t.Errorf("missing endpoints node should be red, got: %s", line)
	}
}

func TestGraphMissingUpdatedAtIsRed(t *testing.T) {
	dot := mustGraph(t, missingDoc, nil, nil)
	if line := agentLine(dot, "agent"); !strings.Contains(line, "Bare\\n@bare@example.com\\nv1.0") || !strings.Contains(line, `color="#c0392b"`) {
		t.Errorf("root should be red when updated_at is missing, got: %s", line)
	}
}

func TestGraphLintWarningIsAmber(t *testing.T) {
	w := []Warning{{Code: "NO-CARD-URL", Path: "endpoints.card", Message: "missing card"}}
	dot := mustGraph(t, healthyDoc, nil, w)
	if line := agentLine(dot, "ep_card"); !strings.Contains(line, `color="#e6a23c"`) {
		t.Errorf("warning-scoped endpoint should be amber, got: %s", line)
	}
}

func TestGraphSchemaErrorIsRed(t *testing.T) {
	r := []Result{{PropertyPath: "agent.handle", Message: "pattern mismatch"}}
	dot := mustGraph(t, healthyDoc, r, nil)
	if line := agentLine(dot, "agent"); !strings.Contains(line, `color="#c0392b"`) {
		t.Errorf("schema error on agent.handle should make root red, got: %s", line)
	}
}

func TestGraphInvalidJSONErrors(t *testing.T) {
	if _, err := Graph([]byte("{not json"), nil, nil); err == nil {
		t.Fatal("expected an error for non-JSON input")
	}
}

func TestGraphEmptyDocHasRedEndpoints(t *testing.T) {
	dot := mustGraph(t, `{}`, nil, nil)
	if line := agentLine(dot, "endpoints"); !strings.Contains(line, "(missing)") {
		t.Errorf("empty doc should surface missing endpoints red, got: %s", line)
	}
}

func TestGraphProtocolsOnlyWhenTrue(t *testing.T) {
	dot := mustGraph(t, healthyDoc, nil, nil)
	if !strings.Contains(dot, `"proto_mcp" [`) {
		t.Errorf("mcp=true should emit a protocol node")
	}
	if strings.Contains(dot, `"proto_a2a" [`) {
		t.Errorf("a2a=false should NOT emit a protocol node")
	}
}
