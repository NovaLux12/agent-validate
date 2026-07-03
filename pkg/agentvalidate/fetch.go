package agentvalidate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// FetchOptions tunes how FetchURL behaves. All fields are optional; the
// zero value gives a sensible default: 30s timeout, follow up to 5
// redirects, fetch up to 1 MiB.
type FetchOptions struct {
	// Timeout for the entire request, including redirects.
	Timeout time.Duration
	// MaxRedirects caps how many Location: hops we follow.
	MaxRedirects int
	// MaxBodyBytes caps the response body we read; oversized bodies
	// are truncated and treated as a fetch error.
	MaxBodyBytes int64
	// UserAgent to send with the request. Defaults to
	// "agent-validate/<version>" if empty.
	UserAgent string
}

func (o FetchOptions) withDefaults() FetchOptions {
	if o.Timeout == 0 {
		o.Timeout = 30 * time.Second
	}
	if o.MaxRedirects == 0 {
		o.MaxRedirects = 5
	}
	if o.MaxBodyBytes == 0 {
		o.MaxBodyBytes = 1 << 20 // 1 MiB
	}
	if o.UserAgent == "" {
		o.UserAgent = "agent-validate/dev (https://github.com/NovaLux12/agent-validate)"
	}
	return o
}

// FetchURL retrieves an agent.json from a remote URL. It returns the
// raw bytes so callers can pass them straight to Validate and Lint.
//
// Security choices:
//   - Only http:// and https:// schemes are accepted. file://, ftp://,
//     and anything else returns an error rather than a silent fallback.
//     This blocks accidental SSRF via copy-paste of file:// URLs.
//   - Redirects are NOT followed automatically unless MaxRedirects > 0.
//     (Default is 5, matching stdlib behaviour.) When we do follow, we
//     re-validate the scheme on each hop.
//   - The body is capped to MaxBodyBytes; truncating giant responses
//     silently would produce confusing validation failures downstream.
//
// FetchURL is best-effort and DNS/network errors are surfaced as
// ErrFetch to the caller — there are no automatic retries.
func FetchURL(ctx context.Context, rawURL string, opts FetchOptions) ([]byte, error) {
	opts = opts.withDefaults()

	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFetch, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("%w: only http and https are supported (got %q)", ErrFetch, u.Scheme)
	}

	client := &http.Client{
		Timeout: opts.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= opts.MaxRedirects {
				return fmt.Errorf("stopped after %d redirects", opts.MaxRedirects)
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("redirect to unsupported scheme %q", req.URL.Scheme)
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFetch, err)
	}
	req.Header.Set("User-Agent", opts.UserAgent)
	req.Header.Set("Accept", "application/json, application/agent+json;q=0.9, */*;q=0.5")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFetch, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: HTTP %d %s", ErrFetch, resp.StatusCode, strings.TrimSpace(resp.Status))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, opts.MaxBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFetch, err)
	}
	if int64(len(body)) > opts.MaxBodyBytes {
		return nil, fmt.Errorf("%w: response body exceeded %d bytes", ErrFetch, opts.MaxBodyBytes)
	}
	return body, nil
}

// ErrFetch is the sentinel for any fetch-time error. Callers can use
// errors.Is(err, agentvalidate.ErrFetch) to distinguish network/URL
// problems from JSON schema errors.
var ErrFetch = errors.New("fetch failed")

// ResolveWellKnownURL is a small convenience: given a base URL
// "https://example.com", it returns
// "https://example.com/.well-known/agent.json". This is the
// conventional place to publish an agent card per the
// agent-identity-kit spec.
//
// We don't auto-fetch here — the caller decides whether to follow the
// convention. The function returns an error for non-http(s) inputs so
// a passing-the-base-URL-through-from-arbitrary-input doesn't end up
// at a file:// scheme.
func ResolveWellKnownURL(rawBase string) (string, error) {
	u, err := url.Parse(rawBase)
	if err != nil {
		return "", fmt.Errorf("invalid URL %q: %w", rawBase, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("invalid URL %q: only http and https", rawBase)
	}
	// Strip any path so we always land on /.well-known/agent.json
	// at the root.
	u.Path = ""
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/") + "/.well-known/agent.json", nil
}
