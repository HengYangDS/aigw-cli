package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"aigw-cli/internal/adapter/claudedesktop"
	"aigw-cli/internal/cli"
)

// Claude Desktop's profile without AIGW's record looped: sync refused and named
// the check, which named sync. Every surface now names the one recovery, and
// deleting the profile and AIGW's entry, as it says, lets sync and check pass.
func TestClaudeDesktopWithoutItsRecordNamesOneRecovery(t *testing.T) {
	client := projectedClients[claudedesktop.ID](t)
	recovery := "Restore " + filepath.Base(client.record) + " in " + filepath.Base(client.file) + "'s folder from a copy of it, or delete " + filepath.Base(client.file) + " there and remove AIGW's entry and appliedId from _meta.json, then run `aigw sync`"
	if err := os.Remove(client.record); err != nil {
		t.Fatal(err)
	}
	assertRecoveryOnEverySurface(t, client.app, client.output, claudedesktop.ID, recovery, client.directory(), client.surfaces())
	if err := os.Remove(client.file); err != nil {
		t.Fatal(err)
	}
	metadataPath := filepath.Join(client.directory(), "_meta.json")
	var metadata map[string]any
	if err := json.Unmarshal(readFile(t, metadataPath), &metadata); err != nil {
		t.Fatal(err)
	}
	delete(metadata, "appliedId")
	delete(metadata, "entries")
	data, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, metadataPath, data, 0o600)
	syncAndCheckPass(t, client.app, client.output, claudedesktop.ID)
	if err := cli.Execute(client.app, []string{"client", "disable", claudedesktop.ID}); err != nil {
		t.Fatalf("withdrawal after the recovery: %v\n%s", err, client.output)
	}
}
