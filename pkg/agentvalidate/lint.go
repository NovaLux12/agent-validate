package agentvalidate

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Warning is an advisory lint finding. Unlike Result (which mirrors
// JSON Schema's hard failures), Warning is a soft suggestion — the
// document can pass schema validation and still trigger lint warnings.
type Warning struct {
	// Path mirrors the JSON pointer convention used by Result, so
	// callers can render warnings uniformly with schema errors.
	Path string
	// Code is a stable identifier (e.g., "H001", "DEPRECATED")
	// suitable for filtering in CI scripts. The code should not
	// change once a rule is published.
	Code string
	// Message is the human-readable explanation of the warning.
	Message string
}

// String renders a Warning as a single short line, with the code prefix
// on the front so users can scan a warning list and quickly orient.
func (w Warning) String() string {
	if w.Path != "" {
		return fmt.Sprintf("%s %s: %s", w.Code, w.Path, w.Message)
	}
	return fmt.Sprintf("%s: %s", w.Code, w.Message)
}

// handleRe matches the Fediverse-style handle format required by the
// upstream schema's pattern keyword. We use it as a precondition: a
// handle that doesn't match this is malformed at the schema level and
// the schema validator will already complain, so we don't bother
// emitting H002 for it (would just duplicate the same complaint).
//
// handleCanonicalRe is a stricter lowercase-only variant used to
// surface the "valid but unconventional" cases the schema accepts:
// uppercase, mixed case, etc. H002 fires only when handleRe matches
// but handleCanonicalRe does not.
var (
	handleRe          = regexp.MustCompile(`^@[a-zA-Z0-9_-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	handleCanonicalRe = regexp.MustCompile(`^@[a-z0-9_-]+@[a-z0-9.-]+\.[a-z]{2,}$`)
	// Bare-reserved domains that are syntactically valid handles but
	// semantically noisy: gmail.com etc. shouldn't appear as the
	// "domain" half of an agent handle.
	handleReservedDomainRe = regexp.MustCompile(`@(gmail|yahoo|hotmail|outlook|icloud|aol|proton|pm)\.(com|net|me)$`)
)

// Lint runs a set of soft advisory checks against an agent.json
// document. It first parses the JSON into a generic map (so it can
// inspect structure independently of schema validation), then walks
// the result, emitting Warnings.
//
// Warnings are intentionally not strict — passing Lint is not required
// for the validator's exit-zero behaviour. The split lets callers
// distinguish "blocked by schema" from "should fix before publishing".
//
// If the input is not valid JSON, Lint returns a single warning with
// Code "JSON" — it does not propagate the parse error, because the
// caller usually wants both schema failures AND lint warnings in a
// single run.
func Lint(data []byte) []Warning {
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return []Warning{{Code: "JSON", Message: fmt.Sprintf("could not parse JSON: %v", err)}}
	}
	if doc == nil {
		return []Warning{{Code: "EMPTY", Message: "document is empty"}}
	}

	out := []Warning{}
	out = append(out, lintAgent(doc)...)
	out = append(out, lintOwner(doc)...)
	out = append(out, lintCapabilities(doc)...)
	out = append(out, lintEndpoints(doc)...)
	out = append(out, lintTrust(doc)...)
	out = append(out, lintTimestamps(doc)...)
	return out
}

// get safely pulls a nested map field, returning nil on type mismatch
// so the lint rules can chain and short-circuit cleanly.
func get(m map[string]any, keys ...string) map[string]any {
	cur := m
	for _, k := range keys {
		v, ok := cur[k].(map[string]any)
		if !ok {
			return nil
		}
		cur = v
	}
	return cur
}

func getStr(m map[string]any, key string) string {
	v, ok := m[key].(string)
	if !ok {
		return ""
	}
	return v
}

func getArr(m map[string]any, key string) []any {
	v, ok := m[key].([]any)
	if !ok {
		return nil
	}
	return v
}

// lintAgent covers agent.name, agent.handle, agent.description.
func lintAgent(doc map[string]any) []Warning {
	a := get(doc, "agent")
	if a == nil {
		return nil
	}
	var out []Warning

	handle := getStr(a, "handle")
	// H002 fires only for handles that pass the schema's pattern but
	// fail the canonical (lowercase) form. Schema-invalid handles get
	// the same complaint from the schema validator; no point in
	// duplicating it here.
	if handle != "" && handleRe.MatchString(handle) && !handleCanonicalRe.MatchString(handle) {
		out = append(out, Warning{
			Path:    "agent.handle",
			Code:    "H002",
			Message: "handle is not lowercase-canonical (e.g., uppercase or mixed case) — verify this is intentional; the conventional form is all-lowercase",
		})
	}
	if handle != "" && handleReservedDomainRe.MatchString(strings.ToLower(handle)) {
		out = append(out, Warning{
			Path:    "agent.handle",
			Code:    "H003",
			Message: "handle domain is a personal email provider — agents are usually published under the owner's domain, not a free-mail account",
		})
	}

	desc := getStr(a, "description")
	// Rune count, not byte count: a 100-character CJK description is
	// 300 bytes but perfectly readable; we want to flag genuinely long
	// descriptions that exceed the social-card soft cap.
	if utf8.RuneCountInString(desc) > 280 {
		out = append(out, Warning{
			Path:    "agent.description",
			Code:    "DESC-TOO-LONG",
			Message: "description is longer than 280 characters; it may truncate in directory listings and directory cards",
		})
	}

	name := getStr(a, "name")
	if name != "" && containsConfusable(name) {
		out = append(out, Warning{
			Path:    "agent.name",
			Code:    "CONFUSABLE",
			Message: "agent.name contains characters that look like Latin letters (Cyrillic/Greek confusables); consider using ASCII for the official display name",
		})
	}
	return out
}

// containsConfusable returns true if a string contains non-ASCII
// characters that often get misused for phishing/impersonation, but
// preserves common whitespace and punctuation.
func containsConfusable(s string) bool {
	for _, r := range s {
		// Allow common punctuation, spaces, ASCII.
		if r < 128 {
			continue
		}
		// Allow CJK, Arabic, Hebrew — these are genuine non-Latin scripts.
		switch {
		case r >= 0x4E00 && r <= 0x9FFF: // CJK Unified
			return false
		case r >= 0x0600 && r <= 0x06FF: // Arabic
			return false
		case r >= 0x0590 && r <= 0x05FF: // Hebrew
			return false
		case r >= 0x3040 && r <= 0x309F: // Hiragana
			return false
		case r >= 0x30A0 && r <= 0x30FF: // Katakana
			return false
		case r >= 0xAC00 && r <= 0xD7AF: // Hangul
			return false
		}
		// Anything else non-ASCII: Cyrillic, Greek, full-width Latin,
		// mathematical variants, etc. Flag it.
		return true
	}
	return false
}

// lintOwner covers owner.name, owner.verified.
func lintOwner(doc map[string]any) []Warning {
	o := get(doc, "owner")
	if o == nil {
		return nil
	}
	var out []Warning
	if getStr(o, "name") == "" {
		// Schema requires, but if schema passed, this is fine.
	}
	if v, ok := o["verified"].(bool); ok && v {
		// verified=true is meaningful only if a registry actually vouches.
		// We can't know that here, but we can remind the operator.
		out = append(out, Warning{
			Path:    "owner.verified",
			Code:    "VERIFIED-CLAIM",
			Message: "owner.verified=true is a strong claim — make sure a registry or attestation path actually vouches for this ownership before publishing",
		})
	}
	return out
}

// lintCapabilities flags duplicates and non-standard character shapes.
func lintCapabilities(doc map[string]any) []Warning {
	caps := getArr(doc, "capabilities")
	if len(caps) == 0 {
		return nil
	}
	seen := make(map[string]int, len(caps))
	var out []Warning
	for i, c := range caps {
		s, ok := c.(string)
		if !ok {
			continue
		}
		seen[s]++
		if seen[s] > 1 && seen[s] == 2 {
			out = append(out, Warning{
				Path:    fmt.Sprintf("capabilities[%d]", i),
				Code:    "CAP-DUP",
				Message: fmt.Sprintf("capability %q appears more than once; capabilities should be unique", s),
			})
		}
		if strings.ContainsAny(s, " \t\n") {
			out = append(out, Warning{
				Path:    fmt.Sprintf("capabilities[%d]", i),
				Code:    "CAP-WHITESPACE",
				Message: fmt.Sprintf("capability %q contains whitespace; tags should be kebab-case single tokens (e.g. \"code-generation\")", s),
			})
		}
	}
	if len(caps) > 30 {
		out = append(out, Warning{
			Path:    "capabilities",
			Code:    "CAP-EXCESSIVE",
			Message: fmt.Sprintf("%d capabilities listed; very long lists dilute the meaningful signal — consider the 5–10 most relevant tags", len(caps)),
		})
	}
	return out
}

// lintEndpoints nudges cards to declare their canonical URL.
func lintEndpoints(doc map[string]any) []Warning {
	e := get(doc, "endpoints")
	if e == nil {
		return []Warning{{
			Path:    "endpoints",
			Code:    "NO-ENDPOINTS",
			Message: "no endpoints block — declare endpoints.card with the canonical URL where this card can be fetched, so directories can verify it",
		}}
	}
	cardURL := getStr(e, "card")
	if cardURL == "" {
		return []Warning{{
			Path:    "endpoints.card",
			Code:    "NO-CARD-URL",
			Message: "endpoints.card is missing — directories use this to verify the card matches a published URL",
		}}
	}
	if !strings.HasPrefix(cardURL, "https://") && !strings.HasPrefix(cardURL, "http://") {
		return []Warning{{
			Path:    "endpoints.card",
			Code:    "CARD-URL-NOT-HTTP",
			Message: "endpoints.card should be an absolute http(s) URL",
		}}
	}
	if !strings.Contains(cardURL, "/.well-known/agent.json") && !strings.Contains(cardURL, "/agent.json") {
		return []Warning{{
			Path:    "endpoints.card",
			Code:    "CARD-URL-UNUSUAL",
			Message: "endpoints.card does not end with /.well-known/agent.json or /agent.json; double-check this is the URL you intend to publish at",
		}}
	}
	return nil
}

// lintTrust surfaces obvious inconsistencies in trust claims.
func lintTrust(doc map[string]any) []Warning {
	t := get(doc, "trust")
	if t == nil {
		return nil
	}
	var out []Warning
	level := getStr(t, "level")
	if level == "verified" {
		out = append(out, Warning{
			Path:    "trust.level",
			Code:    "TRUST-VERIFIED",
			Message: "trust.level=verified is the highest level; verified_by should list at least one registry",
		})
	}
	verifiedBy, _ := t["verified_by"].([]any)
	if level == "verified" && len(verifiedBy) == 0 {
		out = append(out, Warning{
			Path:    "trust.verified_by",
			Code:    "TRUST-UNBACKED",
			Message: "trust.level=verified but verified_by is empty — these should track together",
		})
	}
	return out
}

// lintTimestamps nudges cards to keep created_at / updated_at fresh.
func lintTimestamps(doc map[string]any) []Warning {
	var out []Warning
	if _, ok := doc["updated_at"]; !ok {
		out = append(out, Warning{
			Path:    "updated_at",
			Code:    "NO-UPDATED-AT",
			Message: "updated_at is missing — refresh this when the card changes; directories can use it to detect stale snapshots",
		})
	}
	return out
}
