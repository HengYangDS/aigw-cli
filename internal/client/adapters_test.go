package client

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/claude"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secrets"
)

func TestProjectionChangesWhenForwardedUpstreamIdentityChanges(t *testing.T) {
	for _, adapter := range []Adapter{claudeAdapter{}} {
		t.Run(adapter.Spec().ID, func(t *testing.T) {
			client := adapter.Spec().ID
			before := configuration.NewConfig()
			before.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{OpenAIResponses: "https://first.test/v1", Anthropic: "https://first.test"}}
			protocol := configuration.ProtocolOpenAIResponses
			if client == configuration.ClientClaude {
				protocol = configuration.ProtocolAnthropic
			}
			before.Routes["route"] = qualifiedRoute("Route", "gateway", "model", protocol)
			before.Clients[client] = configuration.ClientBinding{Route: "route", Enabled: true}
			if err := before.SetForwardingEndpoint(client, "http://127.0.0.1:8792/v1"); err != nil {
				t.Fatal(err)
			}
			after := before.Clone()
			account := after.Accounts["gateway"]
			account.Endpoints = configuration.Endpoints{OpenAIResponses: "https://second.test/v1", Anthropic: "https://second.test"}
			after.Accounts["gateway"] = account
			if !adapter.ProjectionChanged(before, after) {
				t.Fatal("unchanged forwarding hid a changed credential projection identity")
			}
		})
	}
}

func qualifiedRoute(label, account, model string, protocols ...configuration.EndpointProtocol) configuration.Route {
	interfaces := make(map[configuration.EndpointProtocol][]configuration.Capability, len(protocols))
	for _, protocol := range protocols {
		interfaces[protocol] = []configuration.Capability{}
	}
	return configuration.Route{
		Label: label, Account: account, Model: model, UpstreamModel: model, Interfaces: interfaces,
	}
}

type captureAdapterRunner struct {
	err       error
	calls     int
	deadlines []bool
	plans     []process.Plan
	outputs   [][]byte
	stderr    []byte
	observe   func(process.Plan)
}

func (runner *captureAdapterRunner) RunCaptureStreams(ctx context.Context, plan process.Plan) ([]byte, []byte, error) {
	output, err := runner.RunCapture(ctx, plan)
	if err != nil {
		return nil, output, err
	}
	return output, runner.stderr, nil
}

func (runner *captureAdapterRunner) RunCapture(ctx context.Context, plan process.Plan) ([]byte, error) {
	if runner.observe != nil {
		runner.observe(plan)
	}
	runner.plans = append(runner.plans, plan)
	runner.calls++
	_, hasDeadline := ctx.Deadline()
	runner.deadlines = append(runner.deadlines, hasDeadline)
	if runner.calls <= len(runner.outputs) {
		return runner.outputs[runner.calls-1], runner.err
	}
	return nil, runner.err
}

