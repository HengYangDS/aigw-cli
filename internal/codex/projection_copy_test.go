package codex

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

// A verification home receives only what AIGW projects. The operator's MCP
// servers, plugins, hooks and notifications would otherwise start there without
// the authorization that only the operator's own home holds.
func TestCopyProjectionLeavesOperatorExtensionsBehind(t *testing.T) {
	stubCodexBundledCatalog(t, ExecutableIdentity{Version: "1.0.0", SHA256: "aaaa"}, "gpt-6.1-sol")
	source := writeCodexTestConfig(t, `notify = ["notify-send"]
model_reasoning_effort = "xhigh"

[agents]
max_depth = 3

[features]
memories = true

[mcp_servers.serena]
command = "serena"

[plugins."notion@openai-api-curated"]
enabled = true

[marketplaces.local]
source = "/operator/marketplace"
`)
	ref := codexHomeTarget(source)
	ref.Executable = filepath.Join(filepath.Dir(source), "codex")
	runtime := catalogTestRuntime("gpt-6.1-sol")
	if _, err := ReconcileConfigs(nil, []TargetRef{ref}, runtime); err != nil {
		t.Fatal(err)
	}
	projected, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "verification", "config.toml")
	if err := CopyProjection(source, target); err != nil {
		t.Fatal(err)
	}
	if err := ValidateConfig(target, runtime); err != nil {
		t.Fatalf("copied projection is not valid at its destination: %v", err)
	}
	copied, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Notify          []string       `toml:"notify"`
		ReasoningEffort string         `toml:"model_reasoning_effort"`
		Features        map[string]any `toml:"features"`
		MCPServers      map[string]any `toml:"mcp_servers"`
		Plugins         map[string]any `toml:"plugins"`
		Marketplaces    map[string]any `toml:"marketplaces"`
	}
	if err := toml.Unmarshal(copied, &document); err != nil {
		t.Fatal(err)
	}
	if document.Notify != nil || document.ReasoningEffort != "" || document.Features["memories"] != nil ||
		document.MCPServers != nil || document.Plugins != nil || document.Marketplaces != nil {
		t.Fatalf("verification copy carried operator settings:\n%s", copied)
	}
	if codexSchedulerHash(string(copied)) != codexSchedulerHash(string(projected)) {
		t.Fatal("verification copy changed the scheduler policy that its sidecar records")
	}
	if after, err := os.ReadFile(source); err != nil || string(after) != string(projected) {
		t.Fatalf("copy changed the operator's configuration: %v", err)
	}
}

func TestCopyProjectionOfAnUnprojectedConfigurationIsEmpty(t *testing.T) {
	source := writeCodexTestConfig(t, "[mcp_servers.serena]\ncommand = \"serena\"\n")
	target := filepath.Join(t.TempDir(), "verification", "config.toml")
	if err := CopyProjection(source, target); err != nil {
		t.Fatal(err)
	}
	if copied, err := os.ReadFile(target); err != nil || len(copied) != 0 {
		t.Fatalf("verification copy of an unprojected configuration = %q, %v", copied, err)
	}
}
