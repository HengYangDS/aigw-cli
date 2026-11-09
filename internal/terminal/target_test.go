package terminal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// HomeDirectory reads the home from either variable that names it, and none
// when both are empty or absent.
func TestHomeDirectoryReadsTheHomeFromItsVariables(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
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

func TestSnakeValueReplacesHyphens(t *testing.T) {
	for value, want := range map[string]string{"already-converged": "already_converged", "update": "update", "": ""} {
		if got := SnakeValue(value); got != want {
			t.Errorf("SnakeValue(%q) = %q, want %q", value, got, want)
		}
	}
}
