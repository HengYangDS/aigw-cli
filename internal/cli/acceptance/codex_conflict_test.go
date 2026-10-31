package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
)

// An outside edit to AIGW-managed Codex settings is refused rather than
// overwritten. Every surface that reports the refusal names the one action that
// can succeed on the refused state, so the operator is never sent to a command
// that refuses again.
func TestCodexOutsideRetryEditRecoveryDoesNotLoop(t *testing.T) {
	const recovery = "Undo the outside edit to the AIGW-managed Codex settings, then run `aigw sync`"
	app, output, secretStore, _, _ := testApp(t, "")
	target := filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, target, []byte("model = \"gpt-native\"\n"), 0o600)
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["gateway"] = domain.Account{Label: "Gateway", Endpoints: domain.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	cfg.Routes["sol"] = qualifiedRoute("Sol", "gateway", "gpt-sol", domain.ProtocolOpenAIResponses)
	cfg.SetSelectedRoute(codex.ID, "sol", "")
	cfg.SetClientActivation(codex.ID, true, executableFixture(t, "codex"), []string{target})
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("gateway", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatalf("project the Codex Route: %v\n%s", err, output)
	}
	projected := readFile(t, target)
	edited := strings.Replace(string(projected), "[model_providers.aigw]\n", "[model_providers.aigw]\nrequest_max_retries = 0\nstream_max_retries = 0\n", 1)
	if edited == string(projected) {
		t.Fatalf("fixture has no AIGW-managed provider table:\n%s", projected)
	}
	writeFile(t, target, []byte(edited), 0o600)
	refused := ownedFileSnapshot(t, filepath.Dir(target), app.Config.Path())

	for _, command := range [][]string{
		{"use", "--for", "codex", "--dry-run", "--json", "sol"},
		{"sync", "--dry-run", "--json"},
		{"sync", "--json"},
		{"check", "--json"},
		{"status", "--json"},
		{"doctor", "--json"},
	} {
		output.Reset()
		err := cli.Execute(app, command)
		// The recovery is prose, so every document gives it as next_detail.
		var result struct {
			NextAction string `json:"next_action"`
			NextDetail string `json:"next_detail"`
			Clients    map[string]struct {
				NextAction string `json:"next_action"`
				NextDetail string `json:"next_detail"`
			} `json:"clients"`
		}
		if decodeErr := json.Unmarshal(output.Bytes(), &result); decodeErr != nil {
			t.Fatalf("%v: %v (command error %v)\n%s", command, decodeErr, err, output)
		}
		if result.NextDetail != recovery || result.NextAction != "" {
			t.Errorf("%v next step = %q, %q; want the recovery from the outside edit", command, result.NextAction, result.NextDetail)
		}
		if client, listed := result.Clients[codex.ID]; listed && (client.NextDetail != recovery || client.NextAction != "") {
			t.Errorf("%v Codex next step = %q, %q; want the recovery from the outside edit", command, client.NextAction, client.NextDetail)
		}
		if after := ownedFileSnapshot(t, filepath.Dir(target), app.Config.Path()); !equalSnapshots(after, refused) {
			t.Fatalf("%v changed client files or owned state while refusing", command)
		}
	}
	output.Reset()
	if err := cli.Execute(app, []string{"doctor"}); err == nil {
		t.Fatal("doctor passed with an outside edit to managed Codex settings")
	}
	if human := output.String(); !strings.Contains(human, "Next\n  "+recovery) {
		t.Fatalf("doctor human next action:\n%s", human)
	}

	writeFile(t, target, projected, 0o600)
	output.Reset()
	if err := cli.Execute(app, []string{"sync", "--json"}); err != nil {
		t.Fatalf("sync after the outside edit was undone: %v\n%s", err, output)
	}
	output.Reset()
	if err := cli.Execute(app, []string{"check", "--json"}); err != nil {
		t.Fatalf("check after the outside edit was undone: %v\n%s", err, output)
	}
	var checked struct {
		Clients map[string]struct {
			ProjectionReady bool `json:"projection_ready"`
		} `json:"clients"`
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(output.Bytes(), &checked); err != nil || !checked.OK || !checked.Clients[codex.ID].ProjectionReady {
		t.Fatalf("check after recovery = %+v, %v\n%s", checked, err, output)
	}
}

// ownedFileSnapshot reads every file in the client directory and the AIGW
// configuration, so a refusal that writes any of them is visible.
func ownedFileSnapshot(t *testing.T, directory, configPath string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := map[string]string{configPath: string(readFile(t, configPath))}
	for _, entry := range entries {
		path := filepath.Join(directory, entry.Name())
		info, err := entry.Info()
		if err != nil {
			t.Fatal(err)
		}
		snapshot[path] = info.Mode().String() + "\n" + string(readFile(t, path))
	}
	return snapshot
}

func equalSnapshots(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for path, content := range left {
		if other, present := right[path]; !present || other != content {
			return false
		}
	}
	return true
}
