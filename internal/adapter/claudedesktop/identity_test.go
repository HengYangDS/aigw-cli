package claudedesktop

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestProfileIdentityMatchesClaudeDesktopContract(t *testing.T) {
	if !regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[1-8][a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$`).MatchString(profileID) {
		t.Fatalf("profile ID %q is not a Claude Desktop configuration UUID", profileID)
	}
}

func TestProjectionRefusesAnUnownedProfile(t *testing.T) {
	paths := PathsForLibrary(filepath.Join(t.TempDir(), "Claude-3p", "configLibrary"))
	writeFile(t, paths.Profile, []byte("{}"))
	want := "Claude Desktop AIGW profile already exists without AIGW ownership"
	if _, err := Prepare(paths, &Desired{}); err == nil || err.Error() != want {
		t.Fatalf("Prepare() error = %v, want %q", err, want)
	}
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
