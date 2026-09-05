package readiness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseReadiness(t *testing.T) {
	tmp := t.TempDir()
	module := filepath.Join(tmp, "go.mod")
	if err := os.WriteFile(module, []byte("module example\n\ngo 1.27.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateToolchain(module, "go1.27.0"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateToolchain(module, "go0.0.0"); err == nil || !strings.Contains(err.Error(), "expected") {
		t.Fatalf("wrong toolchain=%v", err)
	}
	if err := os.WriteFile(module, []byte("module example\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateToolchain(module, "go1.27.0"); err == nil || !strings.Contains(err.Error(), "no Go version") {
		t.Fatalf("missing version=%v", err)
	}
	if err := ValidateToolchain(filepath.Join(tmp, "missing.mod"), "go1.27.0"); err == nil {
		t.Fatal("missing go.mod accepted")
	}

	if err := ValidateVersion("1.2.3-rc.1"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateVersion("1.2.3"); err == nil {
		t.Fatal("unsigned GA accepted")
	}
	document := filepath.Join(tmp, "readiness.md")
	if err := os.WriteFile(document, []byte("# Release readiness\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDocument(document); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(document, []byte("Current status (2026-07-14)\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDocument(document); err == nil {
		t.Fatal("stale readiness document accepted")
	}
	if err := ValidateDocument(filepath.Join(tmp, "missing-readiness")); err == nil || !strings.Contains(err.Error(), "cannot read") {
		t.Fatalf("missing readiness document=%v", err)
	}
}

func TestParseEpoch(t *testing.T) {
	instant, err := ParseEpoch("0")
	if err != nil || instant.Unix() != 0 {
		t.Fatalf("instant=%v err=%v", instant, err)
	}
	for _, raw := range []string{"-1", "not-an-epoch"} {
		if _, err := ParseEpoch(raw); err == nil {
			t.Fatalf("invalid epoch accepted: %q", raw)
		}
	}
}
