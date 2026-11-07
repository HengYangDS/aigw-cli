package status

import (
	"encoding/json"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/domain"
)

// Status names each client's model provider/model in text and JSON, a model
// on several protocols counts once, and the document uses the new words in its
// keys, under schema_version 3. A disabled client keeps its selection.
func TestStatusNamesEachModelProviderModel(t *testing.T) {
	runtime, cfg, out := configuredReadinessRuntime(t)
	twin := cfg.Models["claude"]
	twin.Protocols = map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}
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
