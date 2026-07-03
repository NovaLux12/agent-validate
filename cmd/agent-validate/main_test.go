// Integration tests for the agent-validate CLI. These are minimal
// end-to-end checks: build the binary, run it on a fixture, assert
// the exit code. Anything we want to test about pure Go logic lives
// in pkg/agentvalidate's tests; this file only covers CLI plumbing.

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCLIBuilds verifies the binary actually builds and prints the
// version string. Skipping if `go build` isn't available would mask
// problems, so we fail fast instead.
func TestCLIBuilds(t *testing.T) {
	repoRoot := findRepoRoot(t)
	bin := filepath.Join(t.TempDir(), "agent-validate")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/agent-validate")
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	run := exec.Command(bin, "--version")
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("--version failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "agent-validate ") {
		t.Errorf("--version output unexpected: %q", out)
	}
}

func TestCLIRejectsBadArgs(t *testing.T) {
	bin := buildCLI(t)
	run := exec.Command(bin)
	if out, err := run.CombinedOutput(); err == nil {
		t.Fatalf("expected non-zero exit for missing arg, got 0; output: %s", out)
	}
}

func TestCLIPassOnMinimal(t *testing.T) {
	bin := buildCLI(t)
	repoRoot := findRepoRoot(t)
	cmd := exec.Command(bin, "--quiet", filepath.Join(repoRoot, "examples", "minimal.agent.json"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected exit 0 on minimal example, got: %v\n%s", err, out)
	}
	_ = out
}

func TestCLIFailOnBrokens(t *testing.T) {
	bin := buildCLI(t)
	tmp := filepath.Join(t.TempDir(), "broken.json")
	if err := os.WriteFile(tmp, []byte(`{"version":"1.0","agent":{"name":"x"},"owner":{"name":"y"}}`), 0o644); err != nil {
		t.Fatalf("write tmp: %v", err)
	}
	cmd := exec.Command(bin, "--mode", "validate", tmp)
	if err := cmd.Run(); err == nil {
		t.Fatalf("expected non-zero exit on broken card, got 0")
	}
}

func TestCLIDumpSchema(t *testing.T) {
	bin := buildCLI(t)
	dest := filepath.Join(t.TempDir(), "schema.json")
	cmd := exec.Command(bin, "--dump-schema", dest)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("--dump-schema failed: %v\n%s", err, out)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if !strings.Contains(string(data), "Agent Card") {
		t.Errorf("dumped schema doesn't look like Agent Card schema, got %q", string(data[:min(60, len(data))]))
	}
}

func TestCLIStdin(t *testing.T) {
	bin := buildCLI(t)
	repoRoot := findRepoRoot(t)
	src, err := os.ReadFile(filepath.Join(repoRoot, "examples", "minimal.agent.json"))
	if err != nil {
		t.Fatalf("read minimal: %v", err)
	}
	cmd := exec.Command(bin, "--quiet", "--mode", "validate", "-")
	cmd.Stdin = strings.NewReader(string(src))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("stdin validate failed: %v\n%s", err, out)
	}
}

// findRepoRoot walks up until it finds go.mod — these integration
// tests need to know where the repo is so they can reference
// examples/ relatively. If we're not in a Go module, fail loudly.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for cur := wd; cur != "/"; cur = filepath.Dir(cur) {
		if _, err := os.Stat(filepath.Join(cur, "go.mod")); err == nil {
			return cur
		}
	}
	t.Fatalf("no go.mod above %s", wd)
	return ""
}

// buildCLI compiles the binary once and returns its path. Tests that
// share this skip the build cost.
var sharedBin string

func buildCLI(t *testing.T) string {
	t.Helper()
	if sharedBin != "" {
		if _, err := os.Stat(sharedBin); err == nil {
			return sharedBin
		}
	}
	repoRoot := findRepoRoot(t)
	sharedBin = filepath.Join(t.TempDir(), "agent-validate")
	cmd := exec.Command("go", "build", "-o", sharedBin, "./cmd/agent-validate")
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	return sharedBin
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
