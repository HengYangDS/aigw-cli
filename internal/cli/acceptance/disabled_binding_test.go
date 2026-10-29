package cli_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/cli"
	claudeclient "aigw-cli/internal/client/claude"
	"aigw-cli/internal/client/codex"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secrets"
)

// accountDenyingStore fails credential metadata reads for chosen Accounts, as
// a denied operating-system credential item does, and records every read.
type accountDenyingStore struct {
	secrets.Store
	denied   map[string]error
	observed []string
}

func (store *accountDenyingStore) Exists(account string) (bool, error) {
	store.observed = append(store.observed, account)
	if err := store.denied[account]; err != nil {
		return false, err
	}
	return store.Store.Exists(account)
}

// enabledCodexWithDisabledClaude enables Codex on Account one and keeps Claude
// Code's selected Route on Account two with its binding disabled.
func enabledCodexWithDisabledClaude(t *testing.T) (*cli.App, *bytes.Buffer, *accountDenyingStore) {
	t.Helper()
	app, out, memory, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	addAccountRoute(&cfg, "gpt", "one", "One", domain.Endpoints{OpenAIResponses: "https://one.test/v1"}, codex.ID, "gpt-test")
	addAccountRoute(&cfg, "claude", "two", "Two", domain.Endpoints{Anthropic: "https://two.test"}, claudeclient.ID, "claude-test")
	cfg.SetSelectedRoute(codex.ID, "gpt", "")
	cfg.SetSelectedRoute(claudeclient.ID, "claude", "")
	codexTarget := filepath.Join(t.TempDir(), "configuration.toml")
	writeFile(t, codexTarget, []byte("model_provider = \"native\"\n"), 0o600)
	cfg.SetClientActivation(codex.ID, true, executableFixture(t, "codex"), []string{codexTarget})
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	for _, account := range []string{"one", "two"} {
		if err := memory.Set(account, "token-"+account); err != nil {
			t.Fatal(err)
		}
	}
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatal(err)
	}
	store := &accountDenyingStore{Store: memory, denied: map[string]error{}}
	app.Secrets = store
	return app, out, store
}

func TestDisabledBindingAccountCannotChangeReadiness(t *testing.T) {
	app, out, store := enabledCodexWithDisabledClaude(t)
	commands := [][]string{{"status", "--json"}, {"check", "--json"}, {"doctor", "--json"}}
	observe := func() []string {
		documents := make([]string, len(commands))
		for index, arguments := range commands {
			out.Reset()
			// check and doctor exit non-zero on findings; their documents are compared.
			_ = cli.Execute(app, arguments)
			documents[index] = out.String()
		}
		return documents
	}
	available := observe()
	store.denied["two"] = errors.New("credential metadata denied for Account two")
	store.observed = nil
	denied := observe()

	for index, arguments := range commands {
		if denied[index] != available[index] {
			t.Errorf("%s changed when the disabled binding's Account became unobservable:\navailable: %s\ndenied: %s", strings.Join(arguments, " "), available[index], denied[index])
		}
	}
	if slices.Contains(store.observed, "two") {
		t.Errorf("readiness read the disabled binding's Account: %v", store.observed)
	}
	var status struct {
		NextAction string `json:"next_action"`
		Clients    map[string]struct {
			State  string `json:"state"`
			Detail string `json:"detail"`
		} `json:"clients"`
	}
	if err := json.Unmarshal([]byte(denied[0]), &status); err != nil {
		t.Fatal(err)
	}
	if claude := status.Clients[claudeclient.ID]; claude.State != "deferred" || claude.Detail != "The client is not installed or enabled" || status.NextAction == "aigw doctor" {
		t.Errorf("status with an unobservable disabled binding = %+v", status)
	}
}
