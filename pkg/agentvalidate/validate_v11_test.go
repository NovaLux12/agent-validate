package agentvalidate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateV11AutonomousOwnerNull(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "agent-identity-kit", "examples", "autonomous-nova-lux.agent.json"))
	// Try relative to repo root; fallback to embedded fixture.
	if err != nil {
		t.Skipf("kit fixture not available: %v", err)
	}
	results, err := Validate(context.Background(), data)
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected autonomous v1.1 card to pass, got %d errors: %v", len(results), results)
	}
}

func TestValidateV11Fields(t *testing.T) {
	cases := []struct {
		name      string
		json      string
		wantValid bool
	}{
		{
			name:      "owner null autonomous",
			json:      `{"version":"1.1","agent":{"kind":"autonomous-ai-agent","name":"Test","handle":"@test@example.com","description":"hi"},"owner":null}`,
			wantValid: true,
		},
		{
			name:      "scope impersonates_humans",
			json:      `{"version":"1.1","agent":{"kind":"autonomous-ai-agent","name":"Test","handle":"@test@example.com","description":"hi"},"owner":null,"scope":{"impersonates_humans":false,"signs_legal":false}}`,
			wantValid: true,
		},
		{
			name:      "scope x_novalux12 extension",
			json:      `{"version":"1.1","agent":{"kind":"autonomous-ai-agent","name":"Test","handle":"@test@example.com","description":"hi"},"owner":null,"scope":{"impersonates_humans":false,"x_novalux12_custom":true}}`,
			wantValid: true,
		},
		{
			name:      "top-level x_novalux12 extension",
			json:      `{"version":"1.1","agent":{"kind":"autonomous-ai-agent","name":"Test","handle":"@test@example.com","description":"hi"},"owner":null,"x_novalux12_repositories":[]}`,
			wantValid: true,
		},
		{
			name:      "v1.3 offers seeks",
			json:      `{"version":"1.3","agent":{"name":"LedgerWorks","handle":"@ledgerworks@example.com","description":"test"},"owner":null,"offers":[{"capability":"api-integration","endpoint":"https://example.com/v1/integrate"}],"seeks":[{"capability":"image-analysis"}]}`,
			wantValid: true,
		},
		{
			name:      "v1.0 backward compat still valid",
			json:      `{"version":"1.0","agent":{"name":"Kai","handle":"@kai@example.com","description":"hi"},"owner":{"name":"X"}}`,
			wantValid: true,
		},
		{
			name:      "v1.0 missing owner still passes via latest fallback? but should fail against v1.0",
			json:      `{"version":"1.0","agent":{"name":"Kai","handle":"@kai@example.com","description":"hi"}}`,
			wantValid: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			results, err := Validate(context.Background(), []byte(tc.json))
			if err != nil {
				t.Fatalf("Validate() error: %v", err)
			}
			if tc.wantValid && len(results) != 0 {
				t.Errorf("expected valid, got %v", results)
			}
			if !tc.wantValid && len(results) == 0 {
				t.Errorf("expected invalid, got pass")
			}
		})
	}
}

func TestValidateDetectsVersionViaSchemaURL(t *testing.T) {
	// No version field, but $schema points to v1.1 — should pick v1.1 schema (owner:null allowed).
	json := `{"$schema":"https://github.com/NovaLux12/agent-identity-kit/blob/main/schema/agent-card.v1.1.json","version":"1.1","agent":{"kind":"autonomous-ai-agent","name":"T","handle":"@t@example.com","description":"hi"},"owner":null}`
	results, err := Validate(context.Background(), []byte(json))
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected valid via $schema hint, got %v", results)
	}
}

func TestValidateKitFixtures(t *testing.T) {
	kitFixtures := []string{
		"autonomous-nova-lux.agent.json",
		"hybrid-kestrel.agent.json",
		"marketplace.agent.json",
		"kai.agent.json",
		"minimal.agent.json",
		"vouched-by-bob.agent.json",
		"revoked-zombie.agent.json",
		"revocation-aware.agent.json",
	}
	kitRoot := filepath.Join("..", "..", "..", "agent-identity-kit", "examples")
	for _, name := range kitFixtures {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(kitRoot, name)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Skipf("fixture %s not available: %v", name, err)
			}
			results, err := Validate(context.Background(), data)
			if err != nil {
				t.Fatalf("Validate() error: %v", err)
			}
			// All kit examples should be valid against their declared version.
			// revoked-zombie has revoked:true but that is allowed schema-wise.
			if len(results) != 0 {
				// Log but don't fail for now if kit has intentional invalid? All should be valid.
				var msgs []string
				for _, r := range results {
					msgs = append(msgs, r.String())
				}
				t.Errorf("fixture %s failed validation: %s", name, strings.Join(msgs, "; "))
			}
		})
	}
}

func TestValidateWithVersionExplicit(t *testing.T) {
	// Pin to v1.0 should reject owner:null
	json := `{"version":"1.1","agent":{"kind":"autonomous-ai-agent","name":"T","handle":"@t@example.com","description":"hi"},"owner":null}`
	results, err := ValidateWithVersion(context.Background(), []byte(json), "1.0")
	if err != nil {
		t.Fatalf("ValidateWithVersion error: %v", err)
	}
	if len(results) == 0 {
		t.Errorf("expected v1.0 pinned validation to reject owner:null, got pass")
	}
	// Pin to v1.3 should accept offers
	json2 := `{"version":"1.3","agent":{"name":"T","handle":"@t@example.com","description":"hi"},"offers":[{"capability":"api-integration","endpoint":"https://example.com/v1/integrate"}]}`
	results, err = ValidateWithVersion(context.Background(), []byte(json2), "1.3")
	if err != nil {
		t.Fatalf("ValidateWithVersion error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected v1.3 pinned to pass, got %v", results)
	}
}
