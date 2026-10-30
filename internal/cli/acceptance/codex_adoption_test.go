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

func TestCodexSyncAdoptsNativeModelChoice(t *testing.T) {
	app, output, secretStore, _, httpClient := testApp(t, "")
	target := filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, target, []byte("model = \"gpt-native\"\n"), 0o600)
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["gateway"] = domain.Account{Label: "Gateway", Endpoints: domain.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	cfg.Routes["sol"] = qualifiedRoute("Sol", "gateway", "gpt-sol", domain.ProtocolOpenAIResponses)
	cfg.Routes["terra"] = qualifiedRoute("Terra", "gateway", "gpt-terra", domain.ProtocolOpenAIResponses)
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
	projected := string(readFile(t, target))
	native := strings.Replace(projected, `model = "gpt-sol" # managed by AIGW`, `model = "gpt-terra"`, 1)
	writeFile(t, target, []byte(native), 0o600)

	output.Reset()
	if err := cli.Execute(app, []string{"sync", "--json"}); err != nil {
		t.Fatalf("sync after a native Codex model choice: %v\n%s", err, output)
	}
	var result struct {
		Adopted map[string]string `json:"adopted_selections"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil || result.Adopted[codex.ID] != "terra" {
		t.Fatalf("adopted selections = %v, %v\n%s", result.Adopted, err, output)
	}
	if after, err := app.Config.Load(); err != nil || after.SelectedRoute(codex.ID) != "terra" {
		t.Fatalf("Codex binding was not adopted: %v", err)
	}
	adopted := string(readFile(t, target))
	for _, want := range []string{`model = "gpt-terra" # managed by AIGW`, `name = "AIGW: Terra"`} {
		if !strings.Contains(adopted, want) {
			t.Fatalf("adopted projection lacks %q:\n%s", want, adopted)
		}
	}
	for range 2 {
		output.Reset()
		if err := cli.Execute(app, []string{"sync", "--dry-run", "--json"}); err != nil {
			t.Fatalf("dry run after adoption: %v\n%s", err, output)
		}
		var preview struct {
			Adopted map[string]string `json:"adopted_selections"`
			Targets []struct {
				Client string `json:"client"`
				Action string `json:"action"`
			} `json:"targets"`
		}
		if err := json.Unmarshal(output.Bytes(), &preview); err != nil || len(preview.Adopted) != 0 {
			t.Fatalf("dry run adopted again: %v, %v\n%s", preview.Adopted, err, output)
		}
		for _, item := range preview.Targets {
			if item.Client == codex.ID && item.Action != "already-converged" {
				t.Fatalf("adopted Codex state did not converge: %+v", preview)
			}
		}
	}
	if httpClient.calls != 0 || strings.Contains(output.String(), "fixture-token") {
		t.Fatal("adoption sent requests or disclosed credentials")
	}
}

func TestCodexSyncRefusesAModelChoiceWithAnotherEdit(t *testing.T) {
	app, output, secretStore, _, _ := testApp(t, "")
	target := filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, target, []byte("model = \"gpt-native\"\n"), 0o600)
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["gateway"] = domain.Account{Label: "Gateway", Endpoints: domain.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	cfg.Routes["sol"] = qualifiedRoute("Sol", "gateway", "gpt-sol", domain.ProtocolOpenAIResponses)
	cfg.Routes["terra"] = qualifiedRoute("Terra", "gateway", "gpt-terra", domain.ProtocolOpenAIResponses)
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
	drifted := strings.Replace(string(readFile(t, target)), `model = "gpt-sol" # managed by AIGW`, `model = "gpt-terra"`, 1)
	drifted = strings.Replace(drifted, `base_url = "https://gateway.test/v1"`, `base_url = "https://drift.test/v1"`, 1)
	writeFile(t, target, []byte(drifted), 0o600)
	before, err := os.ReadFile(app.Config.Path())
	if err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"sync"}); err == nil {
		t.Fatal("sync adopted a model choice that came with a provider edit")
	}
	if string(readFile(t, target)) != drifted {
		t.Fatal("refused sync changed the Codex configuration")
	}
	if after, err := os.ReadFile(app.Config.Path()); err != nil || string(after) != string(before) {
		t.Fatalf("refused sync changed AIGW configuration: %v", err)
	}
}