func TestClaudeVerificationBoundsClientProcesses(t *testing.T) {
	claudeExecutable := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(claudeExecutable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	claudeConfig := configuration.NewConfig()
	claudeConfig.Accounts["gateway"] = configuration.Account{Endpoints: configuration.Endpoints{Anthropic: "https://gateway.test"}}
	claudeConfig.Routes["claude"] = qualifiedRoute("", "gateway", "claude-test", configuration.ProtocolAnthropic)
	claudeConfig.SetSelectedRoute(configuration.ClientClaude, "claude", "")
	claudeConfig.SetClientActivation(configuration.ClientClaude, true, claudeExecutable, nil)
	claudeRuntime, err := claudeConfig.ResolveRuntime(configuration.ClientClaude, "")
	if err != nil {
		t.Fatal(err)
	}
	secretStore := secrets.NewMemoryStore()
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	claudeRunner := &captureAdapterRunner{err: errors.New("stop after deadline observation")}
	settings := filepath.Join(t.TempDir(), "settings.json")
	aigwExecutable := filepath.Join(t.TempDir(), "aigw")
	if _, err := claude.ReconcileSettings(settings, false, claudeRuntime, aigwExecutable, claudeRuntime.Model); err != nil {
		t.Fatal(err)
	}
	_, _ = (claudeAdapter{}).Verify(context.Background(), Dependencies{Runner: claudeRunner, Secrets: secretStore, ClaudeSettingsPath: settings, AIGWExecutable: aigwExecutable}, claudeConfig, claudeRuntime, "")
	if len(claudeRunner.deadlines) != 1 || !claudeRunner.deadlines[0] {
		t.Fatalf("Claude verification deadlines = %#v", claudeRunner.deadlines)
	}
}

func TestClaudeAdapterApplyValidatesIntentAndRestoresObservedPreimage(t *testing.T) {
	enabledWithoutRoute := configuration.NewConfig()
	enabledWithoutRoute.SetClientActivation(configuration.ClientClaude, true, "/usr/bin/claude", nil)
	if _, err := (claudeAdapter{}).Apply(context.Background(), Dependencies{}, configuration.NewConfig(), enabledWithoutRoute); err == nil {
		t.Fatal("Apply() accepted an enabled adapter without a route")
	}

	configured := configuration.NewConfig()
	configured.Accounts["gateway"] = configuration.Account{Endpoints: configuration.Endpoints{Anthropic: "https://gateway.test"}}
	configured.Routes["claude"] = qualifiedRoute("", "gateway", "claude-test", configuration.ProtocolAnthropic)
	configured.SetSelectedRoute(configuration.ClientClaude, "claude", "")
	configured.SetClientActivation(configuration.ClientClaude, true, "/usr/bin/claude", nil)
	if _, err := (claudeAdapter{}).Apply(context.Background(), Dependencies{}, configuration.NewConfig(), configured); err == nil || !strings.Contains(err.Error(), "settings path is empty") {
		t.Fatalf("Apply() error = %v", err)
	}

	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	receipt, err := (claudeAdapter{}).Apply(context.Background(), Dependencies{
		ClaudeSettingsPath: settingsPath,
		AIGWExecutable:     filepath.Join(t.TempDir(), "aigw"),
	}, enabledWithoutRoute, configured)
	if err != nil {
		t.Fatal(err)
	}
	if err := receipt.Rollback(); err != nil {
		t.Fatalf("Rollback() must restore observed files without reinterpreting prior intent: %v", err)
	}
	for _, path := range []string{settingsPath, settingsPath + ".aigw-state.json"} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("projection artifact remains after rollback: %s: %v", path, err)
		}
	}
}

func TestClaudeVerificationRequiresTheSynchronizedProjection(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "claude")
	if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := configuration.NewConfig()
	cfg.SetClientActivation(configuration.ClientClaude, true, executable, nil)
	store := secrets.NewMemoryStore()
	if err := store.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	runner := &captureAdapterRunner{err: errors.New("client must not start")}
	deps := Dependencies{Runner: runner, Secrets: store, ClaudeSettingsPath: filepath.Join(root, "settings.json"), AIGWExecutable: filepath.Join(root, "aigw")}
	runtime := configuration.Runtime{AccountID: "gateway", RouteID: "claude", Model: "claude-model", Endpoint: "https://gateway.test"}
	_, err := (claudeAdapter{}).Verify(context.Background(), deps, cfg, runtime, "")
	if err == nil || !strings.Contains(err.Error(), "not synchronized") || runner.calls != 0 {
		t.Fatalf("unsynchronized projection: calls=%d error=%v", runner.calls, err)
	}
}

