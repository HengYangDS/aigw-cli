package doctor

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/claude"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"

	"github.com/spf13/cobra"
)

type commandResult struct {
	Checks []Check `json:"checks"`
	OK     bool    `json:"ok"`
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
	cfg.Profiles["team"] = configuration.Profile{
		Label:   "Team",
		Account: "team",
		Models: configuration.Models{
			configuration.ClientClaude: "claude-test",
			configuration.ClientCodex:  "gpt-test",
		},
	}
	cfg.Routes.Default = "team"
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
	return Dependencies{Config: store, Secrets: secretStore, Out: out}, out, secretStore
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
		"secret:team":              "System secret",
		"adapter:claude":           "Claude adapter",
		"adapter:codex":            "Codex adapter",
		"launcher:claude":          "Claude launcher",
		"path:claude":              "Claude PATH activation",
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
		{Check{Name: "secret:team", OK: true}, "team · available"},
		{Check{Name: "secret:team"}, "team · missing"},
		{Check{Name: "adapter:claude", OK: true, Detail: "enabled"}, "Enabled"},
		{Check{Name: "adapter:claude", Detail: "enabled but executable is missing"}, "Enabled, but no executable is configured"},
		{Check{Name: "adapter:codex", Detail: "enabled but no Codex config target is configured"}, "Enabled, but no Codex configuration file is configured"},
		{Check{Name: "launcher:claude", OK: true}, "AIGW-managed Claude launcher is ready"},
		{Check{Name: "launcher:claude", Detail: "AIGW managed Claude launcher is missing"}, "AIGW-managed Claude launcher is missing"},
		{Check{Name: "path:claude", OK: true}, "AIGW-managed Claude PATH activation is ready"},
		{Check{Name: "path:claude", Detail: "AIGW-managed Claude PATH activation is missing"}, "Claude PATH activation is missing"},
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
		"run `aigw setup`":                             "aigw setup",
		"run `aigw repair`":                            "aigw repair",
		"run `aigw sync` to reconcile this target":     "aigw sync",
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
	checks := Collect(deps)
	for _, name := range []string{"secret:team", "adapter:claude", "adapter:codex"} {
		check := findCheck(t, checks, name)
		if check.OK || check.Fix == "" {
			t.Fatalf("%s = %#v", name, check)
		}
	}
	if findCheck(t, checks, "adapter:claude").Detail != "enabled but executable is missing" {
		t.Fatalf("checks = %#v", checks)
	}

	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{
		Enabled:    true,
		Executable: "codex",
	}
	deps, _, _ = doctorDependencies(t, cfg)
	check := findCheck(t, Collect(deps), "adapter:codex")
	if check.OK || check.Detail != "enabled but no Codex config target is configured" || check.Fix != "run `aigw repair`" {
		t.Fatalf("Codex adapter check = %#v", check)
	}
	if got := sortedDoctorAccountNames(configuration.Config{Accounts: map[string]configuration.Account{"z": {}, "a": {}}}); strings.Join(got, ",") != "a,z" {
		t.Fatalf("sorted accounts = %v", got)
	}

	bad := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
	if err := os.MkdirAll(bad.Path(), 0o700); err != nil {
		t.Fatal(err)
	}
	broken := Collect(Dependencies{Config: bad, Secrets: secrets.NewMemoryStore()})
	if check := findCheck(t, broken, "config"); check.OK || !strings.Contains(check.Fix, bad.Path()) {
		t.Fatalf("config check = %#v", check)
	}
}

