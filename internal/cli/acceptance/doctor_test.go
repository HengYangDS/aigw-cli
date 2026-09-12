package cli_test

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/cli"
	"aigw-cli/internal/codex"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
)

func TestDoctorReportsOneCredentialCheckForSharedAccount(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["team"] = configuration.Account{Label: "Team", Endpoints: configuration.Endpoints{OpenAIResponses: "https://team.test/v1", Anthropic: "https://team.test"}}
	cfg.Profiles["codex"] = configuration.Profile{Label: "Codex", Account: "team", Client: configuration.ClientCodex, Model: "codex-test"}
	cfg.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "team", Client: configuration.ClientClaude, Model: "claude-test"}
	cfg.Routes[configuration.ClientCodex] = "codex"
	cfg.Routes[configuration.ClientClaude] = "claude"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"doctor", "--json"}); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Checks []struct {
			Name string `json:"name"`
			OK   bool   `json:"ok"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	credentials := 0
	for _, check := range result.Checks {
		if strings.HasPrefix(check.Name, "secret:") {
			credentials++
			if check.Name != "secret:team" || !check.OK {
				t.Errorf("credential check = %+v", check)
			}
		}
	}
	if credentials != 1 {
		t.Fatalf("credential checks = %d, want one shared Account", credentials)
	}
}

func TestDoctorReportsCredentialObservationFailure(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-test")
	want := errors.New("credential observation failed")
	app.Secrets = &recordingCredentialStore[string]{backend: secrets.NewMemoryStore(), existsErr: want}

	if err := cli.Execute(app, []string{"doctor", "--json"}); err == nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{`"name": "secret:one"`, `"ok": false`, "credential backend failed", want.Error()} {
		if !strings.Contains(out.String(), fragment) {
			t.Fatalf("doctor output lacks %q: %s", fragment, out.String())
		}
	}
}

func TestDoctorDetectsCodexProjectionDrift(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	target := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\nmodel = \"gpt-original\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := configuration.Profile{Label: "GPT 5.6 Sol Codex", Account: "dmx", Client: configuration.ClientCodex, Model: "gpt-5.6-sol"}
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMX", Endpoints: configuration.Endpoints{OpenAIResponses: "https://example.test/v1"}}
	cfg.Profiles["gpt-5.6-sol"] = profile
	cfg.Routes[configuration.ClientCodex] = "gpt-5.6-sol"
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{target}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "test-token"); err != nil {
		t.Fatal(err)
	}
	runtime, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	runtime.CredentialCommand = app.Executable
	if err := codex.SyncConfig(target, runtime); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	drifted := strings.Replace(string(data), `model = "gpt-5.6-sol" # managed by AIGW`, `model = "gpt-5.6-terra" # managed by AIGW`, 1)
	if err := os.WriteFile(target, []byte(drifted), 0o600); err != nil {
		t.Fatal(err)
	}

	err = cli.Execute(app, []string{"doctor", "--json"})
	if err == nil {
		t.Fatalf("doctor --json error = %v", err)
	}
	if !strings.Contains(out.String(), `"codex:target-1"`) || !strings.Contains(out.String(), "model selection") || !strings.Contains(out.String(), `"ok": false`) {
		t.Fatalf("doctor output = %s", out.String())
	}
}

func TestDoctorReportsGlobalClientTokenEnvironmentWithoutLeakingValue(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	const secret = "doctor-environment-token-must-not-appear"
	app.Env = []string{"ANTHROPIC_AUTH_TOKEN=" + secret}

	if err := cli.Execute(app, []string{"doctor", "--json"}); err == nil {
		t.Fatalf("doctor --json error = %v", err)
	}
	result := out.String()
	if !strings.Contains(result, `"name": "environment:client-token"`) ||
		!strings.Contains(result, "ANTHROPIC_AUTH_TOKEN") ||
		!strings.Contains(result, `"ok": false`) {
		t.Fatalf("doctor output = %s", result)
	}
	if strings.Contains(result, secret) {
		t.Fatalf("doctor leaked environment token: %s", result)
	}
}

