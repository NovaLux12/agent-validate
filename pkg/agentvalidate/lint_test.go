package agentvalidate

import (
	"strings"
	"testing"
)

func TestLintOnMinimal(t *testing.T) {
	got := Lint([]byte(`{
        "version": "1.0",
        "agent": {"name":"X","handle":"@x@example.com","description":"x"},
        "owner": {"name":"X"}
    }`))
	// We expect at least the "no endpoints" warning because the
	// minimal example never declares endpoints.
	found := false
	for _, w := range got {
		if w.Code == "NO-ENDPOINTS" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected NO-ENDPOINTS warning, got: %v", got)
	}
}

func TestLintOnFreeMailHandle(t *testing.T) {
	got := Lint([]byte(`{
        "version": "1.0",
        "agent": {"name":"X","handle":"@x@gmail.com","description":"x"},
        "owner": {"name":"X"},
        "endpoints": {"card": "https://example.com/.well-known/agent.json"}
    }`))
	found := false
	for _, w := range got {
		if w.Code == "H003" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected H003 warning for gmail.com handle, got: %v", got)
	}
}

func TestLintDuplicateCapability(t *testing.T) {
	got := Lint([]byte(`{
        "version": "1.0",
        "agent": {"name":"X","handle":"@x@example.com","description":"x"},
        "owner": {"name":"X"},
        "endpoints": {"card": "https://example.com/.well-known/agent.json"},
        "capabilities": ["foo", "bar", "foo"]
    }`))
	found := false
	for _, w := range got {
		if w.Code == "CAP-DUP" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected CAP-DUP for duplicate capability, got: %v", got)
	}
}

func TestLintUppercaseHandle(t *testing.T) {
	// H002 should fire on a handle that is schema-valid (mixed case
	// is permitted by the schema pattern) but not lowercase-canonical.
	got := Lint([]byte(`{
        "version": "1.0",
        "agent": {"name":"Kai","handle":"@Kai@Reflectt.AI","description":"x"},
        "owner": {"name":"X"},
        "endpoints": {"card": "https://example.com/.well-known/agent.json"}
    }`))
	found := false
	for _, w := range got {
		if w.Code == "H002" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected H002 for uppercase handle, got: %v", got)
	}
}

func TestLintNoH002OnLowercase(t *testing.T) {
	// A canonical lowercase handle must not fire H002.
	got := Lint([]byte(`{
        "version": "1.0",
        "agent": {"name":"Kai","handle":"@kai@reflectt.ai","description":"x"},
        "owner": {"name":"X"},
        "endpoints": {"card": "https://example.com/.well-known/agent.json"}
    }`))
	for _, w := range got {
		if w.Code == "H002" {
			t.Fatalf("unexpected H002 on canonical lowercase handle: %v", w)
		}
	}
}

func TestLintDescriptionCJKRuneCount(t *testing.T) {
	// 100 CJK runes is 300 bytes; the byte-counting version of
	// description length would falsely flag this. With the rune-count
	// fix, this should pass clean.
	desc := strings.Repeat("亜", 100) // 100 CJK runes
	body := `{
        "version": "1.0",
        "agent": {"name":"X","handle":"@x@example.com","description":"` + desc + `"},
        "owner": {"name":"X"},
        "endpoints": {"card": "https://example.com/.well-known/agent.json"}
    }`
	for _, w := range Lint([]byte(body)) {
		if w.Code == "DESC-TOO-LONG" {
			t.Fatalf("DESC-TOO-LONG fired on %d-byte CJK description (should use rune count): %v", len(desc), w)
		}
	}
}

func TestLintOnBadJSON(t *testing.T) {
	got := Lint([]byte(`{not json`))
	found := false
	for _, w := range got {
		if w.Code == "JSON" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected JSON warning on bad input, got: %v", got)
	}
}

func TestLintVerifiedClaim(t *testing.T) {
	got := Lint([]byte(`{
        "version": "1.0",
        "agent": {"name":"X","handle":"@x@example.com","description":"x"},
        "owner": {"name":"X","verified": true},
        "endpoints": {"card": "https://example.com/.well-known/agent.json"}
    }`))
	found := false
	for _, w := range got {
		if w.Code == "VERIFIED-CLAIM" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected VERIFIED-CLAIM warning for owner.verified=true, got: %v", got)
	}
}

func TestLintTrustLevelVerifiedNoVerifiers(t *testing.T) {
	got := Lint([]byte(`{
        "version": "1.0",
        "agent": {"name":"X","handle":"@x@example.com","description":"x"},
        "owner": {"name":"X"},
        "endpoints": {"card": "https://example.com/.well-known/agent.json"},
        "trust": {"level":"verified","verified_by":[]}
    }`))
	found := false
	for _, w := range got {
		if w.Code == "TRUST-UNBACKED" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected TRUST-UNBACKED warning, got: %v", got)
	}
}

func TestLintConfusableAgentName(t *testing.T) {
	// Cyrillic 'А' (U+0410) instead of Latin 'A' — common phishing trick.
	got := Lint([]byte(`{
        "version": "1.0",
        "agent": {"name":"\u0410lice","handle":"@alice@example.com","description":"x"},
        "owner": {"name":"X"},
        "endpoints": {"card": "https://example.com/.well-known/agent.json"}
    }`))
	found := false
	for _, w := range got {
		if w.Code == "CONFUSABLE" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected CONFUSABLE warning for Cyrillic-lookalike name, got: %v", got)
	}
}

func TestResultString(t *testing.T) {
	r := Result{PropertyPath: "agent.handle", Message: "pattern mismatch", Invalid: "@"}
	got := r.String()
	for _, want := range []string{"agent.handle", "pattern mismatch", "@"} {
		if !strings.Contains(got, want) {
			t.Errorf("Result.String() missing %q in %q", want, got)
		}
	}
}

func TestWarningString(t *testing.T) {
	w := Warning{Path: "agent.handle", Code: "H002", Message: "weird handle"}
	got := w.String()
	for _, want := range []string{"H002", "agent.handle", "weird handle"} {
		if !strings.Contains(got, want) {
			t.Errorf("Warning.String() missing %q in %q", want, got)
		}
	}
}
