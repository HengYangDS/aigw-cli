package doctor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aigw-cli/internal/client"
	configuration "aigw-cli/internal/configuration"
	domainreadiness "aigw-cli/internal/readiness"
	"aigw-cli/internal/secrets"
	"aigw-cli/internal/synchronization"

	"github.com/spf13/cobra"
)

type commandResult struct {
	CredentialBackend secrets.BackendSelection          `json:"credential_backend"`
	Checks            []Check                           `json:"checks"`
	Clients           map[string]domainreadiness.Client `json:"clients"`
	OK                bool                              `json:"ok"`
}

func executeDoctorCommand(command *cobra.Command) error {
	command.SilenceErrors = true
	command.SilenceUsage = true
	return command.Execute()
}

func validDoctorConfig() configuration.Config {
	cfg := configuration.NewConfig()
	cfg.Accounts["team"] = configuration.Account{
		Label: "Team",
		Endpoints: configuration.Endpoints{
			Anthropic:       "https://team.test",
			OpenAIResponses: "https://team.test/v1",
		},
	}
	cfg.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "team", Client: configuration.ClientClaude, Model: "claude-test"}
	cfg.Profiles["codex"] = configuration.Profile{Label: "Codex", Account: "team", Client: configuration.ClientCodex, Model: "gpt-test"}
	cfg.Routes[configuration.ClientClaude] = "claude"
	cfg.Routes[configuration.ClientCodex] = "codex"
	return cfg
}

func doctorDependencies(t *testing.T, cfg configuration.Config) (Dependencies, *bytes.Buffer, *secrets.MemoryStore) {
	t.Helper()
	store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	secretStore := secrets.NewMemoryStore()
	return Dependencies{
		Config: store, Secrets: secretStore, Clients: synchronization.Synchronizer{Registry: client.DefaultRegistry()}, Out: out,
	}, out, secretStore
}

func executeJSON(t *testing.T, deps Dependencies) commandResult {
	t.Helper()
	cmd := NewCommand(deps)
	cmd.SetArgs([]string{"--json"})
	if err := executeDoctorCommand(cmd); err != nil {
		t.Fatal(err)
	}
	var result commandResult
	if err := json.Unmarshal(deps.Out.(*bytes.Buffer).Bytes(), &result); err != nil {
		t.Fatalf("decode doctor JSON: %v", err)
	}
	return result
}

func findCheck(t *testing.T, checks []Check, name string) Check {
	t.Helper()
	for _, check := range checks {
		if check.Name == name {
			return check
		}
	}
	t.Fatalf("missing check %q: %#v", name, checks)
	return Check{}
}

func TestHumanProjectionFailsClosedForFutureChecks(t *testing.T) {
	for _, check := range []Check{
		{Name: "future:internal", OK: true, Detail: "internal success detail"},
		{Name: "future:internal", Detail: "internal failure detail", Fix: "internal repair instruction"},
	} {
		if got := Label(check.Name); got != "Other check" {
			t.Fatalf("label = %q, want Other check", got)
		}
		want := "Check failed"
		if check.OK {
			want = "Healthy"
		}
		if got := Detail(check); got != want {
			t.Fatalf("detail = %q, want %q", got, want)
		}
		if !check.OK && Fix(check) != "aigw doctor --json" {
			t.Fatalf("fix = %q", Fix(check))
		}
	}
}

func TestNextActionFallsBackForMixedOrUnclassifiedFailures(t *testing.T) {
	for _, checks := range [][]Check{
		{
			{Name: "codex:target-1", Fix: "run `aigw sync` to reconcile this target"},
			{Name: "launcher:claude", Fix: "run `aigw repair`"},
		},
		{{Name: "config", Detail: "unexpected"}},
	} {
		if got := NextAction(checks); got != "aigw repair" {
			t.Fatalf("next action = %q", got)
		}
	}
}

