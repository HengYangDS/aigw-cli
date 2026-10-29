package claude

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/client"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secrets"
)

func TestClaudeVerificationBoundsClientProcesses(t *testing.T) {
	claudeExecutable := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(claudeExecutable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	claudeConfig := domain.NewConfig(testAdmission())
	claudeConfig.Accounts["gateway"] = domain.Account{Endpoints: domain.Endpoints{Anthropic: "https://gateway.test"}}
	claudeConfig.Routes["claude"] = qualifiedRoute("", "gateway", "claude-test", domain.ProtocolAnthropic)
	claudeConfig.SetSelectedRoute(ID, "claude", "")
	claudeConfig.SetClientActivation(ID, true, claudeExecutable, nil)
	claudeRuntime, err := claudeConfig.ResolveRuntime(ID, "")
	if err != nil {
		t.Fatal(err)
	}
	secretStore := secrets.NewMemoryStore()
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	claudeRunner := &captureAdapterRunner{err: errors.New("stop after deadline observation")}
	home := t.TempDir()
	settings := filepath.Join(home, ".claude", "settings.json")
	aigwExecutable := filepath.Join(t.TempDir(), "aigw")
	if _, err := ReconcileSettings(settings, false, claudeRuntime, aigwExecutable, claudeRuntime.Model); err != nil {
		t.Fatal(err)
	}
	_, _ = (claudeAdapter{}).Verify(context.Background(), client.Dependencies{Runner: claudeRunner, Secrets: secretStore, Host: hostAt(home), AIGWExecutable: aigwExecutable}, claudeConfig, claudeRuntime, "")
	if len(claudeRunner.deadlines) != 1 || !claudeRunner.deadlines[0] {
		t.Fatalf("Claude verification deadlines = %#v", claudeRunner.deadlines)
	}
}

func TestClaudeVerificationRequiresTheSynchronizedProjection(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "claude")
	if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig(testAdmission())
	cfg.SetClientActivation(ID, true, executable, nil)
	store := secrets.NewMemoryStore()
	if err := store.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	runner := &captureAdapterRunner{err: errors.New("client must not start")}
	deps := client.Dependencies{Runner: runner, Secrets: store, Host: hostAt(root), AIGWExecutable: filepath.Join(root, "aigw")}
	runtime := domain.Runtime{AccountID: "gateway", RouteID: "claude", Model: "claude-model", Endpoint: "https://gateway.test"}
	_, err := (claudeAdapter{}).Verify(context.Background(), deps, cfg, runtime, "")
	if err == nil || !strings.Contains(err.Error(), "not synchronized") || runner.calls != 0 {
		t.Fatalf("unsynchronized projection: calls=%d error=%v", runner.calls, err)
	}
}

