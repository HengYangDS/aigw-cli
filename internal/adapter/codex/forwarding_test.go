package codex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/domain"
)

// Codex's base URL is the destination that its forwarding composes: the
// forwarding service's root, then the scheme, authority and base path of the
// provider's Responses endpoint. The credential command keeps naming the
// provider's upstream, so the Token is the one of the direct connection.
func TestCodexProjectsTheForwardingDestination(t *testing.T) {
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig(testAdmission())
	cfg.Providers["dmxapi"] = domain.Provider{Label: "DMXAPI", Endpoints: domain.Endpoints{Responses: "https://www.dmxapi.cn/v1"}}
	cfg.Models[domain.NewModelRef("dmxapi", "gpt-6.1-sol")] = qualifiedModel("", "dmxapi", "gpt-6.1-sol", domain.ProtocolResponses)
	cfg.Clients[ID] = domain.ClientBinding{Model: domain.NewModelRef("dmxapi", "gpt-6.1-sol"), Enabled: true, Targets: []string{target}}
	direct, err := cfg.ResolveRuntime(ID, domain.ModelRef{})
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.SetForwardingRoot(ID, "http://127.0.0.1:8792"); err != nil {
		t.Fatal(err)
	}
	runtime, err := cfg.ResolveRuntime(ID, domain.ModelRef{})
	if err != nil {
		t.Fatal(err)
	}
	runtime.CredentialCommand = filepath.Join(t.TempDir(), "aigw")
	if _, err := ReconcileConfigs(nil, []TargetRef{HomeTarget(target)}, runtime, runtime, false); err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(written), `base_url = "http://127.0.0.1:8792/https/www.dmxapi.cn/v1"`) {
		t.Fatalf("Codex's file does not name the forwarding destination:\n%s", written)
	}
	if runtime.CredentialProjectionFingerprint(ID) != direct.CredentialProjectionFingerprint(ID) {
		t.Fatal("forwarding changed the upstream that the credential command names")
	}
}