func TestClaudeAdapterReportsExecutableAndSecretFailures(t *testing.T) {
	runtime := configuration.Runtime{AccountID: "gateway"}
	secretStore := secrets.NewMemoryStore()
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	dependencies := Dependencies{Secrets: secretStore}
	cfg := configuration.NewConfig()
	if _, err := (claudeAdapter{}).Verify(context.Background(), dependencies, cfg, runtime, ""); err == nil || !strings.Contains(err.Error(), "adapter is disabled") {
		t.Fatalf("Verify() disabled error = %v", err)
	}
	cfg.SetClientActivation(configuration.ClientClaude, true, "", nil)
	if _, err := (claudeAdapter{}).Verify(context.Background(), dependencies, cfg, runtime, ""); err == nil || !strings.Contains(err.Error(), "adapter is disabled") {
		t.Fatalf("Verify() unconfigured error = %v", err)
	}

	loop := filepath.Join(t.TempDir(), "loop")
	if err := os.Symlink(loop, loop); err != nil {
		t.Skipf("symbolic link unavailable: %v", err)
	}
	cfg.SetClientActivation(configuration.ClientClaude, true, loop, nil)
	status := (claudeAdapter{}).Inspect(context.Background(), Dependencies{}, cfg, configuration.Runtime{})
	if status.Ready || status.Issue != "Cannot inspect Claude executable" {
		t.Fatalf("status = %#v", status)
	}

	if _, err := (claudeAdapter{}).Verify(context.Background(), Dependencies{}, cfg, runtime, ""); err == nil || !strings.Contains(err.Error(), "secret store is unavailable") {
		t.Fatalf("Verify() error = %v", err)
	}
	if _, err := (claudeAdapter{}).Verify(context.Background(), dependencies, cfg, runtime, ""); err == nil || !strings.Contains(err.Error(), "inspect Claude executable") {
		t.Fatalf("Verify() inspection error = %v", err)
	}
	cfg.SetClientActivation(configuration.ClientClaude, true, filepath.Join(t.TempDir(), "missing"), nil)
	if _, err := (claudeAdapter{}).Verify(context.Background(), dependencies, cfg, runtime, ""); err == nil || !strings.Contains(err.Error(), "executable is unavailable") {
		t.Fatalf("Verify() unavailable error = %v", err)
	}
}

func TestProjectionErrorAndInvalidRuntimeBranches(t *testing.T) {
	t.Run("invalid enabled Claude runtime", func(t *testing.T) {
		before := configuration.NewConfig()
		before.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{Anthropic: "https://gateway.test"}}
		before.Routes["claude"] = qualifiedRoute("Claude", "gateway", "claude-test", configuration.ProtocolAnthropic)
		before.SetSelectedRoute(configuration.ClientClaude, "claude", "")
		before.SetClientActivation(configuration.ClientClaude, true, "/opt/claude", nil)
		after := before.Clone()
		delete(before.Routes, "claude")
		delete(after.Routes, "claude")
		if changed := builtinRegistry().ChangedClients(before, after); len(changed) != 1 || changed[0] != configuration.ClientClaude {
			t.Fatal("invalid Claude runtime was treated as unchanged")
		}
	})
}

func TestExternalCredentialRunnerRetainsFailureWithoutUnknownSecrets(t *testing.T) {
	for _, diagnostic := range []string{
		"warning: external client secret=public-secret-marker\n",
		"ERROR external client secret=public-secret-marker\n",
		"INFO external client secret=public-secret-marker\n",
	} {
		runner := externalCredentialRunner{runner: &captureAdapterRunner{outputs: [][]byte{[]byte("AIGW_OK\n")}, stderr: []byte(diagnostic)}}
		output, stderr, err := runner.RunCaptureStreams(t.Context(), process.Plan{})
		if err != nil || string(output) != "AIGW_OK\n" || strings.Contains(string(stderr), "public-secret-marker") {
			t.Fatalf("external diagnostic privacy = %q, %v", stderr, err)
		}
		if process.DiagnosticFailure(stderr) != process.DiagnosticFailure([]byte(diagnostic)) {
			t.Fatal("external credential privacy hid a native failure")
		}
	}
}

func TestExternalCredentialRunnerPreservesCancellationWithoutDiagnostics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runner := externalCredentialRunner{runner: &captureAdapterRunner{err: errors.New("public-secret-marker")}}
	out, err := runner.RunCapture(ctx, process.Plan{})
	if !errors.Is(err, context.Canceled) || len(out) != 0 {
		t.Fatalf("cancellation=%v, output bytes=%d", err, len(out))
	}
	stdout, stderr, err := runner.RunCaptureStreams(ctx, process.Plan{})
	if !errors.Is(err, context.Canceled) || len(stdout) != 0 || len(stderr) != 0 {
		t.Fatalf("stream cancellation=%v, output bytes=%d/%d", err, len(stdout), len(stderr))
	}
}
