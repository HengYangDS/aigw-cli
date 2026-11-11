package claude

import (
	"context"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform/process"
)

func TestSettingsSurfaceIdentityIsStable(t *testing.T) {
	if got := string(SettingsSurface); got != "claude-settings" {
		t.Fatalf("Claude settings surface = %q", got)
	}
}

func TestClaudeProjectionChangesWhenForwardedUpstreamIdentityChanges(t *testing.T) {
	before := domain.NewConfig(testAdmission())
	before.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{Responses: "https://first.test/v1", Anthropic: "https://first.test"}}
	before.Models[domain.NewModelRef("gateway", "model")] = qualifiedModel("Route", "gateway", "model", domain.ProtocolAnthropic)
	before.Clients[ID] = domain.ClientBinding{Model: domain.NewModelRef("gateway", "model"), Enabled: true}
	if err := before.SetForwardingEndpoint(ID, "http://127.0.0.1:8792/v1"); err != nil {
		t.Fatal(err)
	}
	after := before.Clone()
	account := after.Providers["gateway"]
	account.Endpoints = domain.Endpoints{Responses: "https://second.test/v1", Anthropic: "https://second.test"}
	after.Providers["gateway"] = account
	if !(claudeAdapter{}).ProjectionChanged(before, after) {
		t.Fatal("unchanged forwarding hid a changed credential projection identity")
	}
}

func TestClaudeAdapterApplyValidatesIntentAndRestoresObservedPreimage(t *testing.T) {
	enabledWithoutRoute := domain.NewConfig(testAdmission())
	enabledWithoutRoute.SetClientActivation(ID, true, "/usr/bin/claude", nil)
	if _, err := (claudeAdapter{}).Apply(context.Background(), adapter.Dependencies{}, domain.NewConfig(testAdmission()), enabledWithoutRoute); err == nil {
		t.Fatal("Apply() accepted an enabled adapter without a route")
	}

	configured := domain.NewConfig(testAdmission())
	configured.Providers["gateway"] = domain.Provider{Endpoints: domain.Endpoints{Anthropic: "https://gateway.test"}}
	configured.Models[domain.NewModelRef("gateway", "claude-test")] = qualifiedModel("", "gateway", "claude-test", domain.ProtocolAnthropic)
	configured.SetSelectedModel(ID, domain.NewModelRef("gateway", "claude-test"), "")
	configured.SetClientActivation(ID, true, "/usr/bin/claude", nil)
	if _, err := (claudeAdapter{}).Apply(context.Background(), adapter.Dependencies{}, domain.NewConfig(testAdmission()), configured); err == nil || !strings.Contains(err.Error(), "settings path is empty") {
		t.Fatalf("Apply() error = %v", err)
	}

	home := t.TempDir()
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	receipt, err := (claudeAdapter{}).Apply(context.Background(), adapter.Dependencies{
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

func qualifiedModel(label, account, model string, protocols ...domain.EndpointProtocol) domain.Model {
	interfaces := make(map[domain.EndpointProtocol][]domain.Capability, len(protocols))
	for _, protocol := range protocols {
		interfaces[protocol] = []domain.Capability{}
	}
	return domain.Model{
		Label: label, Provider: account, Family: model, ID: model, Protocols: interfaces,
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
