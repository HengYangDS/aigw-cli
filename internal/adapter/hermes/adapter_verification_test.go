package hermes

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform/process"
	"aigw-cli/internal/secret"

	"go.yaml.in/yaml/v3"
)

func hermesVerificationFixture(t *testing.T) (domain.Config, domain.Runtime, adapter.Dependencies) {
	t.Helper()
	t.Setenv("HERMES_DISABLE_LAZY_INSTALLS", "0")
	executable := filepath.Join(t.TempDir(), "hermes")
	if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig(testAdmission())
	cfg.Providers["gateway"] = domain.Provider{Endpoints: domain.Endpoints{Anthropic: "https://gateway.test"}}
	cfg.Models[domain.NewModelRef("gateway", "claude-test")] = qualifiedModel("", "gateway", "claude-test", domain.ProtocolAnthropic)
	target := filepath.Join(t.TempDir(), "config.yaml")
	cfg.SetSelectedModel(ID, domain.NewModelRef("gateway", "claude-test"), domain.ProtocolAnthropic)
	cfg.SetClientActivation(ID, true, executable, []string{target})
	if err := os.WriteFile(target, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	selected, err := cfg.ResolveRuntime(ID, domain.ModelRef{})
	if err != nil {
		t.Fatal(err)
	}
	store := secret.NewMemoryStore()
	if err := store.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"installs", "tools"} {
		path := filepath.Join(filepath.Dir(target), name)
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "state"), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return cfg, selected, adapter.Dependencies{Secrets: store, AIGWExecutable: filepath.Join(t.TempDir(), "aigw")}
}

func TestHermesVerificationUsesTheOfficialSingleTurnContract(t *testing.T) {
	cfg, selected, deps := hermesVerificationFixture(t)
	t.Setenv("HERMES_GATEWAY_LOCK_DIR", filepath.Join(t.TempDir(), "operator-gateway-locks"))
	root := filepath.Dir(cfg.Clients[ID].Targets[0])
	environment := os.Environ()
	runner := &captureAdapterRunner{outputs: [][]byte{[]byte("Hermes Agent v1\n"), []byte("AIGW_OK\n")}}
	runner.observe = func(plan process.Plan) {
		requireHermesVerificationHome(t, plan, root, environment)
	}
	deps.Runner = runner
	_, err := (hermesAdapter{}).Verify(t.Context(), deps, cfg, selected, "")
	if runtime.GOOS == "windows" {
		requireHermesVerificationRefusal(t, err, runner.calls)
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"chat", "--quiet", "--query-file", "-", "--oneshot", "--max-turns", "1", "--run-budget", "45", "--ignore-rules", "--source", "tool"}
	if len(runner.plans) != 2 || !slices.Equal(runner.plans[1].Args, want) {
		t.Fatalf("Hermes verification plans = %#v", runner.plans)
	}
	requireHermesVerificationCleanup(t, runner.plans[0].Directory, root)
}

func TestHermesVerificationFailuresPreserveNativeState(t *testing.T) {
	for _, test := range []struct {
		name, want, diagnostic, response string
		stage                            int
		cause                            error
	}{
		{name: "deadline", stage: 1, cause: context.DeadlineExceeded, want: "version probe timed out"},
		{name: "cancellation", stage: 1, cause: context.Canceled, want: "version probe interrupted"},
		{name: "execution", stage: 1, cause: os.ErrPermission, want: "version probe failed"},
		{name: "version-warning", stage: 1, diagnostic: "warning: secret=must-not-leak", want: "diagnostics suppressed"},
		{name: "inference-warning", stage: 2, diagnostic: "warning: secret=must-not-leak", want: "diagnostics suppressed"},
		{name: "inference-failure", stage: 2, cause: os.ErrPermission, want: "inference failed"},
		{name: "missing-marker", stage: 2, response: "unexpected response", want: "expected AIGW_OK"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg, selected, deps := hermesVerificationFixture(t)
			root := filepath.Dir(cfg.Clients[ID].Targets[0])
			environment := os.Environ()
			runner := &captureAdapterRunner{outputs: [][]byte{[]byte("Hermes Agent v1\n"), []byte("AIGW_OK\n")}}
			if test.response != "" {
				runner.outputs[1] = []byte(test.response)
			}
			runner.observe = func(plan process.Plan) {
				requireHermesVerificationHome(t, plan, root, environment)
				if runner.calls+1 == test.stage {
					runner.err, runner.stderr = test.cause, []byte(test.diagnostic)
				}
			}
			deps.Runner = runner
			_, err := (hermesAdapter{}).Verify(t.Context(), deps, cfg, selected, "")
			if runtime.GOOS == "windows" {
				requireHermesVerificationRefusal(t, err, runner.calls)
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.want) || strings.Contains(err.Error(), "must-not-leak") || runner.calls != test.stage {
				t.Fatalf("Hermes refusal = %v after %d calls, want %q at stage %d", err, runner.calls, test.want, test.stage)
			}
			requireHermesVerificationCleanup(t, runner.plans[0].Directory, root)
		})
	}
}