func TestHumanFormattingBranches(t *testing.T) {
	labels := map[string]string{
		"environment:client-token": "Client token environment",
		"config":                   "Local configuration",
		"credential:backend":       "Credential backend",
		"secret:team":              "System secret",
		"adapter:claude":           "Claude adapter",
		"adapter:codex":            "Codex adapter",
		"projection:codex":         "Codex route",
		"codex:target-7":           "Codex configuration target 7",
	}
	for name, want := range labels {
		if got := Label(name); got != want {
			t.Errorf("Label(%q) = %q, want %q", name, got, want)
		}
	}
	details := []struct {
		check Check
		want  string
	}{
		{Check{Name: "environment:client-token", OK: true}, "No global client token environment variables detected"},
		{Check{Name: "environment:client-token", Detail: "global client token environment variables are set: OPENAI_API_KEY"}, "Global client token environment variables detected: OPENAI_API_KEY"},
		{Check{Name: "environment:client-token"}, "Global client token environment variables detected"},
		{Check{Name: "config", Detail: "valid"}, "Configuration is valid"},
		{Check{Name: "config", Detail: "not configured"}, "First-time setup is incomplete"},
		{Check{Name: "config", Detail: "read config: denied"}, "Cannot read or validate configuration"},
		{Check{Name: "credential:backend"}, "Credential storage is unavailable"},
		{Check{Name: "secret:team", OK: true}, "team · available"},
		{Check{Name: "secret:team"}, "team · missing"},
		{Check{Name: "adapter:claude", OK: true, Detail: "enabled"}, "Enabled"},
		{Check{Name: "adapter:claude", Detail: "enabled but executable is missing"}, "Enabled, but no executable is configured"},
		{Check{Name: "adapter:codex", Detail: "enabled but no Codex config target is configured"}, "Enabled, but no Codex configuration file is configured"},
		{Check{Name: "projection:codex", Detail: "unavailable"}, "Current Codex route cannot be resolved"},
		{Check{Name: "codex:target-1", OK: true}, "Matches the current route"},
		{Check{Name: "codex:target-1"}, "Does not match the current route"},
	}
	for _, test := range details {
		if got := Detail(test.check); got != test.want {
			t.Errorf("Detail(%+v) = %q, want %q", test.check, got, test.want)
		}
	}
	if got := Fix(Check{Fix: "run `aigw rotate team`"}); got != "aigw rotate team" {
		t.Fatalf("generic run fix = %q", got)
	}
	for raw, want := range map[string]string{
		"aigw doctor":       "aigw doctor",
		"run `aigw setup`":  "aigw setup",
		"run `aigw repair`": "aigw repair",
		"remove them from the parent environment; now": "Remove the variables above from the parent environment that launched this terminal",
		"inspect or restore /private/configuration":    "Inspect or restore the local configuration file",
	} {
		if got := Fix(Check{Fix: raw}); got != want {
			t.Errorf("Fix(%q) = %q, want %q", raw, got, want)
		}
	}
	if got := doctorTitle(""); got != "" {
		t.Fatalf("empty title = %q", got)
	}
	if got := strings.Join(ForbiddenClientTokenEnvironment([]string{"malformed", "OPENAI_API_KEY=one", "OPENAI_API_KEY=two"}), ","); got != "OPENAI_API_KEY" {
		t.Fatalf("forbidden environment names = %q", got)
	}
}