func TestCollectExercisesLauncherAndProjectionStates(t *testing.T) {
	cfg := validDoctorConfig()
	home := t.TempDir()
	bin := t.TempDir()
	manager := claude.Launcher{GOOS: "other", BinDir: bin, Home: home, Shell: "/bin/zsh", AIGWExecutable: filepath.Join(bin, "aigw")}
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: "claude"}
	deps, _, secretsStore := doctorDependencies(t, cfg)
	deps.ClaudeLauncher = manager
	if err := secretsStore.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	checks := Collect(deps)
	if check := findCheck(t, checks, "adapter:claude"); check.OK || !strings.Contains(check.Detail, "launcher is missing") {
		t.Fatalf("adapter check = %#v", check)
	}
	if check := findCheck(t, checks, "launcher:claude"); check.OK || !strings.Contains(check.Detail, "launcher is missing") {
		t.Fatalf("launcher check = %#v", check)
	}

	if err := os.WriteFile(filepath.Join(bin, "aigw"), []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte("AIGW managed Claude launcher\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".zshrc"), []byte("# >>> AIGW Claude launcher PATH >>>\n"+bin+"\n# <<< AIGW Claude launcher PATH <<<\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	checks = Collect(deps)
	if check := findCheck(t, checks, "launcher:claude"); !check.OK {
		t.Fatalf("launcher check = %#v", check)
	}
	if check := findCheck(t, checks, "path:claude"); !check.OK {
		t.Fatalf("activation check = %#v", check)
	}

	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "codex", Targets: []string{filepath.Join(t.TempDir(), "missing.toml")}}
	deps, _, _ = doctorDependencies(t, cfg)
	check := findCheck(t, Collect(deps), "codex:target-1")
	if check.OK || !strings.Contains(check.Detail, "read Codex config") || check.Fix != "run `aigw sync` to reconcile this target" {
		t.Fatalf("projection check = %#v", check)
	}

	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "codex", Targets: []string{"unused"}}
	delete(cfg.Accounts, "team")
	if checks := codexProjectionChecks(cfg); len(checks) != 1 || checks[0].Name != "projection:codex" || checks[0].OK {
		t.Fatalf("route checks = %#v", checks)
	}
	if got := codexProjectionChecks(configuration.NewConfig()); got != nil {
		t.Fatalf("disabled projection checks = %#v", got)
	}
	if got := claudeLauncherChecks(Dependencies{}, configuration.NewConfig()); got != nil {
		t.Fatalf("disabled launcher checks = %#v", got)
	}
}

func TestLauncherReadFailuresAreDiagnostic(t *testing.T) {
	cfg := validDoctorConfig()
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: "claude"}
	blocked := filepath.Join(t.TempDir(), "claude")
	if err := os.MkdirAll(blocked, 0o700); err != nil {
		t.Fatal(err)
	}
	manager := claude.Launcher{GOOS: "darwin", BinDir: filepath.Dir(blocked), Home: t.TempDir(), Shell: "/bin/zsh"}
	deps, _, _ := doctorDependencies(t, cfg)
	deps.ClaudeLauncher = manager
	checks := Collect(deps)
	for _, name := range []string{"adapter:claude", "launcher:claude"} {
		check := findCheck(t, checks, name)
		if check.OK || !strings.Contains(check.Detail, "inspect Claude launcher") {
			t.Fatalf("%s = %#v", name, check)
		}
	}

	launcherPath := filepath.Join(t.TempDir(), "claude")
	manager = claude.Launcher{GOOS: "darwin", BinDir: filepath.Dir(launcherPath), Home: filepath.Join(t.TempDir(), "missing", "home"), Shell: "/bin/zsh", AIGWExecutable: "/usr/bin/true"}
	if err := os.WriteFile(launcherPath, []byte("#!/bin/sh\n# AIGW managed Claude launcher\nexec '/usr/bin/true' __run-claude \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	checks = claudeLauncherChecks(Dependencies{ClaudeLauncher: manager}, cfg)
	if check := findCheck(t, checks, "path:claude"); check.OK || check.Detail != "AIGW-managed Claude PATH activation is missing" {
		t.Fatalf("activation = %#v", check)
	}
}

func TestCommandHumanAndJSONPaths(t *testing.T) {
	cfg := validDoctorConfig()
	deps, out, secretStore := doctorDependencies(t, cfg)
	if err := secretStore.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	result := executeJSON(t, deps)
	if !result.OK || !AllOK(result.Checks) {
		t.Fatalf("result = %#v", result)
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
	if out.Len() != 0 || !strings.Contains(render.String(), "No problems found") {
		t.Fatalf("out=%q render=%q", out.String(), render.String())
	}

	cmd = NewCommand(deps)
	cmd.SetArgs([]string{"unexpected"})
	if err := executeDoctorCommand(cmd); err == nil {
		t.Fatal("doctor accepted a positional argument")
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

func TestAllOKAndNextActionBranches(t *testing.T) {
	if !AllOK(nil) || AllOK([]Check{{Name: "bad"}}) {
		t.Fatal("AllOK result mismatch")
	}
	for _, test := range []struct {
		checks []Check
		want   string
	}{
		{[]Check{{Name: "config", Detail: "not configured"}}, "aigw setup"},
		{[]Check{{Name: "healthy", OK: true}, {Name: "codex:target-1", Fix: "run `aigw sync` to reconcile this target"}}, "aigw sync"},
		{nil, "aigw repair"},
	} {
		if got := NextAction(test.checks); got != test.want {
			t.Errorf("NextAction(%#v) = %q, want %q", test.checks, got, test.want)
		}
	}
}
