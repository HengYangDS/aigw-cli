package doctor

import (
	"aigw-cli/internal/app"
	domainreadiness "aigw-cli/internal/app/readiness"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/client/claude"
	"aigw-cli/internal/client/codex"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
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

// hostAt observes home as the user's home directory, so Claude Code's settings
// file is home/.claude/settings.json.
func hostAt(home string) discovery.Host {
	return discovery.Host{GOOS: runtime.GOOS, Home: home, Env: map[string]string{"HOME": home, "USERPROFILE": home}}
}

// claudeSettingsPath returns the Claude Code settings file on host.
func claudeSettingsPath(t testing.TB, host discovery.Host) string {
	t.Helper()
	path, err := claude.SettingsPath(host)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func executeDoctorCommand(command *cobra.Command) error {
	command.SilenceErrors = true
	command.SilenceUsage = true
	return command.Execute()
}

func validDoctorConfig() domain.Config {
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["team"] = domain.Account{
		Label: "Team",
		Endpoints: domain.Endpoints{
			Anthropic:       "https://team.test",
			OpenAIResponses: "https://team.test/v1",
		},
	}
	cfg.Routes["claude"] = domain.Route{Label: "Claude", Account: "team", Model: "claude-test", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.Routes["codex"] = domain.Route{Label: "Codex", Account: "team", Model: "gpt-test", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
	cfg.SetSelectedRoute(claude.ID, "claude", "")
	cfg.SetSelectedRoute(codex.ID, "codex", "")
	return cfg
}

func doctorDependencies(t *testing.T, cfg domain.Config) (invocation.Context, *bytes.Buffer, secret.Store) {
	t.Helper()
	configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	secretStore := secret.NewMemoryStore()
	return invocation.Context{
		Config: configStore, Secrets: secretStore, Clients: clients.Registry(), Out: out,
	}, out, secretStore
}

type observingDoctorSecrets struct {
	secret.Store
	existsCalls []string
}

func (store *observingDoctorSecrets) Exists(accountID string) (bool, error) {
	store.existsCalls = append(store.existsCalls, accountID)
	return store.Store.Exists(accountID)
}

func executeJSON(t *testing.T, cmd *cobra.Command, out *bytes.Buffer) commandResult {
	t.Helper()
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

func findCheck(t *testing.T, checks []checkResult, name string) checkResult {
	t.Helper()
	for _, check := range checks {
		if check.Name == name {
			return check
		}
	}
	t.Fatalf("missing check %q: %#v", name, checks)
	return checkResult{}
}

func TestCollectReportsConfigSecretsAndAdapterFailures(t *testing.T) {
	cfg := validDoctorConfig()
	cfg.SetClientActivation(claude.ID, true, "", nil)
	cfg.SetClientActivation(codex.ID, true, "", nil)
	deps, _, _ := doctorDependencies(t, cfg)
	checks := collectChecks(context.Background(), deps.Dependencies)
	if check := findCheck(t, checks, "secret:team"); check.OK || check.Fix == "" {
		t.Fatalf("missing Account Token = %#v", check)
	}
	for _, check := range checks {
		if check.Name == "adapter:claude" || check.Name == "adapter:codex" {
			t.Fatalf("deferred client must not become a failed or healthy adapter check: %#v", check)
		}
	}

	cfg.SetClientActivation(codex.ID, true, "codex", nil)
	deps, _, _ = doctorDependencies(t, cfg)
	check := findCheck(t, collectChecks(context.Background(), deps.Dependencies), "adapter:codex")
	if check.OK || check.Detail != "Codex configuration target is missing" || check.Fix != "run `aigw repair`" {
		t.Fatalf("Codex adapter check = %#v", check)
	}
	bad := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	if err := os.MkdirAll(bad.Path(), 0o700); err != nil {
		t.Fatal(err)
	}
	broken := collectChecks(context.Background(), app.Dependencies{Config: bad, Secrets: secret.NewMemoryStore(), Clients: clients.Registry()})
	if check := findCheck(t, broken, "config"); check.OK || check.Detail != "cannot read or validate configuration" || check.Fix != "inspect or restore the local configuration file" {
		t.Fatalf("config check = %#v", check)
	}
}

func TestCollectRequiresSecretsOnlyForAccountsSelectedByActiveRoutes(t *testing.T) {
	cfg := validDoctorConfig()
	cfg.SetClientActivation(claude.ID, true, "", nil)
	cfg.Accounts["optional"] = domain.Account{
		Label: "Optional",
		Endpoints: domain.Endpoints{
			Anthropic:       "https://optional.test",
			OpenAIResponses: "https://optional.test/v1",
		},
	}
	cfg.Routes["optional"] = domain.Route{Label: "Optional", Account: "optional", Model: "gpt-optional"}
	deps, _, secretStore := doctorDependencies(t, cfg)
	if err := secretStore.Set("team", "token"); err != nil {
		t.Fatal(err)
	}

	checks := collectChecks(context.Background(), deps.Dependencies)
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
	for _, client := range []string{claude.ID, codex.ID} {
		cfg.SetClientActivation(client, true, "", nil)
	}
	cfg.Accounts["native"] = domain.Account{
		Label: "Native",
		Endpoints: domain.Endpoints{
			OpenAIResponses: "https://native.test/v1",
		},
	}
	cfg.Routes["native"] = domain.Route{
		Label:   "Native",
		Account: "native",
		Model:   "native-model",
		Interfaces: map[domain.EndpointProtocol][]domain.Capability{
			domain.ProtocolOpenAIResponses: {},
		},
	}
	cfg.Clients[codex.ID] = domain.ClientBinding{
		Route: "native", Enabled: true, ModelProvider: "amazon-bedrock",
		Authentication: domain.AuthenticationClientNative,
	}
	deps, _, secretStore := doctorDependencies(t, cfg)
	if err := secretStore.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	observed := &observingDoctorSecrets{Store: secretStore}
	deps.Secrets = observed

	checks := collectChecks(context.Background(), deps.Dependencies)
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
	cfg.SetClientActivation(claude.ID, true, filepath.Join(t.TempDir(), "missing"), nil)
	deps, _, secretsStore := doctorDependencies(t, cfg)
	if err := secretsStore.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	checks := collectChecks(context.Background(), deps.Dependencies)
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
	cfg.SetClientActivation(claude.ID, true, executable, nil)
	deps, _, _ = doctorDependencies(t, cfg)
	deps.Host = hostAt(t.TempDir())
	deps.Executable = filepath.Join(t.TempDir(), "aigw")
	checks = collectChecks(context.Background(), deps.Dependencies)
	if check := findCheck(t, checks, "adapter:claude"); check.OK || !strings.Contains(check.Detail, "not synchronized") {
		t.Fatalf("unsynchronized adapter check = %#v", check)
	}
	clientRuntime, err := cfg.ResolveRuntime(claude.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := claude.ReconcileSettings(claudeSettingsPath(t, deps.Host), false, clientRuntime, deps.Executable, clientRuntime.Model); err != nil {
		t.Fatal(err)
	}
	checks = collectChecks(context.Background(), deps.Dependencies)
	if check := findCheck(t, checks, "adapter:claude"); !check.OK {
		t.Fatalf("adapter check = %#v", check)
	}

	data, err := os.ReadFile(claudeSettingsPath(t, deps.Host))
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	document["model"] = json.RawMessage(`"opus[1m]"`)
	data, _ = json.MarshalIndent(document, "", "  ")
	if err := os.WriteFile(claudeSettingsPath(t, deps.Host), append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	checks = collectChecks(context.Background(), deps.Dependencies)
	if check := findCheck(t, checks, "adapter:claude"); !check.OK || !strings.Contains(check.Detail, "native model preference") {
		t.Fatalf("native Claude model preference check = %#v", check)
	}

	home := t.TempDir()
	cfg.SetClientActivation(codex.ID, true, "codex", []string{filepath.Join(home, ".codex", "config.toml")})
	deps, _, _ = doctorDependencies(t, cfg)
	deps.Env = []string{"HOME=" + home}
	check := findCheck(t, collectChecks(context.Background(), deps.Dependencies), "codex:target-1")
	if check.OK || check.Detail != "Codex configuration target is unavailable" || check.Fix != "run `aigw sync`" || check.Target != filepath.Join("~", ".codex", "config.toml") {
		t.Fatalf("projection check = %#v", check)
	}

	cfg.SetClientActivation(codex.ID, true, "codex", []string{"unused"})
	delete(cfg.Accounts, "team")
	if check := findCheck(t, adapterChecks(context.Background(), deps.Dependencies, cfg, ""), "projection:codex"); check.OK || check.Detail != "selected client route cannot be resolved" {
		t.Fatalf("route check = %#v", check)
	}
	if got := adapterChecks(context.Background(), deps.Dependencies, domain.NewConfig(clients.Admission()), ""); len(got) != len(clients.Admission().IDs()) {
		t.Fatalf("disabled adapter checks = %#v", got)
	}
}

func TestClaudeExecutableReadFailuresAreDiagnostic(t *testing.T) {
	cfg := validDoctorConfig()
	blocked := filepath.Join(t.TempDir(), "claude")
	if err := os.MkdirAll(blocked, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg.SetClientActivation(claude.ID, true, blocked, nil)
	deps, _, _ := doctorDependencies(t, cfg)
	check := findCheck(t, collectChecks(context.Background(), deps.Dependencies), "adapter:claude")
	if check.OK || !strings.Contains(check.Detail, "unavailable") {
		t.Fatalf("adapter = %#v", check)
	}
}

func TestCommandHumanAndJSONPaths(t *testing.T) {
	cfg := validDoctorConfig()
	deps, out, secretStore := doctorDependencies(t, cfg)
	inspect := func(app.Dependencies, domain.Config) map[string]domainreadiness.Client {
		return map[string]domainreadiness.Client{
			claude.ID: {State: domainreadiness.Configured, Route: "claude", Account: "team"},
			codex.ID:  {State: domainreadiness.Deferred, NextAction: "aigw sync"},
		}
	}
	if err := secretStore.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	result := executeJSON(t, newCommand(deps, inspect), out)
	if !result.OK || !allOK(result.Checks) {
		t.Fatalf("result = %#v", result)
	}
	if result.Clients[claude.ID].State != domainreadiness.Configured {
		t.Fatalf("canonical clients = %#v", result.Clients)
	}

	out.Reset()
	cmd := newCommand(deps, inspect)
	if err := executeDoctorCommand(cmd); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "No client is enabled") || !strings.Contains(out.String(), "Local diagnostics passed") {
		t.Fatalf("human output = %s", out.String())
	}

	out.Reset()
	deps.Env = []string{"OPENAI_API_KEY=secret"}
	cmd = newCommand(deps, inspect)
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
	if err := executeDoctorCommand(newCommand(deps, inspect)); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 || !strings.Contains(render.String(), "No client is enabled") ||
		!strings.Contains(render.String(), "Claude") || !strings.Contains(render.String(), "Configured") ||
		!strings.Contains(render.String(), "Codex") || !strings.Contains(render.String(), "Deferred") {
		t.Fatalf("out=%q render=%q", out.String(), render.String())
	}

	cmd = newCommand(deps, inspect)
	cmd.SetArgs([]string{"unexpected"})
	if err := executeDoctorCommand(cmd); err == nil {
		t.Fatal("doctor accepted a positional argument")
	}
}

func TestCommandJSONExposesTheCanonicalCredentialBackendSelection(t *testing.T) {
	cfg := validDoctorConfig()
	deps, out, _ := doctorDependencies(t, cfg)
	root := filepath.Join(t.TempDir(), "secrets")
	store, err := secret.Select(secret.Selection{
		GOOS:         runtime.GOOS,
		Root:         root,
		KeyringProbe: func(secret.Store) error { return errors.New("service unavailable") },
	})
	if err != nil {
		t.Fatal(err)
	}
	deps.Secrets = store

	result := executeJSON(t, newCommand(deps, nil), out)
	want := secret.BackendSelection{
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
	store, err := secret.Select(secret.Selection{GOOS: runtime.GOOS, Root: root})
	if err != nil {
		t.Fatal(err)
	}
	deps.Secrets = store

	result := executeJSON(t, newCommand(deps, nil), out)
	if result.CredentialBackend.Availability != "unavailable" || result.CredentialBackend.RecoveryAction != "aigw doctor" {
		t.Fatalf("credential backend = %#v", result.CredentialBackend)
	}
	check := findCheck(t, result.Checks, "credential:backend")
	if check.OK || check.Fix != "aigw doctor" || check.Detail != "credential backend is unavailable" {
		t.Fatalf("credential backend check = %#v", check)
	}
}

func TestCommandPreservesCanonicalClientsWhenInspectionFails(t *testing.T) {
	cfg := validDoctorConfig()
	deps, out, secretStore := doctorDependencies(t, cfg)
	if err := secretStore.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	inspect := func(app.Dependencies, domain.Config) map[string]domainreadiness.Client {
		return map[string]domainreadiness.Client{
			claude.ID: {
				State:      domainreadiness.Unavailable,
				Route:      "claude",
				Account:    "team",
				Detail:     "Credential metadata is unavailable",
				NextAction: "aigw doctor",
			},
		}
	}

	result := executeJSON(t, newCommand(deps, inspect), out)
	state, ok := result.Clients[claude.ID]
	if result.OK || result.NextAction != "aigw doctor" || !ok || state.State != domainreadiness.Unavailable || state.NextAction != "aigw doctor" {
		t.Fatalf("canonical clients = %#v\n%s", result.Clients, out.String())
	}

	out.Reset()
	if err := executeDoctorCommand(newCommand(deps, inspect)); err == nil {
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
	cmd := newCommand(deps, nil)
	cmd.SetArgs([]string{"--json"})
	if err := executeDoctorCommand(cmd); !errors.Is(err, want) {
		t.Fatalf("JSON write error = %v", err)
	}
	deps.RenderOut = failingWriter{err: want}
	if err := executeDoctorCommand(newCommand(deps, nil)); !errors.Is(err, want) {
		t.Fatalf("render error = %v", err)
	}
}

func TestCommandPropagatesWriterFailureWhilePresentingProblems(t *testing.T) {
	cfg := validDoctorConfig()
	deps, _, _ := doctorDependencies(t, cfg)
	want := errors.New("write failed")
	deps.RenderOut = failingWriter{err: want}
	if err := executeDoctorCommand(newCommand(deps, nil)); !errors.Is(err, want) {
		t.Fatalf("problem render error = %v", err)
	}
	deps.Out = failingWriter{err: want}
	cmd := newCommand(deps, nil)
	cmd.SetArgs([]string{"--json"})
	if err := executeDoctorCommand(cmd); !errors.Is(err, want) {
		t.Fatalf("problem JSON write error = %v", err)
	}
}

func TestCommandConstructorExposesStableName(t *testing.T) {
	if got := NewCommand(invocation.Context{}).Name(); got != "doctor" {
		t.Fatalf("doctor command = %q", got)
	}
}
