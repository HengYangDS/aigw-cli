package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli"
)

// A client file inside home is named as ~/… by the checks and by sync, so a
// local check exposes no absolute private path, and following the fix lets the
// same command run.
func TestACodexTargetInsideHomeNamesItsPath(t *testing.T) {
	client := projectedClients[codex.ID](t)
	gone := filepath.Join(client.app.Host.Home, "work", "config.toml")
	if err := os.MkdirAll(filepath.Dir(gone), 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, gone, []byte("approval_policy = \"never\"\n"), 0o600)
	client.output.Reset()
	if err := cli.Execute(client.app, []string{"client", "edit", codex.ID, "--target", gone}); err != nil {
		t.Fatalf("the edit with its folder there: %v\n%s", err, client.output)
	}
	if err := os.RemoveAll(filepath.Dir(gone)); err != nil {
		t.Fatal(err)
	}
	want := "Create ~/work/config.toml and the folder that holds it, or choose another path, then run the same command again."
	for _, command := range [][]string{{"check", codex.ID, "--depth", "local", "--json"}, {"status", "--json"}, {"sync", "--json"}} {
		client.output.Reset()
		_ = cli.Execute(client.app, command)
		if strings.Contains(client.output.String(), client.app.Host.Home) || !strings.Contains(client.output.String(), want) {
			t.Fatalf("%v names the private home: %s", command, client.output)
		}
	}
}

// A Codex target whose folder does not exist names that folder and the file to
// create, on client edit and on the use and sync that a removed folder breaks,
// never another edit that would be refused the same way. Following it makes
// the same command pass: the folder alone does not, because AIGW creates no
// Codex file, and the file under the folder does.
func TestACodexTargetInAMissingFolderNamesTheFolderAndFile(t *testing.T) {
	client := projectedClients[codex.ID](t)
	gone := filepath.Join(t.TempDir(), "gone", "config.toml")
	// The target lies outside this test home, so the fix names the file by its
	// own name and exposes no private folder.
	want := "Create config.toml and the folder that holds it, or choose another path, then run the same command again."
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
			t.Fatalf("%v names %+v, want %q", command, failure, want)
		}
	}
	// The folder alone leaves the same edit refused: AIGW creates no Codex
	// configuration file, so the fix names that file too.
	if err := os.MkdirAll(filepath.Dir(gone), 0o700); err != nil {
		t.Fatal(err)
	}
	client.output.Reset()
	if err := cli.Execute(client.app, []string{"client", "edit", codex.ID, "--target", gone}); err == nil {
		t.Fatalf("the edit after only the folder exists passed:\n%s", client.output)
	}
	writeFile(t, gone, nil, 0o600)
	client.output.Reset()
	if err := cli.Execute(client.app, []string{"client", "edit", codex.ID, "--target", gone}); err != nil {
		t.Fatalf("the edit after the folder and file exist: %v\n%s", err, client.output)
	}
}
