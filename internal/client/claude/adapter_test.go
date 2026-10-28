package claude

import (
	"context"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"aigw-cli/internal/client"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/process"
)

func TestClaudeProjectionChangesWhenForwardedUpstreamIdentityChanges(t *testing.T) {
	before := configuration.NewConfig()
	before.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{OpenAIResponses: "https://first.test/v1", Anthropic: "https://first.test"}}
	before.Routes["route"] = qualifiedRoute("Route", "gateway", "model", configuration.ProtocolAnthropic)
	before.Clients[configuration.ClientClaude] = configuration.ClientBinding{Route: "route", Enabled: true}
	if err := before.SetForwardingEndpoint(configuration.ClientClaude, "http://127.0.0.1:8792/v1"); err != nil {
		t.Fatal(err)
	}
	after := before.Clone()
	account := after.Accounts["gateway"]
	account.Endpoints = configuration.Endpoints{OpenAIResponses: "https://second.test/v1", Anthropic: "https://second.test"}
	after.Accounts["gateway"] = account
	if !(claudeAdapter{}).ProjectionChanged(before, after) {
		t.Fatal("unchanged forwarding hid a changed credential projection identity")
	}
}

func TestClaudeAdapterApplyValidatesIntentAndRestoresObservedPreimage(t *testing.T) {
	enabledWithoutRoute := configuration.NewConfig()
	enabledWithoutRoute.SetClientActivation(configuration.ClientClaude, true, "/usr/bin/claude", nil)
	if _, err := (claudeAdapter{}).Apply(context.Background(), client.Dependencies{}, configuration.NewConfig(), enabledWithoutRoute); err == nil {
		t.Fatal("Apply() accepted an enabled adapter without a route")
	}

	configured := configuration.NewConfig()
	configured.Accounts["gateway"] = configuration.Account{Endpoints: configuration.Endpoints{Anthropic: "https://gateway.test"}}
	configured.Routes["claude"] = qualifiedRoute("", "gateway", "claude-test", configuration.ProtocolAnthropic)
	configured.SetSelectedRoute(configuration.ClientClaude, "claude", "")
	configured.SetClientActivation(configuration.ClientClaude, true, "/usr/bin/claude", nil)
	if _, err := (claudeAdapter{}).Apply(context.Background(), client.Dependencies{}, configuration.NewConfig(), configured); err == nil || !strings.Contains(err.Error(), "settings path is empty") {
		t.Fatalf("Apply() error = %v", err)
	}

	home := t.TempDir()
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	receipt, err := (claudeAdapter{}).Apply(context.Background(), client.Dependencies{
		Host:           hostAt(home),
		AIGWExecutable: filepath.Join(t.TempDir(), "aigw"),
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

// hostAt observes home as the user's home directory, so Claude Code's settings
// file is home/.claude/settings.json.
func hostAt(home string) discovery.Host {
	return discovery.Host{GOOS: goruntime.GOOS, Home: home, Env: map[string]string{"HOME": home, "USERPROFILE": home}}
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
