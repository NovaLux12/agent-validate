package agentvalidate

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResolveWellKnownURL(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"https://example.com", "https://example.com/.well-known/agent.json", false},
		{"https://example.com/", "https://example.com/.well-known/agent.json", false},
		{"https://example.com/path", "https://example.com/.well-known/agent.json", false},
		{"http://localhost:8080", "http://localhost:8080/.well-known/agent.json", false},
		{"ftp://nope.example.com", "", true},
		{"file:///etc/passwd", "", true},
		{"://malformed", "", true},
		// Userinfo (username:password) must be stripped so the result
		// is safe to log or surface in error messages.
		{"https://user:secret@example.com", "https://example.com/.well-known/agent.json", false},
	}
	for _, tc := range cases {
		got, err := ResolveWellKnownURL(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("%q: expected error, got nil", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: unexpected error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%q: got %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFetchURLLocalhost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"hello":"world","agent":{"name":"x"}}`))
	}))
	defer server.Close()

	got, err := FetchURL(context.Background(), server.URL, FetchOptions{Timeout: 1_000_000_000}) // 1s
	if err != nil {
		t.Fatalf("FetchURL failed: %v", err)
	}
	if !strings.Contains(string(got), `"agent"`) {
		t.Errorf("expected response body with agent, got %q", got)
	}
}

func TestFetchURL404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	_, err := FetchURL(context.Background(), server.URL, FetchOptions{Timeout: 1_000_000_000})
	if err == nil {
		t.Fatalf("expected error on 404, got nil")
	}
	if !errors.Is(err, ErrFetch) {
		t.Errorf("expected ErrFetch, got %v", err)
	}
}

func TestFetchURLRejectsUnsupportedScheme(t *testing.T) {
	cases := []string{
		"file:///etc/passwd",
		"ftp://example.com/agent.json",
		"gopher://example.com",
		"javascript:alert(1)",
	}
	for _, raw := range cases {
		_, err := FetchURL(context.Background(), raw, FetchOptions{Timeout: 1_000_000_000})
		if err == nil {
			t.Errorf("%s: expected error, got nil", raw)
		}
		if !errors.Is(err, ErrFetch) {
			t.Errorf("%s: expected ErrFetch, got %v", raw, err)
		}
	}
}

func TestFetchURLBodyTooLarge(t *testing.T) {
	big := strings.Repeat("a", 1024*1024+1) // 1 MiB + 1
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(big))
	}))
	defer server.Close()

	_, err := FetchURL(context.Background(), server.URL, FetchOptions{
		Timeout:      1_000_000_000,
		MaxBodyBytes: 1024, // tiny cap so we don't wait for the full upload
	})
	if err == nil {
		t.Fatalf("expected oversize-body error, got nil")
	}
}

func TestFetchURLRedirectLimit(t *testing.T) {
	// Server issues 5 redirects in a chain; with MaxRedirects=5 the
	// client should follow all 5 and reach the final handler.
	// Pre-fix, MaxRedirects=5 actually allowed only 4 (off-by-one).
	redirects := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if redirects < 5 {
			redirects++
			http.Redirect(w, r, "/next", http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	body, err := FetchURL(context.Background(), server.URL+"/start", FetchOptions{
		Timeout:      2_000_000_000, // 2s
		MaxRedirects: 5,
	})
	if err != nil {
		t.Fatalf("expected to follow all 5 redirects, got error: %v", err)
	}
	if !strings.Contains(string(body), `"ok"`) {
		t.Errorf("expected final body, got %q", body)
	}
	if redirects != 5 {
		t.Errorf("expected 5 redirects issued, server saw %d", redirects)
	}
}

func TestFetchURLUserAgentIncludesVersion(t *testing.T) {
	var seenUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	if _, err := FetchURL(context.Background(), server.URL, FetchOptions{Timeout: 1_000_000_000}); err != nil {
		t.Fatalf("FetchURL: %v", err)
	}
	if !strings.Contains(seenUA, "agent-validate/") {
		t.Errorf("expected User-Agent to identify agent-validate, got %q", seenUA)
	}
	if !strings.Contains(seenUA, Version) {
		t.Errorf("expected User-Agent to include version %q, got %q", Version, seenUA)
	}
}