func TestCollectReportsConfigSecretsAndAdapterFailures(t *testing.T) {
	cfg := validDoctorConfig()
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true}
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true}
	deps, _, _ := doctorDependencies(t, cfg)
	checks := Collect(context.Background(), deps)
	for _, name := range []string{"secret:team", "adapter:claude", "adapter:codex"} {
		check := findCheck(t, checks, name)
		if check.OK || check.Fix == "" {
			t.Fatalf("%s = %#v", name, check)
		}
	}
	if findCheck(t, checks, "adapter:claude").Detail != "Claude executable is not configured" {
		t.Fatalf("checks = %#v", checks)
	}

	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{
		Enabled:    true,
		Executable: "codex",
	}
	deps, _, _ = doctorDependencies(t, cfg)
	check := findCheck(t, Collect(context.Background(), deps), "adapter:codex")
	if check.OK || check.Detail != "Codex configuration target is missing" || check.Fix != "run `aigw repair`" {
		t.Fatalf("Codex adapter check = %#v", check)
	}
	bad := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
	if err := os.MkdirAll(bad.Path(), 0o700); err != nil {
		t.Fatal(err)
	}
	broken := Collect(context.Background(), Dependencies{Config: bad, Secrets: secrets.NewMemoryStore(), Clients: synchronization.Synchronizer{Registry: client.DefaultRegistry()}})
	if check := findCheck(t, broken, "config"); check.OK || !strings.Contains(check.Fix, bad.Path()) {
		t.Fatalf("config check = %#v", check)
	}
}

func TestCollectRequiresSecretsOnlyForAccountsSelectedByActiveRoutes(t *testing.T) {
	cfg := validDoctorConfig()
	cfg.Accounts["optional"] = configuration.Account{
		Label: "Optional",
		Endpoints: configuration.Endpoints{
			Anthropic:       "https://optional.test",
			OpenAIResponses: "https://optional.test/v1",
		},
	}
	cfg.Profiles["optional"] = configuration.Profile{Label: "Optional", Account: "optional", Client: configuration.ClientCodex, Model: "gpt-optional"}
	deps, _, secretStore := doctorDependencies(t, cfg)
	if err := secretStore.Set("team", "token"); err != nil {
		t.Fatal(err)
	}

	checks := Collect(context.Background(), deps)
	if !AllOK(checks) {
		t.Fatalf("optional unconnected Account made doctor unhealthy: %#v", checks)
	}
	if check := findCheck(t, checks, "secret:team"); !check.OK {
		t.Fatalf("selected Account secret = %#v", check)
	}
	for _, check := range checks {
		if check.Name == "secret:optional" {
			t.Fatalf("unselected Account received a required secret check: %#v", check)
		}
	}
}

func TestCollectExercisesClaudeExecutableAndProjectionStates(t *testing.T) {
	cfg := validDoctorConfig()
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: filepath.Join(t.TempDir(), "missing")}
	deps, _, secretsStore := doctorDependencies(t, cfg)
	if err := secretsStore.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	checks := Collect(context.Background(), deps)
	if check := findCheck(t, checks, "adapter:claude"); check.OK || !strings.Contains(check.Detail, "executable is unavailable") {
		t.Fatalf("adapter check = %#v", check)
	}
	executable := filepath.Join(t.TempDir(), "claude")
	if runtime.GOOS == "windows" {
		executable += ".exe"
	}
	if err := os.WriteFile(executable, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executable}
	deps, _, _ = doctorDependencies(t, cfg)
	checks = Collect(context.Background(), deps)
	if check := findCheck(t, checks, "adapter:claude"); !check.OK {
		t.Fatalf("adapter check = %#v", check)
	}

	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "codex", Targets: []string{filepath.Join(t.TempDir(), "missing.toml")}}
	deps, _, _ = doctorDependencies(t, cfg)
	check := findCheck(t, Collect(context.Background(), deps), "codex:target-1")
	if check.OK || !strings.Contains(check.Detail, "read Codex config") || check.Fix != "run `aigw sync`" {
		t.Fatalf("projection check = %#v", check)
	}

	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "codex", Targets: []string{"unused"}}
	delete(cfg.Accounts, "team")
	if check := findCheck(t, adapterChecks(context.Background(), deps.Clients, cfg), "projection:codex"); check.OK {
		t.Fatalf("route check = %#v", check)
	}
	if got := adapterChecks(context.Background(), deps.Clients, configuration.NewConfig()); len(got) != len(configuration.AdmittedClientIDs()) {
		t.Fatalf("disabled adapter checks = %#v", got)
	}
}