func requireHermesVerificationCleanup(t *testing.T, home, root string) {
	t.Helper()
	if _, err := os.Lstat(home); !os.IsNotExist(err) {
		t.Fatalf("verification home survived cleanup: %v", err)
	}
	for name, expected := range map[string]string{"config.yaml": "{}\n", "installs/state": "installs", "tools/state": "tools"} {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil || string(data) != expected {
			t.Fatalf("verification changed native %s state: %q %v", name, data, err)
		}
	}
}

func requireHermesVerificationHome(t *testing.T, plan process.Plan, root string, originalEnvironment []string) {
	t.Helper()
	if relative, err := filepath.Rel(root, plan.Directory); err != nil || !strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		t.Fatalf("verification home %q shares the native installation root %q", plan.Directory, root)
	}
	for _, name := range []string{"HOME", "USERPROFILE", "HOMEDRIVE", "HOMEPATH"} {
		matches := func(entry string) bool { return strings.HasPrefix(entry, name+"=") }
		before, after := slices.IndexFunc(originalEnvironment, matches), slices.IndexFunc(plan.Env, matches)
		if (before < 0 && after >= 0) || (before >= 0 && (after < 0 || originalEnvironment[before] != plan.Env[after])) {
			t.Fatalf("verification changed native home variable %s", name)
		}
	}
	if !slices.Contains(plan.Env, "HERMES_HOME="+plan.Directory) ||
		!slices.Contains(plan.Env, "HERMES_DISABLE_LAZY_INSTALLS=1") || slices.Contains(plan.Env, "HERMES_DISABLE_LAZY_INSTALLS=0") {
		t.Fatal("verification must isolate Hermes settings without changing the OS installation home")
	}
	locks := slices.DeleteFunc(slices.Clone(plan.Env), func(entry string) bool { return !strings.HasPrefix(entry, "HERMES_GATEWAY_LOCK_DIR=") })
	if !slices.Equal(locks, []string{"HERMES_GATEWAY_LOCK_DIR=" + filepath.Join(plan.Directory, "gateway-locks")}) {
		t.Fatalf("verification gateway host locks = %v, want only the verification home's own", locks)
	}
	tools, err := os.Readlink(filepath.Join(plan.Directory, "tools"))
	if err != nil || tools != filepath.Join(root, "tools") {
		t.Fatalf("verification does not borrow the installed tools: %v", err)
	}
	_ = requireHermesVerificationSettings(t, plan)
}

func requireHermesVerificationSettings(t *testing.T, plan process.Plan) map[string]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(plan.Directory, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var projected struct {
		Agent struct {
			ReasoningOverrides map[string]string `yaml:"reasoning_overrides"`
		} `yaml:"agent"`
		Security map[string]*bool `yaml:"security"`
		Updates  map[string]*bool `yaml:"updates"`
		CLI      map[string]*bool `yaml:"cli"`
		Gateway  map[string]int   `yaml:"gateway"`
	}
	if err := yaml.Unmarshal(data, &projected); err != nil {
		t.Fatal(err)
	}
	if projected.Gateway["unmanaged_idle_exit_seconds"] != 2 {
		t.Fatalf("isolated Hermes verification keeps its gateway idle for %d seconds, want 2", projected.Gateway["unmanaged_idle_exit_seconds"])
	}
	if value := projected.Security["allow_lazy_installs"]; value == nil || *value {
		t.Fatal("isolated Hermes verification permits runtime dependency installation")
	}
	if value := projected.Updates["check"]; value == nil || *value {
		t.Fatal("isolated Hermes verification permits passive software update checks")
	}
	if value := projected.CLI["expose_on_path"]; value == nil || *value {
		t.Fatal("isolated Hermes verification permits native launcher publication")
	}
	return projected.Agent.ReasoningOverrides
}

