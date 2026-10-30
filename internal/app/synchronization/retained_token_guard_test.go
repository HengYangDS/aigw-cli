package synchronization

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
)

func TestMissingCandidateTokenPreservesRetainedProjection(t *testing.T) {
	root := t.TempDir()
	installName := "aigw"
	if runtime.GOOS == "windows" {
		installName += ".exe"
	}
	oldSource := filepath.Join(root, "old-"+installName)
	if err := os.WriteFile(oldSource, []byte("intact predecessor reader"), 0o700); err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(root, "data")
	retained, err := secret.VersionedEntrypointPath(dataDir, oldSource, installName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := secret.EnsureEntrypoint(oldSource, retained); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(root, "codex.toml")
	if err := os.WriteFile(target, []byte("# Existing native Codex home\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(target)
	runtime, err := cfg.ResolveRuntime(codex.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	runtime.CredentialCommand = retained
	if err := codex.SyncConfig(target, runtime); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	claudeSettings := filepath.Join(root, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(claudeSettings), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claudeSettings, []byte(`{"theme":"dark"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	afterConfig := cfg.Clone()
	afterConfig.Accounts["ready"] = domain.Account{Label: "Ready", Endpoints: domain.Endpoints{Anthropic: "https://ready.test"}}
	afterConfig.Routes["claude"] = domain.Route{Label: "Claude", Account: "ready", Model: "claude-test", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	afterConfig.SetSelectedRoute(claude.ID, "claude", "")
	afterConfig.SetClientActivation(claude.ID, true, "/opt/claude", nil)

	workerSource := filepath.Join(root, "candidate.go")
	if err := os.WriteFile(workerSource, []byte(`package main
import "os"
func main() {
    if len(os.Args) != 4 { os.Exit(3) }
    if os.Args[3] == "ready" {
        if os.Args[1] == "__aigw-native-credential-exists" { _, _ = os.Stdout.WriteString("1"); return }
        if os.Args[1] == "__aigw-native-credential-read" { _, _ = os.Stdout.WriteString("fixture-token"); return }
    }
    if os.Args[1] == "__aigw-native-credential-exists" { _, _ = os.Stdout.WriteString("0"); return }
    os.Exit(3)
}`), 0o600); err != nil {
		t.Fatal(err)
	}
	candidate := filepath.Join(root, "candidate-"+installName)
	build := exec.Command("go", "build", "-o", candidate, workerSource)
	build.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOPROXY=off", "GOWORK=off")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build candidate reader: %v: %s", err, output)
	}
	current, err := secret.VersionedEntrypointPath(dataDir, candidate, installName)
	if err != nil {
		t.Fatal(err)
	}
	store, err := secret.Select(secret.Selection{Backend: "keyring", Executable: candidate, KeyringProbe: func(secret.Store) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	configurationStore := &configStoreStub{}
	configurationStore.bindConfiguration(t, cfg)
	syncer := Synchronizer{Registry: builtinClients, Config: configurationStore, Secrets: store, Discovery: targetDiscovery(target), Host: hostAt(root), AIGWExecutable: candidate, CredentialPath: current}
	if err := syncer.CommitProjection(t.Context(), cfg, afterConfig, "sync"); err != nil {
		t.Fatalf("independent Claude projection failed: %v", err)
	}
	if after, err := os.ReadFile(target); err != nil || !bytes.Equal(before, after) {
		t.Fatalf("missing candidate Token changed the retained client projection: %v", err)
	}
	claudeBytes, err := os.ReadFile(claudeSettings)
	if err != nil || !strings.Contains(string(claudeBytes), "https://ready.test") {
		t.Fatalf("ready Claude Account was blocked by missing Codex Account Token: %v", err)
	}
}

func TestMissingTokenDoesNotBlockIndependentNativeClientWhenReaderUnavailable(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "codex.toml")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(target)
	codexBinding := cfg.Clients[codex.ID]
	codexBinding.Authentication = domain.AuthenticationClientNative
	codexBinding.ModelProvider = "amazon-bedrock"
	cfg.Clients[codex.ID] = codexBinding
	cfg.Accounts["missing"] = domain.Account{Label: "Missing", Endpoints: domain.Endpoints{Anthropic: "https://missing.test"}}
	cfg.Routes["claude"] = domain.Route{Label: "Claude", Account: "missing", Model: "claude-test", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.SetSelectedRoute(claude.ID, "claude", "")
	cfg.SetClientActivation(claude.ID, true, "/opt/claude", nil)
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	candidate := filepath.Join(root, "candidate-aigw")
	if err := os.WriteFile(candidate, []byte("candidate"), 0o700); err != nil {
		t.Fatal(err)
	}
	reader, err := secret.VersionedEntrypointPath(filepath.Join(root, "data"), candidate, "aigw")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(reader), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(reader, 0o700); err != nil {
		t.Fatal(err)
	}
	configurationStore := &configStoreStub{}
	configurationStore.bindConfiguration(t, cfg)
	syncer := Synchronizer{Registry: builtinClients, Config: configurationStore, Secrets: secret.NewMemoryStore(), Discovery: targetDiscovery(target), AIGWExecutable: candidate, CredentialPath: reader, Host: hostAt(root)}
	plans, err := syncer.Plan(cfg, cfg)
	if err != nil || len(plans) == 0 {
		t.Fatalf("ready native Codex has no projection plan: plans=%v error=%v", plans, err)
	}
	if action, err := syncer.CredentialEntrypointPlan(cfg); err != nil || action != CredentialEntrypointUnchanged {
		t.Fatalf("missing Claude Token planned an unused reader: action=%q error=%v", action, err)
	}
	if err := syncer.CommitProjection(t.Context(), cfg, cfg, "sync"); err != nil {
		t.Fatalf("missing Claude Token blocked independent native Codex: %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil || len(data) == 0 {
		t.Fatalf("native Codex was not projected: %v", err)
	}
}