func TestClaudeExecutableReadFailuresAreDiagnostic(t *testing.T) {
	cfg := validDoctorConfig()
	blocked := filepath.Join(t.TempDir(), "claude")
	if err := os.MkdirAll(blocked, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: blocked}
	deps, _, _ := doctorDependencies(t, cfg)
	check := findCheck(t, Collect(context.Background(), deps), "adapter:claude")
	if check.OK || !strings.Contains(check.Detail, "unavailable") {
		t.Fatalf("adapter = %#v", check)
	}
}

func TestCommandHumanAndJSONPaths(t *testing.T) {
	cfg := validDoctorConfig()
	deps, out, secretStore := doctorDependencies(t, cfg)
	deps.Inspect = func(configuration.Config) map[string]domainreadiness.Client {
		return map[string]domainreadiness.Client{
			configuration.ClientClaude: {State: domainreadiness.Configured, Profile: "claude", Account: "team"},
			configuration.ClientCodex:  {State: domainreadiness.Deferred, NextAction: "aigw sync"},
		}
	}
	if err := secretStore.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	result := executeJSON(t, deps)
	if !result.OK || !AllOK(result.Checks) {
		t.Fatalf("result = %#v", result)
	}
	if result.Clients[configuration.ClientClaude].State != domainreadiness.Configured {
		t.Fatalf("canonical clients = %#v", result.Clients)
	}

	out.Reset()
	cmd := NewCommand(deps)
	if err := executeDoctorCommand(cmd); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "No problems found") {
		t.Fatalf("human output = %s", out.String())
	}

	out.Reset()
	deps.Env = []string{"OPENAI_API_KEY=secret"}
	cmd = NewCommand(deps)
	if err := executeDoctorCommand(cmd); err == nil || !strings.Contains(err.Error(), "doctor found problems") {
		t.Fatalf("doctor error = %v", err)
	}
	if !strings.Contains(out.String(), "Remove the variables above") {
		t.Fatalf("failure output = %s", out.String())
	}

	out.Reset()
	render := &bytes.Buffer{}
	deps.RenderOut = render
	deps.Env = nil
	if err := executeDoctorCommand(NewCommand(deps)); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 || !strings.Contains(render.String(), "No problems found") ||
		!strings.Contains(render.String(), "Claude") || !strings.Contains(render.String(), "Configured") ||
		!strings.Contains(render.String(), "Codex") || !strings.Contains(render.String(), "Deferred") {
		t.Fatalf("out=%q render=%q", out.String(), render.String())
	}

	cmd = NewCommand(deps)
	cmd.SetArgs([]string{"unexpected"})
	if err := executeDoctorCommand(cmd); err == nil {
		t.Fatal("doctor accepted a positional argument")
	}
}

func TestCommandJSONExposesTheCanonicalCredentialBackendSelection(t *testing.T) {
	cfg := validDoctorConfig()
	deps, _, _ := doctorDependencies(t, cfg)
	root := filepath.Join(t.TempDir(), "secrets")
	store, err := secrets.Select(secrets.Selection{
		GOOS:         runtime.GOOS,
		Root:         root,
		KeyringProbe: func(secrets.Store) error { return errors.New("service unavailable") },
	})
	if err != nil {
		t.Fatal(err)
	}
	deps.Secrets = store

	result := executeJSON(t, deps)
	want := secrets.BackendSelection{
		Kind:         "file",
		Availability: "available",
		Mutability:   "read_write",
		Persistence:  "deferred",
	}
	if result.CredentialBackend != want {
		t.Fatalf("credential backend = %#v, want %#v", result.CredentialBackend, want)
	}
	if _, err := os.Stat(filepath.Join(root, "backend")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("doctor persisted automatic selection: %v", err)
	}
}

