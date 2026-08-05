package agentvalidate

// Graph renders the structure of an agent card as a DOT-format digraph,
// suitable for piping into Graphviz (`dot -Tsvg`). It has zero external
// dependencies beyond the standard library — all DOT is emitted via
// fmt.Fprintf on a strings.Builder.
//
// The graph is faithful to the v1 agent-card schema: capability tags are
// flat strings, endpoints live at the top level, and protocols/owner/
// trust/links are distinct subtrees off the root agent node. Nodes and
// edges are coloured by health:
//
//	green  — present and consistent
//	amber  — a soft lint warning applies (advisory)
//	red    — a schema validation failure, or a critical field missing
//
// Colour is a property of the node; the edge from the root borrows the
// child's colour so a single glance shows which subtrees are healthy.

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type health int

const (
	healthy health = iota // green
	warned                // amber
	broken                // red
)

// Colour values are explicit hex so nodes and the legend agree
// regardless of whether Graphviz knows the X11 name (it does not know
// "amber").
const (
	colorGreen = "#2e7d32"
	colorAmber = "#e6a23c"
	colorRed   = "#c0392b"
)

func (h health) color() string {
	switch h {
	case warned:
		return colorAmber
	case broken:
		return colorRed
	default:
		return colorGreen
	}
}

// dotGraph accumulates nodes, edges, and clusters into a DOT document.
type dotGraph struct {
	b strings.Builder
}

func (g *dotGraph) header() {
	g.b.WriteString("digraph agent_card {\n")
	g.b.WriteString("  rankdir=LR;\n")
	g.b.WriteString("  node [fontname=\"Helvetica\"];\n")
	g.b.WriteString("  edge [fontname=\"Helvetica\", fontsize=10];\n")
	g.b.WriteString("  graph [fontname=\"Helvetica\"];\n")
}

func (g *dotGraph) node(id, label string, h health, shape string) {
	fmt.Fprintf(&g.b, "  %q [label=\"%s\", shape=%s, color=\"%s\"];\n",
		id, label, shape, h.color())
}

func (g *dotGraph) edge(from, to string, h health) {
	fmt.Fprintf(&g.b, "  %q -> %q [color=\"%s\"];\n", from, to, h.color())
}

func (g *dotGraph) clusterBegin(name, label string) {
	fmt.Fprintf(&g.b, "  subgraph \"cluster_%s\" {\n", name)
	fmt.Fprintf(&g.b, "    label=\"%s\";\n", label)
	fmt.Fprintf(&g.b, "    style=\"rounded,filled\";\n    fillcolor=\"#fafafa\";\n    color=\"#cccccc\";\n")
}

func (g *dotGraph) clusterEnd() {
	g.b.WriteString("  }\n")
}

// legend emits a small inline legend so the rendered SVG is
// self-describing.
func (g *dotGraph) legend() {
	g.b.WriteString("  subgraph cluster_legend {\n")
	g.b.WriteString("    label=\"Legend\";\n    style=\"rounded\";\n    color=\"#cccccc\";\n")
	g.b.WriteString("    rank=same;\n")
	g.b.WriteString("    \"legend_green\" [label=\"ok\", shape=box, style=filled, fillcolor=\"#2e7d32\"];\n")
	g.b.WriteString("    \"legend_amber\" [label=\"lint\", shape=box, style=filled, fillcolor=\"#e6a23c\"];\n")
	g.b.WriteString("    \"legend_red\" [label=\"error\", shape=box, style=filled, fillcolor=\"#c0392b\"];\n")
	g.b.WriteString("    \"legend_green\" -> \"legend_amber\" [style=invis];\n")
	g.b.WriteString("    \"legend_amber\" -> \"legend_red\" [style=invis];\n")
	g.b.WriteString("  }\n")
}

// bleed returns the colour to use for a node whose baseline health is h.
// A schema validation result landing in the node's scope forces broken;
// a lint warning (when the node isn't already broken) forces warned.
func bleed(h health, warnings []Warning, results []Result, scopes ...string) health {
	if h == broken {
		return broken
	}
	for _, r := range results {
		for _, s := range scopes {
			if r.PropertyPath == s || strings.HasPrefix(r.PropertyPath, s+".") || strings.HasPrefix(r.PropertyPath, s+"[") {
				return broken
			}
		}
	}
	if h == warned {
		return warned
	}
	for _, s := range scopes {
		for _, w := range warnings {
			if w.Path == s || strings.HasPrefix(w.Path, s+".") || strings.HasPrefix(w.Path, s+"[") {
				return warned
			}
		}
	}
	return h
}

func firstStr(v any) string {
	s, _ := v.(string)
	return s
}