func TestDoctorPreservesUnrelatedEnvironment(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-test")
	if err := secretStore.Set("one", "token"); err != nil {
		t.Fatal(err)
	}
	app.Env = []string{"EDITOR=vim", "TERM=xterm-256color"}

	if err := cli.Execute(app, []string{"doctor", "--json"}); err != nil {
		t.Fatalf("doctor --json error = %v", err)
	}
	result := out.String()
	if !strings.Contains(result, `"name": "environment:client-token"`) || !strings.Contains(result, `"ok": true`) {
		t.Fatalf("doctor output = %s", result)
	}
	if strings.Join(app.Env, "\n") != "EDITOR=vim\nTERM=xterm-256color" {
		t.Fatalf("doctor changed unrelated environment: %q", app.Env)
	}
}

func TestDoctorHumanOutputUsesConciseCheckLabels(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	app.Env = []string{"ANTHROPIC_AUTH_TOKEN=test-token"}
	if err := cli.Execute(app, []string{"doctor"}); err == nil {
		t.Fatal("doctor succeeded despite a global client token")
	}
	result := out.String()
	if !strings.Contains(result, "Client token environment") || strings.Contains(result, "environment:client-token") {
		t.Fatalf("doctor human label = %s", result)
	}
}

func TestDoctorHumanOutputTranslatesSuccessfulImplementationDetails(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "team", "team", "Team", configuration.Endpoints{Anthropic: "https://team.test"}, configuration.ClientClaude, "claude-test")
	cfg.Routes[configuration.ClientClaude] = "team"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "claude")}
	synchronizeClaudeProjection(t, app, cfg)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"doctor"}); err != nil {
		t.Fatal(err)
	}
	result := out.String()
	for _, want := range []string{"No global client token environment variables detected", "Configuration is valid", "System secret", "team · available", "Claude adapter", "Enabled"} {
		if !strings.Contains(result, want) {
			t.Fatalf("doctor human output missing %q:\n%s", want, result)
		}
	}
	for _, unwanted := range []string{"no global client token environment variables", "config            valid", "AIGW managed launcher", "PATH activation", "path:claude", "launcher:claude"} {
		if strings.Contains(result, unwanted) {
			t.Fatalf("doctor human output leaked implementation prose %q:\n%s", unwanted, result)
		}
	}
}

func TestDoctorHumanOutputTranslatesCodexProjectionFailureButJSONStaysDiagnostic(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	target := filepath.Join(t.TempDir(), "configuration.toml")
	profile := configuration.Profile{Label: "GPT", Account: "team", Client: configuration.ClientCodex, Model: "gpt-test"}
	cfg := configuration.NewConfig()
	cfg.Accounts["team"] = configuration.Account{Label: "Team", Endpoints: configuration.Endpoints{OpenAIResponses: "https://team.test/v1"}}
	cfg.Profiles["gpt"] = profile
	cfg.Routes[configuration.ClientCodex] = "gpt"
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{target}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\nmodel = \"gpt-test\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"doctor"}); err == nil {
		t.Fatal("doctor should report Codex projection drift")
	}
	human := out.String()
	if !strings.Contains(human, "Codex configuration target 1") || !strings.Contains(human, "Does not match the current route") || strings.Contains(human, "Codex config AIGW state is missing") {
		t.Fatalf("doctor human output = %s", human)
	}
	if !strings.Contains(human, "Next\n  aigw sync") || strings.Contains(human, "Next\n  aigw repair") {
		t.Fatalf("doctor drift next action = %s", human)
	}
	out.Reset()
	if err := cli.Execute(app, []string{"doctor", "--json"}); err == nil {
		t.Fatalf("doctor --json error = %v", err)
	}
	if !strings.Contains(out.String(), `"codex:target-1"`) || !strings.Contains(out.String(), "Codex config AIGW state is missing") {
		t.Fatalf("doctor JSON diagnostic changed = %s", out.String())
	}
}