func TestCommandJSONReportsCredentialBackendInspectionFailure(t *testing.T) {
	cfg := validDoctorConfig()
	deps, _, _ := doctorDependencies(t, cfg)
	root := filepath.Join(t.TempDir(), "secrets")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "backend"), []byte("retired\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := secrets.Select(secrets.Selection{GOOS: runtime.GOOS, Root: root})
	if err != nil {
		t.Fatal(err)
	}
	deps.Secrets = store

	result := executeJSON(t, deps)
	if result.CredentialBackend.Availability != "unavailable" || result.CredentialBackend.RecoveryAction != "aigw doctor" {
		t.Fatalf("credential backend = %#v", result.CredentialBackend)
	}
	check := findCheck(t, result.Checks, "credential:backend")
	if check.OK || check.Fix != "aigw doctor" || !strings.Contains(check.Detail, "invalid persisted") {
		t.Fatalf("credential backend check = %#v", check)
	}
}

func TestCommandPreservesCanonicalClientsWhenInspectionFails(t *testing.T) {
	cfg := validDoctorConfig()
	deps, out, secretStore := doctorDependencies(t, cfg)
	if err := secretStore.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	deps.Inspect = func(configuration.Config) map[string]domainreadiness.Client {
		return map[string]domainreadiness.Client{
			configuration.ClientClaude: {
				State:      domainreadiness.Unavailable,
				Profile:    "claude",
				Account:    "team",
				Detail:     "Credential metadata is unavailable",
				NextAction: "aigw doctor",
			},
		}
	}

	result := executeJSON(t, deps)
	state, ok := result.Clients[configuration.ClientClaude]
	if !ok || state.State != domainreadiness.Unavailable || state.NextAction != "aigw doctor" {
		t.Fatalf("canonical clients = %#v\n%s", result.Clients, out.String())
	}

	out.Reset()
	if err := executeDoctorCommand(NewCommand(deps)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Claude") || !strings.Contains(out.String(), "Unavailable") || strings.Contains(out.String(), "credential metadata unavailable") {
		t.Fatalf("human output = %s", out.String())
	}
}

type failingWriter struct{ err error }

func (writer failingWriter) Write([]byte) (int, error) { return 0, writer.err }

func TestCommandPropagatesWriterFailures(t *testing.T) {
	cfg := validDoctorConfig()
	deps, _, secretStore := doctorDependencies(t, cfg)
	if err := secretStore.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	want := errors.New("write failed")
	deps.Out = failingWriter{err: want}
	cmd := NewCommand(deps)
	cmd.SetArgs([]string{"--json"})
	if err := executeDoctorCommand(cmd); !errors.Is(err, want) {
		t.Fatalf("JSON write error = %v", err)
	}
	deps.RenderOut = failingWriter{err: want}
	if err := executeDoctorCommand(NewCommand(deps)); !errors.Is(err, want) {
		t.Fatalf("render error = %v", err)
	}
}

func TestCommandPropagatesWriterFailureWhilePresentingProblems(t *testing.T) {
	cfg := validDoctorConfig()
	deps, _, _ := doctorDependencies(t, cfg)
	want := errors.New("write failed")
	deps.RenderOut = failingWriter{err: want}
	if err := executeDoctorCommand(NewCommand(deps)); !errors.Is(err, want) {
		t.Fatalf("problem render error = %v", err)
	}
}

func TestAllOKAndNextActionBranches(t *testing.T) {
	if !AllOK(nil) || AllOK([]Check{{Name: "bad"}}) {
		t.Fatal("AllOK result mismatch")
	}
	for _, test := range []struct {
		checks []Check
		want   string
	}{
		{[]Check{{Name: "config", Detail: "not configured"}}, "aigw setup"},
		{[]Check{{Name: "healthy", OK: true}, {Name: "codex:target-1", Fix: "run `aigw sync`"}}, "aigw sync"},
		{nil, "aigw repair"},
	} {
		if got := NextAction(test.checks); got != test.want {
			t.Errorf("NextAction(%#v) = %q, want %q", test.checks, got, test.want)
		}
	}
}
