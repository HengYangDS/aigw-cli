package codex

import (
	"aigw-cli/internal/client"
	"aigw-cli/internal/configuration"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAdoptCodexSelectionRecordsTheOneMatchingRoute(t *testing.T) {
	cfg, targets := codexAdoptionFixture(t, 1)
	native := chooseCodexModel(t, targets[0], "gpt-terra")
	routeID, err := AdoptSelection(client.Dependencies{}, &cfg)
	if err != nil || routeID != "terra" {
		t.Fatalf("adopted Route = %q, %v", routeID, err)
	}
	if runtime, err := cfg.ResolveRuntime(configuration.ClientCodex, ""); err != nil || runtime.RouteID != "terra" || runtime.Protocol != configuration.ProtocolOpenAIResponses {
		t.Fatalf("Codex binding = %#v, %v", cfg.Clients[configuration.ClientCodex], err)
	}
	if after, err := os.ReadFile(targets[0]); err != nil || string(after) != native {
		t.Fatalf("adoption wrote the Codex configuration: %v", err)
	}
}

func TestAdoptCodexSelectionNamesSelectionCommands(t *testing.T) {
	for _, test := range []struct {
		name, model string
		copy        bool
		want        []string
	}{
		{name: "none", model: "gpt-luna", want: []string{"`aigw use --for codex <route>`"}},
		{name: "several", model: "gpt-terra", copy: true, want: []string{"`aigw use --for codex terra`", "`aigw use --for codex terra-copy`"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg, targets := codexAdoptionFixture(t, 1)
			if test.copy {
				cfg.Routes["terra-copy"] = qualifiedRoute("Terra copy", "gateway", "gpt-terra", configuration.ProtocolOpenAIResponses)
				cfg.Normalize()
			}
			chooseCodexModel(t, targets[0], test.model)
			_, err := AdoptSelection(client.Dependencies{}, &cfg)
			for _, want := range test.want {
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("adoption error = %v, want %s", err, want)
				}
			}
			if cfg.SelectedRoute(configuration.ClientCodex) != "sol" {
				t.Fatal("a refused adoption changed the Codex binding")
			}
		})
	}
}

func TestAdoptCodexSelectionRefusesDisagreeingTargets(t *testing.T) {
	cfg, targets := codexAdoptionFixture(t, 2)
	chooseCodexModel(t, targets[0], "gpt-terra")
	chooseCodexModel(t, targets[1], "gpt-luna")
	if _, err := AdoptSelection(client.Dependencies{}, &cfg); err == nil || !strings.Contains(err.Error(), "aigw use --for codex") {
		t.Fatalf("disagreeing targets were adopted: %v", err)
	}
	if cfg.SelectedRoute(configuration.ClientCodex) != "sol" {
		t.Fatal("a refused adoption changed the Codex binding")
	}
}

func TestAdoptCodexSelectionLeavesUnchangedBindings(t *testing.T) {
	for name, prepare := range map[string]func(*configuration.Config, string){
		"unchanged": func(*configuration.Config, string) {},
		"disabled": func(cfg *configuration.Config, target string) {
			chooseCodexModel(t, target, "gpt-terra")
			binding := cfg.Clients[configuration.ClientCodex]
			binding.Enabled = false
			cfg.Clients[configuration.ClientCodex] = binding
		},
		"provider drift": func(_ *configuration.Config, target string) {
			current := chooseCodexModel(t, target, "gpt-terra")
			current = strings.Replace(current, `base_url = "https://gateway.test/v1"`, `base_url = "https://drift.test/v1"`, 1)
			if err := os.WriteFile(target, []byte(current), 0o600); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			cfg, targets := codexAdoptionFixture(t, 1)
			prepare(&cfg, targets[0])
			before := cfg.Clients[configuration.ClientCodex]
			routeID, err := AdoptSelection(client.Dependencies{}, &cfg)
			if err != nil || routeID != "" || cfg.Clients[configuration.ClientCodex].Route != before.Route {
				t.Fatalf("adoption = %q, %v; binding %#v", routeID, err, cfg.Clients[configuration.ClientCodex])
			}
		})
	}
}

// codexAdoptionFixture projects Route sol into count Codex targets. The selected
// Account also offers terra, and another Account offers the same model.
func codexAdoptionFixture(t *testing.T, count int) (configuration.Config, []string) {
	t.Helper()
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	cfg.Accounts["other"] = configuration.Account{Label: "Other", Endpoints: configuration.Endpoints{OpenAIResponses: "https://other.test/v1"}}
	cfg.Routes["sol"] = qualifiedRoute("Sol", "gateway", "gpt-sol", configuration.ProtocolOpenAIResponses)
	cfg.Routes["terra"] = qualifiedRoute("Terra", "gateway", "gpt-terra", configuration.ProtocolOpenAIResponses)
	cfg.Routes["other-terra"] = qualifiedRoute("Other terra", "other", "gpt-terra", configuration.ProtocolOpenAIResponses)
	targets := make([]string, 0, count)
	for range count {
		target := filepath.Join(t.TempDir(), "config.toml")
		if err := os.WriteFile(target, []byte("model = \"gpt-native\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		targets = append(targets, target)
	}
	cfg.SetSelectedRoute(configuration.ClientCodex, "sol", "")
	cfg.SetClientActivation(configuration.ClientCodex, true, "/opt/codex", targets)
	cfg.Normalize()
	runtime, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	runtime.CredentialCommand = filepath.Join(t.TempDir(), "aigw")
	for _, target := range targets {
		if err := SyncConfig(target, runtime); err != nil {
			t.Fatal(err)
		}
	}
	return cfg, targets
}

// chooseCodexModel rewrites the root model the way Codex's picker does: the
// value changes and AIGW's ownership comment disappears.
func chooseCodexModel(t *testing.T, target, model string) string {
	t.Helper()
	projected, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	native := strings.Replace(string(projected), `model = "gpt-sol" # managed by AIGW`, `model = "`+model+`"`, 1)
	if native == string(projected) {
		t.Fatalf("projection has no managed model:\n%s", projected)
	}
	if err := os.WriteFile(target, []byte(native), 0o600); err != nil {
		t.Fatal(err)
	}
	return native
}
