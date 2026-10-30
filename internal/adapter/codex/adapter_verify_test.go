package codex

import (
	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform/process"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCodexVerificationUsesAnIsolatedProjectionForAnUnselectedRoute(t *testing.T) {
	cfg, selected := codexVerificationFixture(t)
	target := cfg.Clients[ID].Targets[0]
	if err := DisableConfig(target); err != nil {
		t.Fatal(err)
	}
	base, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	// An absolute reference on every platform: a rooted Windows path without a
	// drive is rebased onto the configuration's drive, which would change the line.
	userCatalog := "model_catalog_json = '" + filepath.Join(t.TempDir(), "owned", "catalog.json") + "'"
	if err := os.WriteFile(target, append([]byte(userCatalog+"\n"), base...), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SyncConfig(target, selected); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Routes["alternate"] = qualifiedRoute("", "gateway", "gpt-alternate", domain.ProtocolOpenAIResponses)
	runtime, err := cfg.ResolveRuntime(ID, "alternate")
	if err != nil {
		t.Fatal(err)
	}
	runner := &routeVerificationRunner{}
	if _, err := (codexAdapter{}).Verify(context.Background(), adapter.Dependencies{
		Runner: runner, AIGWExecutable: filepath.Join(t.TempDir(), "aigw"),
	}, cfg, runtime, "alternate"); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(before, after) {
		t.Fatal("explicit Route verification changed the selected Codex projection")
	}
	request := runner.plans[len(runner.plans)-1]
	verificationHome := ""
	for _, entry := range request.Env {
		if value, ok := strings.CutPrefix(entry, "CODEX_HOME="); ok {
			verificationHome = value
			break
		}
	}
	if verificationHome == filepath.Dir(target) || verificationHome == "" {
		t.Fatalf("CODEX_HOME = %q, want an isolated projection", verificationHome)
	}
	if !strings.Contains(string(runner.config), userCatalog) {
		t.Fatalf("isolated Codex projection did not preserve user configuration:\n%s", runner.config)
	}
	if _, err := os.Stat(verificationHome); !os.IsNotExist(err) {
		t.Fatalf("isolated Codex projection survived verification: %s: %v", verificationHome, err)
	}
}

func TestFailedCodexVerificationProjectionRemovesItsWorkspace(t *testing.T) {
	root := t.TempDir()
	t.Setenv("TMPDIR", root)
	_, workspace, err := isolateProjection(
		domain.NewConfig(testAdmission()), domain.Runtime{},
		domain.ClientBinding{Targets: []string{root}},
	)
	if err == nil {
		t.Fatal("directory source unexpectedly copied as a Codex config")
	}
	if workspace != "" {
		t.Fatalf("failed projection retained workspace %q", workspace)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed projection left temporary resources: %v, %v", entries, err)
	}
}

func TestCodexVerificationRepairsCosmeticRootSelectionDriftInIsolation(t *testing.T) {
	cfg, selected := codexVerificationFixture(t)
	target := cfg.Clients[ID].Targets[0]
	projected, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	unmarked := strings.Replace(string(projected), `model_provider = "aigw" # managed by AIGW`, `model_provider = "aigw"`, 1)
	unmarked = strings.Replace(unmarked, `model = "gpt-selected" # managed by AIGW`, `model = "gpt-selected"`, 1)
	if err := os.WriteFile(target, []byte(unmarked), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Routes["alternate"] = qualifiedRoute("", "gateway", "gpt-alternate", domain.ProtocolOpenAIResponses)
	runtime, err := cfg.ResolveRuntime(ID, "alternate")
	if err != nil {
		t.Fatal(err)
	}
	runner := &routeVerificationRunner{}
	if _, err := (codexAdapter{}).Verify(context.Background(), adapter.Dependencies{
		Runner: runner, AIGWExecutable: filepath.Join(t.TempDir(), "aigw"),
	}, cfg, runtime, "alternate"); err != nil {
		t.Fatalf("explicit Route verification rejected cosmetic root selection drift: %v", err)
	}
	after, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(before, after) {
		t.Fatal("isolated verification changed the selected Codex projection")
	}
	if !strings.Contains(string(runner.config), `model = "gpt-alternate" # managed by AIGW`) {
		t.Fatalf("isolated projection did not select alternate Route:\n%s", runner.config)
	}
	if !strings.Contains(string(runner.config), `model_provider = "aigw" # managed by AIGW`) {
		t.Fatalf("isolated projection did not restore cosmetic ownership markers:\n%s", runner.config)
	}
	_ = selected
}

type routeVerificationRunner struct {
	plans     []process.Plan
	config    []byte
	challenge string
}

func (runner *routeVerificationRunner) RunCaptureStreams(ctx context.Context, plan process.Plan) ([]byte, []byte, error) {
	output, err := runner.RunCapture(ctx, plan)
	return output, nil, err
}

func (runner *routeVerificationRunner) RunCapture(_ context.Context, plan process.Plan) ([]byte, error) {
	runner.plans = append(runner.plans, plan)
	if slices.Equal(plan.Args, []string{"--version"}) {
		return []byte("codex-cli 9.9.9\n"), nil
	}
	outputPath := argumentValue(plan.Args, "--output-last-message")
	if outputPath == "" {
		if argumentValue(plan.Args, "--output-format") == "stream-json" {
			return []byte(`{"type":"result","is_error":false,"result":"AIGW_OK"}` + "\n"), nil
		}
		return []byte("AIGW_OK\n"), nil
	}
	home := environmentValue(plan.Env, "CODEX_HOME")
	var err error
	runner.config, err = os.ReadFile(filepath.Join(home, "config.toml"))
	if err != nil {
		return nil, err
	}
	tool := ""
	if !slices.Contains(plan.Args, "resume") {
		challenge, err := os.ReadFile(filepath.Join(filepath.Dir(outputPath), "challenge.txt"))
		if err != nil {
			return nil, err
		}
		runner.challenge = strings.TrimSpace(string(challenge))
		tool = fmt.Sprintf(`{"type":"item.completed","item":{"type":"command_execution","status":"completed","command":"cat challenge.txt","exit_code":0,"aggregated_output":%q}}`+"\n", runner.challenge)
	}
	if err := os.WriteFile(outputPath, []byte(runner.challenge+"\n"), 0o600); err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf(`{"type":"thread.started","thread_id":"00000000-0000-4000-8000-000000000001"}`+"\n"+`%s{"type":"item.completed","item":{"type":"agent_message","text":%q}}`+"\n"+`{"type":"turn.completed"}`+"\n", tool, runner.challenge)), nil
}

func argumentValue(arguments []string, name string) string {
	for index, argument := range arguments {
		if argument == name && index+1 < len(arguments) {
			return arguments[index+1]
		}
	}
	return ""
}

func environmentValue(environment []string, name string) string {
	prefix := name + "="
	for _, entry := range environment {
		if value, ok := strings.CutPrefix(entry, prefix); ok {
			return value
		}
	}
	return ""
}
