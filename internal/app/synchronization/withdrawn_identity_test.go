package synchronization

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/domain"
)

// withdrawnClients decides by the one judgement of a file, not by a path's
// spelling: a client whose file the next configuration keeps through a linked
// folder is not withdrawn, while a client that really loses a file, and has no
// Token that AIGW can use, is. Case is the judgement's own concern and is
// tested in internal/platform.
func TestWithdrawnClientsJudgesDroppedFilesByOneIdentity(t *testing.T) {
	syncer := Synchronizer{Stops: &Stops{}, Registry: builtinClients}
	root := t.TempDir()
	folder := filepath.Join(root, "codex")
	if err := os.MkdirAll(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(root, "linked")
	if err := os.Symlink(folder, linked); err != nil {
		t.Skipf("symbolic links are not available here: %v", err)
	}
	file := filepath.Join(folder, "config.toml")
	if err := os.WriteFile(file, []byte("approval_policy = \"never\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(folder, "second.toml")
	if err := os.WriteFile(other, []byte("approval_policy = \"never\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	config := func(targets ...string) domain.Config {
		return domain.Config{Clients: map[string]domain.ClientBinding{
			codex.ID: {Enabled: true, Targets: targets},
		}}
	}
	cases := []struct {
		name      string
		before    domain.Config
		after     domain.Config
		withdrawn bool
	}{
		{"the same file through a linked folder is kept", config(linked + "/config.toml"), config(file), false},
		{"a file the next configuration drops", config(file, other), config(file), true},
		{"a client that loses no file", config(file), config(file), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := syncer.withdrawnClients(c.before, c.after, []string{codex.ID}, nil)
			if named := slices.Contains(got, codex.ID); named != c.withdrawn {
				t.Fatalf("withdrawnClients named Codex %v, want %v (%v)", named, c.withdrawn, got)
			}
		})
	}
}