// Graph parses a JSON agent card and emits a DOT digraph documenting its
// structure, coloured by schema/lint health. schemaResults and warnings
// are the outputs of Validate/Lint; either may be nil. If the document
// is not valid JSON it returns an error.
func Graph(data []byte, schemaResults []Result, warnings []Warning) (string, error) {
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return "", fmt.Errorf("graph: could not parse JSON: %w", err)
	}

	g := &dotGraph{}
	g.header()

	// Root: agent identity.
	agentName := "agent"
	agentHandle := ""
	if a := get(doc, "agent"); a != nil {
		if n := getStr(a, "name"); n != "" {
			agentName = n
		}
		agentHandle = getStr(a, "handle")
	}
	agentVersion := "v" + firstStr(doc["version"])
	rootH := healthy
	if _, ok := doc["updated_at"]; !ok {
		rootH = broken
	}
	rootH = bleed(rootH, warnings, schemaResults, "agent", "updated_at")

	rootLabel := agentName
	if agentHandle != "" {
		rootLabel += fmt.Sprintf("\\n%s", agentHandle)
	}
	rootLabel += fmt.Sprintf("\\n%s", agentVersion)
	g.node("agent", rootLabel, rootH, "doubleoctagon")
	rootID := "agent"

	// Owner.
	ownerName := "owner"
	if o := get(doc, "owner"); o != nil {
		if n := getStr(o, "name"); n != "" {
			ownerName = n
		}
	}
	ownerH := bleed(healthy, warnings, schemaResults, "owner")
	g.node("owner", "owner\\n"+ownerName, ownerH, "box")
	g.edge(rootID, "owner", ownerH)

	// Platform.
	if p := get(doc, "platform"); p != nil {
		label := "platform"
		if run, model := getStr(p, "runtime"), getStr(p, "model"); run != "" || model != "" {
			label += "\\n" + strings.TrimSpace(run+" "+model)
		}
		ph := bleed(healthy, warnings, schemaResults, "platform")
		g.node("platform", label, ph, "box")
		g.edge(rootID, "platform", ph)
	}

	// Capabilities (flat tag list).
	if caps := getArr(doc, "capabilities"); len(caps) > 0 {
		g.clusterBegin("capabilities", "capabilities")
		for i, c := range caps {
			tag, _ := c.(string)
			if tag == "" {
				tag = fmt.Sprintf("cap[%d]", i)
			}
			ch := bleed(healthy, warnings, schemaResults, fmt.Sprintf("capabilities.%d", i))
			// Lint warnings index capabilities as capabilities[i]; bleed
			// covers the dotted form, so catch the bracket form too.
			if ch == healthy {
				for _, w := range warnings {
					if w.Path == fmt.Sprintf("capabilities[%d]", i) {
						ch = warned
					}
				}
			}
			g.node(fmt.Sprintf("cap_%d", i), tag, ch, "ellipse")
			g.edge(rootID, fmt.Sprintf("cap_%d", i), ch)
		}
		g.clusterEnd()
	}

	// Protocols (only emitted when the boolean flag is true).
	if pr := get(doc, "protocols"); pr != nil {
		wrote := false
		for _, key := range []string{"mcp", "a2a", "http"} {
			if b, ok := pr[key].(bool); ok && b {
				if !wrote {
					g.clusterBegin("protocols", "protocols")
					wrote = true
				}
				g.node("proto_"+key, strings.ToUpper(key), healthy, "ellipse")
				g.edge(rootID, "proto_"+key, healthy)
			}
		}
		if wrote {
			g.clusterEnd()
		}
	}

	// Endpoints.
	if ep := get(doc, "endpoints"); ep != nil {
		g.clusterBegin("endpoints", "endpoints")
		known := []string{"card", "inbox", "status"}
		keys := known
		var extra []string
		for k := range ep {
			if !contains(known, k) {
				extra = append(extra, k)
			}
		}
		sort.Strings(extra)
		keys = append(keys, extra...)
		for _, key := range keys {
			eh := healthy
			if getStr(ep, key) == "" {
				eh = broken
			}
			eh = bleed(eh, warnings, schemaResults, "endpoints."+key)
			id := "ep_" + key
			g.node(id, "ep."+key, eh, "box")
			g.edge(rootID, id, eh)
		}
		g.clusterEnd()
	} else {
		g.node("endpoints", "endpoints\\n(missing)", broken, "box")
		g.edge(rootID, "endpoints", broken)
	}

	// Trust.
	if t := get(doc, "trust"); t != nil {
		level := getStr(t, "level")
		if level == "" {
			level = "(unset)"
		}
		th := bleed(healthy, warnings, schemaResults, "trust")
		g.node("trust", "trust\\n"+level, th, "box")
		g.edge(rootID, "trust", th)
	}

	// Voice.
	if get(doc, "voice") != nil {
		vh := bleed(healthy, warnings, schemaResults, "voice")
		g.node("voice", "voice", vh, "box")
		g.edge(rootID, "voice", vh)
	}

	// Links.
	if l := get(doc, "links"); l != nil {
		g.clusterBegin("links", "links")
		for _, key := range []string{"website", "repo", "documentation"} {
			if getStr(l, key) != "" {
				g.node("link_"+key, "link."+key, healthy, "note")
				g.edge(rootID, "link_"+key, healthy)
			}
		}
		g.clusterEnd()
	}

	g.legend()
	g.b.WriteString("}\n")
	return g.b.String(), nil
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
