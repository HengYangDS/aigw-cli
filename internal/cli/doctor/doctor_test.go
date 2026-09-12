package doctor

import (
	"aigw-cli/internal/claude"
	"aigw-cli/internal/client"
	configuration "aigw-cli/internal/configuration"
	domainreadiness "aigw-cli/internal/readiness"
	"aigw-cli/internal/secrets"
	"aigw-cli/internal/synchronization"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

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

func doctorDependencies(t *testing.T, cfg configuration.Config) (Dependencies, *bytes.Buffer, secrets.Store) {
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

type observingDoctorSecrets struct {
	secrets.Store
	existsCalls []string
}

func (store *observingDoctorSecrets) Exists(accountID string) (bool, error) {
	store.existsCalls = append(store.existsCalls, accountID)
	return store.Store.Exists(accountID)
}

func executeJSON(t *testing.T, deps Dependencies, out *bytes.Buffer) commandResult {
	t.Helper()
	cmd := NewCommand(deps)
	cmd.SetArgs([]string{"--json"})
	commandErr := executeDoctorCommand(cmd)
	var result commandResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("decode doctor JSON: %v", err)
	}
	if (commandErr == nil) != result.OK {
		t.Fatalf("doctor outcome disagrees: error=%v, JSON ok=%v", commandErr, result.OK)
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

func TestCollectDoesNotObserveClientNativeCredentials(t *testing.T) {
	cfg := validDoctorConfig()
	cfg.Accounts["native"] = configuration.Account{
		Label: "Native",
		Endpoints: configuration.Endpoints{
			OpenAIResponses: "https://native.test/v1",
		},
	}
	cfg.Profiles["native"] = configuration.Profile{
		Label:          "Native",
		Account:        "native",
		Client:         configuration.ClientCodex,
		Model:          "native-model",
		ModelProvider:  "amazon-bedrock",
		Authentication: configuration.AuthenticationClientNative,
	}
	cfg.Routes[configuration.ClientCodex] = "native"
	deps, _, secretStore := doctorDependencies(t, cfg)
	if err := secretStore.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	observed := &observingDoctorSecrets{Store: secretStore}
	deps.Secrets = observed

	checks := Collect(context.Background(), deps)
	if len(observed.existsCalls) != 1 || observed.existsCalls[0] != "team" {
		t.Fatalf("doctor credential observations = %q, want only the Account-Token route", observed.existsCalls)
	}
	for _, check := range checks {
		if check.Name == "secret:native" {
			t.Fatalf("client-native Account received an AIGW Token check: %#v", check)
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
	deps.Clients.ClaudeSettingsPath = filepath.Join(t.TempDir(), "settings.json")
	deps.Clients.AIGWExecutable = filepath.Join(t.TempDir(), "aigw")
	checks = Collect(context.Background(), deps)
	if check := findCheck(t, checks, "adapter:claude"); check.OK || !strings.Contains(check.Detail, "not synchronized") {
		t.Fatalf("unsynchronized adapter check = %#v", check)
	}
	clientRuntime, err := cfg.ResolveRuntime(configuration.ClientClaude, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := claude.ReconcileSettings(deps.Clients.ClaudeSettingsPath, false, clientRuntime, deps.Clients.AIGWExecutable, clientRuntime.Model); err != nil {
		t.Fatal(err)
	}
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
	result := executeJSON(t, deps, out)
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
	deps, out, _ := doctorDependencies(t, cfg)
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

	result := executeJSON(t, deps, out)
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
	deps, out, _ := doctorDependencies(t, cfg)
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

	result := executeJSON(t, deps, out)
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

	result := executeJSON(t, deps, out)
	state, ok := result.Clients[configuration.ClientClaude]
	if result.OK || result.NextAction != "aigw doctor" || !ok || state.State != domainreadiness.Unavailable || state.NextAction != "aigw doctor" {
		t.Fatalf("canonical clients = %#v\n%s", result.Clients, out.String())
	}

	out.Reset()
	if err := executeDoctorCommand(NewCommand(deps)); err == nil {
		t.Fatal("doctor succeeded despite an unavailable client capability")
	}
	if !strings.Contains(out.String(), "Claude") || !strings.Contains(out.String(), "Unavailable") || !strings.Contains(out.String(), "aigw doctor") || strings.Contains(out.String(), "No problems found") {
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
	deps.Out = failingWriter{err: want}
	cmd := NewCommand(deps)
	cmd.SetArgs([]string{"--json"})
	if err := executeDoctorCommand(cmd); !errors.Is(err, want) {
		t.Fatalf("problem JSON write error = %v", err)
	}
}

func TestCommandConstructorExposesStableName(t *testing.T) {
	if got := NewCommand(Dependencies{}).Name(); got != "doctor" {
		t.Fatalf("doctor command = %q", got)
	}
}
