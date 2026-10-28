package codex

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

// A verification home keeps the operator's settings but none of its extensions.
// MCP servers, plugins, hooks and notifications would otherwise start there
// without the authorization that only the operator's own home holds.
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

[memories]
generate = true

[[hooks.session_start]]
command = "audit"
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
		Memories        map[string]any `toml:"memories"`
		Hooks           map[string]any `toml:"hooks"`
	}
	if err := toml.Unmarshal(copied, &document); err != nil {
		t.Fatal(err)
	}
	if document.Notify != nil || document.MCPServers != nil || document.Plugins != nil ||
		document.Marketplaces != nil || document.Memories != nil || document.Hooks != nil {
		t.Fatalf("verification copy carried operator extensions:\n%s", copied)
	}
	if document.ReasoningEffort != "xhigh" || document.Features["memories"] != true {
		t.Fatalf("verification copy dropped operator settings:\n%s", copied)
	}
	if after, err := os.ReadFile(source); err != nil || string(after) != string(projected) {
		t.Fatalf("copy changed the operator's configuration: %v", err)
	}
}

// Codex resolves a relative model_catalog_json from the directory of the
// configuration that names it, so a copy in another home has to name the same
// file by its absolute path. The catalog stays the user's: AIGW neither copies
// nor records it.
func TestCopyProjectionKeepsAUserCatalogReachable(t *testing.T) {
	stubCodexBundledCatalog(t, ExecutableIdentity{Version: "1.0.0", SHA256: "aaaa"}, "gpt-6.1-sol")
	absolute, err := codexTOMLString(filepath.Join(t.TempDir(), "own.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, value, relative string
	}{
		{name: "beside the configuration", value: `"own-catalog.json" # the user's own`, relative: "own-catalog.json"},
		{name: "parent literal string", value: `'../shared/own catalog.json'`, relative: "../shared/own catalog.json"},
		{name: "absolute", value: absolute},
		{name: "home", value: `"~/catalogs/own.json"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := writeCodexTestConfig(t, "model_catalog_json = "+test.value+"\nmodel_reasoning_effort = \"high\"\n")
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
			var original struct {
				Catalog string `toml:"model_catalog_json"`
			}
			if err := toml.Unmarshal(projected, &original); err != nil {
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
				Catalog         string `toml:"model_catalog_json"`
				ReasoningEffort string `toml:"model_reasoning_effort"`
			}
			if err := toml.Unmarshal(copied, &document); err != nil {
				t.Fatal(err)
			}
			want := original.Catalog
			if test.relative != "" {
				want = filepath.Join(filepath.Dir(source), test.relative)
			}
			if document.Catalog != want || document.ReasoningEffort != "high" {
				t.Fatalf("copied catalog reference = %q, want %q:\n%s", document.Catalog, want, copied)
			}
			if _, err := os.Stat(codexCatalogPath(target)); !os.IsNotExist(err) {
				t.Fatalf("the copy gained an AIGW catalog beside a user-owned one: %v", err)
			}
			if after, err := os.ReadFile(source); err != nil || string(after) != string(projected) {
				t.Fatalf("copy changed the operator's configuration: %v", err)
			}
		})
	}
}

func TestCodexExtensionsLeaveEverySpelling(t *testing.T) {
	text, err := withoutCodexExtensions(`plugins.inline.enabled = true
mcp_servers = { serena = { command = "serena" } }
notify = [
  "notify-send",
  "done",
]
model = "kept"

[features]
memories = true
hooks = true

[[hooks.session_start]]
command = "audit"

[sandbox_workspace_write]
network_access = false
`)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := toml.Unmarshal([]byte(text), &document); err != nil {
		t.Fatalf("result is not TOML: %v\n%s", err, text)
	}
	for _, key := range []string{"plugins", "mcp_servers", "notify", "hooks"} {
		if _, present := document[key]; present {
			t.Errorf("extension %q remains:\n%s", key, text)
		}
	}
	features, _ := document["features"].(map[string]any)
	sandbox, _ := document["sandbox_workspace_write"].(map[string]any)
	if document["model"] != "kept" || features["memories"] != true || features["hooks"] != true || sandbox["network_access"] != false {
		t.Fatalf("settings were removed with the extensions:\n%s", text)
	}
}
