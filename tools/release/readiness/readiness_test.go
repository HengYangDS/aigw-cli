package readiness

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadProductVersion(t *testing.T) {
	root := t.TempDir()
	if _, err := ReadProductVersion(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing VERSION error = %v", err)
	}
	for _, test := range []struct {
		content, want string
	}{
		{content: "1.2.3\n", want: "1.2.3"},
		{content: "1.2.3-rc.1+build.7\n", want: "1.2.3-rc.1+build.7"},
		{content: "\n"},
		{content: "1.2.3 invalid\n"},
		{content: "not-semver\n"},
		{content: "01.2.3\n"},
		{content: "1.2.3-rc.01\n"},
	} {
		t.Run(test.content, func(t *testing.T) {
			if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte(test.content), 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := ReadProductVersion(root)
			if (err == nil) != (test.want != "") || got != test.want {
				t.Fatalf("VERSION = %q, %v; want %q", got, err, test.want)
			}
		})
	}
}

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

func TestReadinessUsesParsedReleaseStability(t *testing.T) {
	for _, tc := range []struct {
		version string
		ready   bool
	}{
		{"1.2.3-rc.1+build.7", true},
		{"1.2.3-preview.1", true},
		{"1.2.3+build-rc.1", false},
		{"not-a-version-rc.1", false},
		{"1.2.3-rc.01", false},
	} {
		t.Run(tc.version, func(t *testing.T) {
			if err := ValidateVersion(tc.version); (err == nil) != tc.ready {
				t.Fatalf("readiness=%v, ready=%t", err, tc.ready)
			}
		})
	}
}