func TestClaudeAdapterReportsExecutableAndSecretFailures(t *testing.T) {
	runtime := domain.Runtime{AccountID: "gateway"}
	secretStore := secrets.NewMemoryStore()
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	dependencies := client.Dependencies{Secrets: secretStore}
	cfg := domain.NewConfig(testAdmission())
	if _, err := (claudeAdapter{}).Verify(context.Background(), dependencies, cfg, runtime, ""); err == nil || !strings.Contains(err.Error(), "adapter is disabled") {
		t.Fatalf("Verify() disabled error = %v", err)
	}
	cfg.SetClientActivation(ID, true, "", nil)
	if _, err := (claudeAdapter{}).Verify(context.Background(), dependencies, cfg, runtime, ""); err == nil || !strings.Contains(err.Error(), "adapter is disabled") {
		t.Fatalf("Verify() unconfigured error = %v", err)
	}

	loop := filepath.Join(t.TempDir(), "loop")
	if err := os.Symlink(loop, loop); err != nil {
		t.Skipf("symbolic link unavailable: %v", err)
	}
	cfg.SetClientActivation(ID, true, loop, nil)
	status := (claudeAdapter{}).Inspect(context.Background(), client.Dependencies{}, cfg, domain.Runtime{})
	if status.Ready || status.Issue != "Cannot inspect Claude executable" {
		t.Fatalf("status = %#v", status)
	}

	if _, err := (claudeAdapter{}).Verify(context.Background(), client.Dependencies{}, cfg, runtime, ""); err == nil || !strings.Contains(err.Error(), "secret store is unavailable") {
		t.Fatalf("Verify() error = %v", err)
	}
	if _, err := (claudeAdapter{}).Verify(context.Background(), dependencies, cfg, runtime, ""); err == nil || !strings.Contains(err.Error(), "inspect Claude executable") {
		t.Fatalf("Verify() inspection error = %v", err)
	}
	cfg.SetClientActivation(ID, true, filepath.Join(t.TempDir(), "missing"), nil)
	if _, err := (claudeAdapter{}).Verify(context.Background(), dependencies, cfg, runtime, ""); err == nil || !strings.Contains(err.Error(), "executable is unavailable") {
		t.Fatalf("Verify() unavailable error = %v", err)
	}
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

func TestFailedClaudeVerificationProjectionRemovesItsWorkspace(t *testing.T) {
	root := t.TempDir()
	t.Setenv("TMPDIR", root)
	_, workspace, err := isolateProjection(
		domain.NewConfig(testAdmission()), domain.Runtime{}, root,
	)
	if err == nil {
		t.Fatal("directory source unexpectedly copied as Claude settings")
	}
	if workspace != "" {
		t.Fatalf("failed projection retained workspace %q", workspace)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed projection left temporary resources: %v, %v", entries, err)
	}
}

func TestClaudeVerificationUsesAnIsolatedProjectionForAnUnselectedRoute(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "claude")
	if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig(testAdmission())
	cfg.Accounts["gateway"] = domain.Account{Endpoints: domain.Endpoints{Anthropic: "https://gateway.test"}}
	cfg.Routes["selected"] = qualifiedRoute("", "gateway", "claude-selected", domain.ProtocolAnthropic)
	cfg.Routes["alternate"] = qualifiedRoute("", "gateway", "claude-alternate", domain.ProtocolAnthropic)
	cfg.SetSelectedRoute(ID, "selected", "")
	cfg.SetClientActivation(ID, true, executable, nil)
	selected, err := cfg.ResolveRuntime(ID, "")
	if err != nil {
		t.Fatal(err)
	}
	settings := filepath.Join(root, ".claude", "settings.json")
	aigwExecutable := filepath.Join(root, "aigw")
	if _, err := ReconcileSettings(settings, false, selected, aigwExecutable, selected.Model); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := cfg.ResolveRuntime(ID, "alternate")
	if err != nil {
		t.Fatal(err)
	}
	store := secrets.NewMemoryStore()
	if err := store.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	runner := &routeVerificationRunner{}
	if _, err := (claudeAdapter{}).Verify(context.Background(), client.Dependencies{
		Runner: runner, Secrets: store, Host: hostAt(root), AIGWExecutable: aigwExecutable,
	}, cfg, runtime, "alternate"); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(before, after) {
		t.Fatal("explicit Route verification changed the selected Claude projection")
	}
	request := runner.plans[len(runner.plans)-1]
	settingsIndex := slices.Index(request.Args, "--settings")
	if settingsIndex < 0 || settingsIndex+1 >= len(request.Args) {
		t.Fatalf("verification plan has no settings path: %#v", request)
	}
	verificationSettings := request.Args[settingsIndex+1]
	if verificationSettings == settings {
		t.Fatalf("verification settings = %q, want an isolated projection", verificationSettings)
	}
	if _, err := os.Stat(filepath.Dir(verificationSettings)); !os.IsNotExist(err) {
		t.Fatalf("isolated Claude projection survived verification: %s: %v", verificationSettings, err)
	}
}

func TestClaudeVerificationRefusesUnresolvedSelectionBeforeInvocation(t *testing.T) {
	for _, failure := range []string{"selected route", "retained reader"} {
		t.Run(failure, func(t *testing.T) {
			home := t.TempDir()
			settings := filepath.Join(home, ".claude", "settings.json")
			if err := os.MkdirAll(filepath.Dir(settings), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(settings, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			executable := filepath.Join(t.TempDir(), "claude")
			if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
				t.Fatal(err)
			}
			cfg := domain.NewConfig(testAdmission())
			cfg.Accounts["gateway"] = domain.Account{Endpoints: domain.Endpoints{Anthropic: "https://gateway.test"}}
			cfg.Routes["claude"] = qualifiedRoute("", "gateway", "claude-test", domain.ProtocolAnthropic)
			binding := domain.ClientBinding{Route: "claude", Enabled: true, Executable: executable}
			cfg.Clients[ID] = binding
			selected, err := cfg.ResolveRuntime(ID, "")
			if err != nil {
				t.Fatal(err)
			}
			store := secrets.NewMemoryStore()
			if err := store.Set("gateway", "fixture-token"); err != nil {
				t.Fatal(err)
			}
			runner := &routeVerificationRunner{}
			deps := client.Dependencies{Runner: runner, Secrets: store, Host: hostAt(home)}
			want := "unknown route"
			if failure == "selected route" {
				binding.Route = "missing"
				cfg.Clients[ID] = binding
			} else {
				_, deps.AIGWExecutable = versionedReaders(t)
				if err := os.Remove(settings); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(settings, 0o700); err != nil {
					t.Fatal(err)
				}
				want = "retained"
			}
			if _, err := (claudeAdapter{}).Verify(t.Context(), deps, cfg, selected, ""); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("verification refusal = %v, want %q", err, want)
			}
			if len(runner.plans) != 0 {
				t.Fatalf("verification invoked a client before resolving its selection: %#v", runner.plans)
			}
		})
	}
}

func versionedReaders(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	paths := make([]string, 0, 2)
	for _, version := range []string{"predecessor", "successor"} {
		source := filepath.Join(root, version)
		if err := os.WriteFile(source, []byte(version), 0o700); err != nil {
			t.Fatal(err)
		}
		reader, err := credential.VersionedEntrypointPath(filepath.Join(root, "data"), source, "aigw")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := credential.EnsureEntrypoint(source, reader); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, reader)
	}
	return paths[0], paths[1]
}
