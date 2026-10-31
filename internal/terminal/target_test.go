package terminal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHomeTargetNamesOnlyFilesUnderHome(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	inside := filepath.Join(home, ".codex", "config.toml")
	outside := filepath.Join(root, "work", "codex", "config.toml")
	for _, test := range []struct {
		path, home, target string
		outside            bool
	}{
		{inside, home, "~/.codex/config.toml", false},
		{outside, home, "", true},
		{inside, "", "", false},
		{home, home, "", false},
		{"relative.toml", home, "", false},
	} {
		target, outside := HomeTarget(test.path, test.home)
		if target != test.target || outside != test.outside {
			t.Errorf("HomeTarget(%q, %q) = %q, %v; want %q, %v", test.path, test.home, target, outside, test.target, test.outside)
		}
	}
	for env, want := range map[string]string{
		"PATH=/bin\x00USERPROFILE=" + home:   home,
		"HOME=" + home + "\x00USERPROFILE=x": home,
		"HOME=\x00PATH=/bin":                 "",
	} {
		if got := HomeDirectory(strings.Split(env, "\x00")); got != want {
			t.Errorf("HomeDirectory(%q) = %q, want %q", env, got, want)
		}
	}
}

// TestHomeTargetFollowsALinkedHome names a resolved path below a home that a
// symbolic link names, as macOS's /var is.
func TestHomeTargetFollowsALinkedHome(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(home, link); err != nil {
		t.Skipf("symbolic links are unavailable: %v", err)
	}
	resolved, err := filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatal(err)
	}
	if target, outside := HomeTarget(filepath.Join(resolved, "file"), link); target != "~/file" || outside {
		t.Errorf("a path below the home that a link names = %q, %v", target, outside)
	}
	// A path that names home through the link, below a directory that does
	// not exist yet, lies under the resolved home too.
	if target, outside := HomeTarget(filepath.Join(link, ".codex", "config.toml"), resolved); target != "~/.codex/config.toml" || outside {
		t.Errorf("a path through the link below the resolved home = %q, %v", target, outside)
	}
	// A link at the file itself does not move the file out of home.
	elsewhere := filepath.Join(root, "elsewhere.json")
	if err := os.WriteFile(elsewhere, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, filepath.Join(home, "settings.json")); err != nil {
		t.Fatal(err)
	}
	if target, outside := HomeTarget(filepath.Join(link, "settings.json"), resolved); target != "~/settings.json" || outside {
		t.Errorf("a linked file below home = %q, %v", target, outside)
	}
}

func TestSnakeValueReplacesHyphens(t *testing.T) {
	for value, want := range map[string]string{"already-converged": "already_converged", "update": "update", "": ""} {
		if got := SnakeValue(value); got != want {
			t.Errorf("SnakeValue(%q) = %q, want %q", value, got, want)
		}
	}
}