func TestHermesVerificationPreservesNativeModelSettings(t *testing.T) {
	cfg, _, deps := hermesVerificationFixture(t)
	target := cfg.Clients[ID].Targets[0]
	original := []byte("agent:\n  reasoning_overrides:\n    mistral-large-3: none\nsecurity:\n  allow_lazy_installs: true\nupdates:\n  check: true\n")
	if err := os.WriteFile(target, original, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.Providers["gateway"] = domain.Provider{Endpoints: domain.Endpoints{Chat: "https://gateway.test/v1"}}
	cfg.Models[domain.NewModelRef("gateway", "mistral-large-3")] = qualifiedModel("", "gateway", "mistral-large-3", domain.ProtocolChat)
	cfg.SetSelectedModel(ID, domain.NewModelRef("gateway", "mistral-large-3"), "")
	selected, err := cfg.ResolveRuntime(ID, domain.ModelRef{})
	if err != nil {
		t.Fatal(err)
	}
	profiles := filepath.Join(filepath.Dir(target), "profiles")
	if err := os.Mkdir(profiles, 0o700); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(profiles, "user-profile")
	if err := os.WriteFile(foreign, []byte("user-owned"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &captureAdapterRunner{outputs: [][]byte{[]byte("Hermes fixture"), []byte("AIGW_OK")}}
	deps.Runner = runner
	if _, err := (hermesAdapter{}).Apply(t.Context(), deps, domain.NewConfig(testAdmission()), cfg); err != nil {
		t.Fatal(err)
	}
	original, err = os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	runner.observe = func(plan process.Plan) {
		if got := requireHermesVerificationSettings(t, plan)["mistral-large-3"]; got != "none" {
			t.Fatalf("native per-model reasoning setting was lost: got %q, want none", got)
		}
	}
	_, err = (hermesAdapter{}).Verify(t.Context(), deps, cfg, selected, "")
	if runtime.GOOS == "windows" {
		requireHermesVerificationRefusal(t, err, runner.calls)
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if after, err := os.ReadFile(target); err != nil || string(after) != string(original) {
		t.Fatal("verification changed the original native configuration")
	}
	entries, err := os.ReadDir(profiles)
	if err != nil || len(entries) != 1 || entries[0].Name() != filepath.Base(foreign) {
		t.Fatalf("verification leaked a profile or removed foreign state: %v, %v", entries, err)
	}
}

func TestHermesVerificationRequiresOneConfiguredHome(t *testing.T) {
	for _, targets := range [][]string{nil, {"one", "two"}} {
		cfg := domain.NewConfig(testAdmission())
		cfg.Clients[ID] = domain.ClientBinding{Enabled: true, Targets: targets}
		runner := &captureAdapterRunner{}
		_, err := (hermesAdapter{}).Verify(t.Context(), adapter.Dependencies{Runner: runner}, cfg, domain.Runtime{}, "")
		if err == nil || !strings.Contains(err.Error(), "one configured home") || len(runner.plans) != 0 {
			t.Fatalf("ambiguous configuration home reached native verification: %v", err)
		}
	}
}

// A verification turn must not start the operator's MCP servers or shell
// hooks: each one adds startup work to the bounded turn, and an extension that
// cannot start there must not decide the result.
func TestHermesVerificationStartsNoOperatorMCPServersOrHooks(t *testing.T) {
	for name, operator := range map[string]string{
		"plain": strings.Join([]string{
			"mcp_servers:",
			"  operator-search:",
			"    command: /operator/bin/search-mcp",
			"  operator-remote:",
			"    url: https://mcp.operator.test/v1",
			"    enabled: true",
			"hooks:",
			"  pre_llm_call:",
			"    - command: /operator/bin/guidance",
			"hooks_auto_accept: true",
			"",
		}, "\n"),
		// A line comment on a replaced key must stay with readable YAML.
		"key comments": strings.Join([]string{
			"mcp_servers: # managed by hermes mcp add",
			"  operator-search:",
			"    command: /operator/bin/search-mcp",
			"hooks: # operator guidance",
			"  pre_llm_call:",
			"    - command: /operator/bin/guidance",
			"hooks_auto_accept: true # approved once",
			"",
		}, "\n"),
	} {
		t.Run(name, func(t *testing.T) {
			requireHermesVerificationLeavesOutExtensions(t, []byte(operator))
		})
	}
}

func requireHermesVerificationLeavesOutExtensions(t *testing.T, operator []byte) {
	t.Helper()
	cfg, selected, deps := hermesVerificationFixture(t)
	target := cfg.Clients[ID].Targets[0]
	if err := os.WriteFile(target, operator, 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &captureAdapterRunner{outputs: [][]byte{[]byte("Hermes fixture"), []byte("AIGW_OK")}}
	runner.observe = func(plan process.Plan) {
		data, err := os.ReadFile(filepath.Join(plan.Directory, "config.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		var projected map[string]any
		if err := yaml.Unmarshal(data, &projected); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"mcp_servers", "hooks"} {
			if entries, isMapping := projected[key].(map[string]any); !isMapping || len(entries) != 0 {
				t.Fatalf("verification projects operator %s %v, want an explicit empty mapping", key, projected[key])
			}
		}
		if accept, present := projected["hooks_auto_accept"]; present {
			t.Fatalf("verification keeps operator hook consent %v", accept)
		}
	}
	deps.Runner = runner
	_, err := (hermesAdapter{}).Verify(t.Context(), deps, cfg, selected, "")
	if runtime.GOOS == "windows" {
		requireHermesVerificationRefusal(t, err, runner.calls)
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if runner.calls != 2 {
		t.Fatalf("verification made %d native calls, want the version probe and one turn", runner.calls)
	}
	if after, err := os.ReadFile(target); err != nil || string(after) != string(operator) {
		t.Fatalf("verification changed the operator's native configuration: %v", err)
	}
}
