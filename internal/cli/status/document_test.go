package status

import (
	"encoding/json"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/domain"
)

// foundExecutables is a host on which discovery finds the given executables.
type foundExecutables map[string]string

func (found foundExecutables) Discover(...string) discovery.Result {
	return discovery.Result{Executables: found}
}

// Status names each client's model provider/model in text and JSON, a model
// on several protocols counts once, and the document uses the new words in its
// keys, under schema_version 3. A disabled client keeps its selection.
func TestStatusNamesEachModelProviderModel(t *testing.T) {
	runtime, cfg, out := configuredReadinessRuntime(t)
	twin := cfg.Models["claude"]
	twin.Protocols = map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolResponses: {}}
	cfg.Models["claude-responses"] = twin
	cfg.SetClientActivation(claude.ID, false, "", nil)
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Secrets.Set("one", "token"); err != nil {
		t.Fatal(err)
	}
	if err := RunStatus(runtime, false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Claude             one/claude-test · ", "Codex              one/gpt-test · "} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("status lacks %q:\n%s", want, out)
		}
	}
	out.Reset()
	if err := RunStatus(runtime, true); err != nil {
		t.Fatal(err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	var clientsDocument map[string]map[string]json.RawMessage
	if err := json.Unmarshal(document["clients"], &clientsDocument); err != nil {
		t.Fatal(err)
	}
	if string(document["schema_version"]) != "3" || string(document["models"]) != "2" || document["routes"] != nil {
		t.Fatalf("document header = %s", out)
	}
	if model, provider := string(clientsDocument[claude.ID]["model"]), string(clientsDocument[claude.ID]["provider"]); model != `"one/claude-test"` || provider != `"one"` {
		t.Fatalf("Claude Code = %s, %s", model, provider)
	}
	for client, fields := range clientsDocument {
		for key := range fields {
			for _, retired := range []string{"route", "account", "projection", "binding", "adapter"} {
				if strings.Contains(key, retired) {
					t.Errorf("%s has the key %s", client, key)
				}
			}
		}
	}
}

// An enabled client whose files AIGW has not written is told apart by
// discovery: an installed one names aigw sync, which writes them, and one that
// is not installed names installing it first. Status names the same step.
func TestStatusTellsAnInstalledClientFromOneThatIsNot(t *testing.T) {
	runtime, cfg, out := configuredReadinessRuntime(t)
	cfg.SetClientActivation(codex.ID, true, "", nil)
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Secrets.Set("one", "token"); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		found               foundExecutables
		detail, action, why string
	}{
		{foundExecutables{}, "The client is not installed", "", "Install Codex if needed, then run `aigw sync`"},
		{foundExecutables{codex.ID: "/opt/codex"}, "The client is installed, but AIGW has not written its files yet", "aigw sync", ""},
	} {
		runtime.Discovery = test.found
		out.Reset()
		if err := RunStatus(runtime, true); err != nil {
			t.Fatal(err)
		}
		var document struct {
			NextAction string                 `json:"next_action"`
			NextDetail string                 `json:"next_detail"`
			Clients    map[string]clientState `json:"clients"`
		}
		if err := json.Unmarshal(out.Bytes(), &document); err != nil {
			t.Fatal(err)
		}
		got := document.Clients[codex.ID]
		if got.Detail != test.detail || got.NextAction != test.action || got.NextDetail != test.why ||
			document.NextAction != test.action || document.NextDetail != test.why {
			t.Fatalf("Codex with %v found = %+v, status next %q, %q", test.found, got, document.NextAction, document.NextDetail)
		}
	}
}
