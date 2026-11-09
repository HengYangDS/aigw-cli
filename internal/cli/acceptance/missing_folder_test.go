package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli"
)

// A Codex target whose folder does not exist names that folder to create,
// on client edit and on the use and sync that a removed folder breaks, never
// another edit that would be refused the same way. Once the folder and file
// exist, the same edit passes.
func TestACodexTargetInAMissingFolderNamesTheFolder(t *testing.T) {
	client := projectedClients[codex.ID](t)
	gone := filepath.Join(t.TempDir(), "gone", "config.toml")
	want := "Create " + filepath.Dir(gone) + ", or choose another path, then run the same command again."
	for _, command := range [][]string{
		{"client", "edit", codex.ID, "--target", gone, "--dry-run", "--json"},
		{"client", "edit", codex.ID, "--target", gone, "--json"},
	} {
		client.output.Reset()
		if err := cli.Execute(client.app, command); err == nil {
			t.Fatalf("%v passed", command)
		}
		var failure struct {
			NextAction string `json:"next_action"`
			NextDetail string `json:"next_detail"`
		}
		if err := json.Unmarshal(client.output.Bytes(), &failure); err != nil {
			t.Fatalf("%v: %v\n%s", command, err, client.output)
		}
		if failure.NextDetail != want || failure.NextAction != "" {
			t.Fatalf("%v names %+v", command, failure)
		}
	}
	if err := os.MkdirAll(filepath.Dir(gone), 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, gone, nil, 0o600)
	client.output.Reset()
	if err := cli.Execute(client.app, []string{"client", "edit", codex.ID, "--target", gone}); err != nil {
		t.Fatalf("the edit after the folder exists: %v\n%s", err, client.output)
	}
}
