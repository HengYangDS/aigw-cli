package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/cli"
)

// With Claude Code's record deleted, disabling Claude Code said that it
// succeeded and left AIGW's helper and endpoint in the settings. It now
// refuses, says what stays, and leaves every file as it was; with the record
// restored as it says, the same command withdraws AIGW's settings.
func TestDisablingClaudeCodeWithoutItsRecordSaysWhatStays(t *testing.T) {
	client := projectedClients[claude.ID](t)
	record := readFile(t, client.record)
	if err := os.Remove(client.record); err != nil {
		t.Fatal(err)
	}
	before := readFile(t, client.file)
	client.output.Reset()
	if err := cli.Execute(client.app, []string{"client", "disable", claude.ID, "--json"}); err == nil {
		t.Fatalf("disabling without the record passed:\n%s", client.output)
	}
	var failure struct {
		Error      string `json:"error"`
		NextDetail string `json:"next_detail"`
	}
	if err := json.Unmarshal(client.output.Bytes(), &failure); err != nil {
		t.Fatalf("%v\n%s", err, client.output)
	}
	if !bytes.Contains([]byte(failure.Error), []byte("apiKeyHelper stays")) || !bytes.Contains([]byte(failure.NextDetail), []byte("Restore settings.json.aigw-state.json")) {
		t.Fatalf("the refusal = %+v", failure)
	}
	if after := readFile(t, client.file); !bytes.Equal(after, before) {
		t.Fatal("the refusal changed the settings")
	}
	writeFile(t, client.record, record, 0o600)
	if err := cli.Execute(client.app, []string{"client", "disable", claude.ID}); err != nil {
		t.Fatalf("disabling with the record restored: %v\n%s", err, client.output)
	}
	var settings map[string]json.RawMessage
	if data, err := os.ReadFile(client.file); err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			t.Fatal(err)
		}
	}
	if _, kept := settings["apiKeyHelper"]; kept {
		t.Fatalf("the withdrawal kept AIGW's helper: %s", readFile(t, client.file))
	}
}
