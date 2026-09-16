package readiness

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Masterminds/semver/v3"
)

func TestLocalVersionPreservesReleaseOrderingAndExactSource(t *testing.T) {
	commit := strings.Repeat("a", 40)
	for _, test := range []struct{ base, next, want string }{
		{"0.1.0-rc.115", "0.1.0-rc.116", "0.1.0-rc.115.local.100+local." + commit},
		{"1.2.3", "1.2.4", "1.2.4-local.100+local." + commit},
	} {
		got, err := LocalVersion(test.base, commit, "100")
		if err != nil || got != test.want {
			t.Fatalf("local identity=%q error=%v, want %q", got, err, test.want)
		}
		base := semver.MustParse(test.base)
		local := semver.MustParse(got)
		next := semver.MustParse(test.next)
		if !local.GreaterThan(base) || !local.LessThan(next) {
			t.Fatalf("local delivery must follow %s and precede %s: %s", base, next, local)
		}
	}
	for _, fields := range [][3]string{
		{"invalid", commit, "100"}, {"1.2.3", "HEAD", "100"}, {"1.2.3", commit, "-1"},
	} {
		if _, err := LocalVersion(fields[0], fields[1], fields[2]); err == nil {
			t.Fatalf("invalid local identity admitted: %v", fields)
		}
	}
}

func TestDeliveryVersionUsesExactSourceAndExplicitMode(t *testing.T) {
	root := t.TempDir()
	if _, err := ReadDeliveryVersion(root, true); err == nil {
		t.Fatal("missing source VERSION admitted")
	}
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("1.2.3-rc.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadDeliveryVersion(root, false); err != nil || got != "1.2.3-rc.1" {
		t.Fatalf("ordinary delivery identity=%q error=%v", got, err)
	}
	if _, err := ReadDeliveryVersion(root, true); err == nil {
		t.Fatal("local delivery accepted a directory without source history")
	}
	for _, args := range [][]string{
		{"init", "--quiet", root},
		{"-C", root, "-c", "core.hooksPath=/dev/null", "-c", "commit.gpgsign=false", "-c", "user.name=Build Test", "-c", "user.email=build@example.test", "commit", "--allow-empty", "--quiet", "-m", "test source"},
	} {
		command := exec.Command("git", args...)
		command.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2026-09-07T00:00:00Z", "GIT_COMMITTER_DATE=2026-09-07T00:00:00Z")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("prepare source identity: %v: %s", err, output)
		}
	}
	commit, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	want := "1.2.3-rc.1.local.1788739200+local." + strings.TrimSpace(string(commit))
	if got, err := ReadDeliveryVersion(root, true); err != nil || got != want {
		t.Fatalf("local identity=%q error=%v; want %q", got, err, want)
	}
}

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