func TestDoctorHumanOutputNeverExposesRawEnvironmentFixText(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	app.Env = []string{"ANTHROPIC_AUTH_TOKEN=test-token"}
	if err := cli.Execute(app, []string{"doctor"}); err == nil {
		t.Fatal("doctor succeeded despite a global client token")
	}
	result := out.String()
	if !strings.Contains(result, "Remove the variables above from the parent environment that launched this terminal") {
		t.Fatalf("doctor environment fix = %s", result)
	}
	if strings.Contains(result, "remove them from the parent environment") {
		t.Fatalf("doctor leaked raw environment fix = %s", result)
	}
}

func TestDoctorHumanOutputTranslatesUnreadableConfigWithoutLeakingPath(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	if err := os.MkdirAll(app.Config.Path(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"doctor"}); err == nil {
		t.Fatal("doctor should fail when config path is a directory")
	}
	result := out.String()
	if !strings.Contains(result, "Cannot read or validate configuration") || !strings.Contains(result, "Inspect or restore the local configuration file") {
		t.Fatalf("doctor config failure output = %s", result)
	}
	if strings.Contains(result, app.Config.Path()) || strings.Contains(result, "is a directory") {
		t.Fatalf("doctor leaked raw config error details = %s", result)
	}
}

func TestDoctorJSONKeepsMachineDiagnosticValues(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "team", "team", "Team", configuration.Endpoints{Anthropic: "https://team.test"}, configuration.ClientClaude, "claude-test")
	cfg.Routes[configuration.ClientClaude] = "team"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"doctor", "--json"}); err != nil {
		t.Fatalf("doctor --json error = %v", err)
	}
	for _, want := range []string{`"name": "config"`, `"detail": "valid"`, `"name": "secret:team"`, `"detail": "available"`} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("doctor JSON lost machine value %q:\n%s", want, out.String())
		}
	}
}

func TestDoctorUnconfiguredPointsToSetupNotRepair(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	if err := cli.Execute(app, []string{"doctor"}); err == nil {
		t.Fatal("doctor succeeded without configuration")
	}
	result := out.String()
	if !strings.Contains(result, "aigw setup") {
		t.Fatalf("doctor should point to setup:\n%s", result)
	}
	if strings.Contains(result, "aigw repair") {
		t.Fatalf("doctor should not suggest repair before setup:\n%s", result)
	}
}

func TestDoctorFormatsPreserveTheDiagnosticOutcome(t *testing.T) {
	for _, test := range []struct {
		name                 string
		configured, jsonMode bool
		nextAction           string
		continuations        int
	}{
		{name: "unconfigured text", nextAction: "aigw setup", continuations: 1},
		{name: "unconfigured JSON", jsonMode: true, nextAction: "aigw setup"},
		{name: "configured text", configured: true},
		{name: "configured JSON", configured: true, jsonMode: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			app, out, secretStore, _, _ := testApp(t, "")
			if test.configured {
				saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-test")
				if err := secretStore.Set("one", "token"); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{"doctor"}
			if test.jsonMode {
				args = append(args, "--json")
			}
			err := cli.Execute(app, args)
			if (err == nil) != test.configured {
				t.Fatalf("doctor error=%v, want success=%t\n%s", err, test.configured, out)
			}
			if !test.jsonMode {
				if strings.Count(out.String(), "Next\n") != test.continuations {
					t.Fatalf("doctor must present its continuation once:\n%s", out)
				}
				return
			}
			var result struct {
				OK         bool   `json:"ok"`
				NextAction string `json:"next_action"`
			}
			decoder := json.NewDecoder(out)
			if err := decoder.Decode(&result); err != nil {
				t.Fatal(err)
			}
			if result.OK != test.configured || result.NextAction != test.nextAction {
				t.Fatalf("doctor JSON=%#v", result)
			}
			if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
				t.Fatalf("doctor emitted content after its JSON report: %v", err)
			}
		})
	}
}
