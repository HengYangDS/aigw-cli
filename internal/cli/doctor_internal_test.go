package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

func TestDoctorHumanProjectionFailsClosedForFutureChecks(t *testing.T) {
	for _, check := range []doctorCheck{
		{Name: "future:internal", OK: true, Detail: "internal success detail"},
		{Name: "future:internal", OK: false, Detail: "internal failure detail", Fix: "internal repair instruction"},
	} {
		if got := doctorCheckLabel(check.Name); got != "Other check" {
			t.Fatalf("label = %q, want Other check", got)
		}
		want := "Check failed"
		if check.OK {
			want = "Healthy"
		}
		if got := doctorCheckDetail(check); got != want {
			t.Fatalf("detail = %q, want %q", got, want)
		}
		if !check.OK {
			if got := doctorCheckFix(check); got != "aigw doctor --json" {
				t.Fatalf("fix = %q, want aigw doctor --json", got)
			}
		}
	}
}

func TestDoctorMixedFailuresFallbackToRepair(t *testing.T) {
	checks := []doctorCheck{
		{Name: "codex:target-1", OK: false, Fix: "run `aigw sync` to reconcile this target"},
		{Name: "shim:claude", OK: false, Fix: "run `aigw repair`"},
	}
	if got := doctorNextAction(checks); got != "aigw repair" {
		t.Fatalf("next action = %q, want aigw repair", got)
	}
}

func TestDoctorUnclassifiedFailureFallsBackToRepair(t *testing.T) {
	checks := []doctorCheck{{Name: "config", OK: false, Detail: "unexpected"}}
	if got := doctorNextAction(checks); got != "aigw repair" {
		t.Fatalf("next action = %q, want aigw repair", got)
	}
}

func TestDoctorFormattingCoverageBranches(t *testing.T) {
	if got := doctorCheckLabel("projection:codex"); got != "Codex route" {
		t.Fatalf("projection label = %q", got)
	}
	details := []struct {
		check doctorCheck
		want  string
	}{
		{check: doctorCheck{Name: "environment:client-token"}, want: "Global client token environment variables detected"},
		{check: doctorCheck{Name: "secret:team"}, want: "team · missing"},
		{check: doctorCheck{Name: "adapter:claude", Detail: "enabled but executable is missing"}, want: "Enabled, but no executable is configured"},
		{check: doctorCheck{Name: "adapter:codex", Detail: "enabled but no Codex config target is configured"}, want: "Enabled, but no Codex configuration file is configured"},
		{check: doctorCheck{Name: "shim:claude", Detail: "AIGW managed Claude shim is missing"}, want: "AIGW-managed Claude launcher is missing"},
		{check: doctorCheck{Name: "projection:codex", Detail: "unavailable"}, want: "Current Codex route cannot be resolved"},
		{check: doctorCheck{Name: "codex:target-1", OK: true}, want: "Matches the current route"},
	}
	for _, test := range details {
		if got := doctorCheckDetail(test.check); got != test.want {
			t.Errorf("doctorCheckDetail(%+v) = %q, want %q", test.check, got, test.want)
		}
	}
	if got := doctorCheckFix(doctorCheck{Fix: "run `aigw rotate team`"}); got != "aigw rotate team" {
		t.Fatalf("generic run fix = %q", got)
	}
	if got := strings.Join(forbiddenClientTokenEnvironment([]string{"malformed", "OPENAI_API_KEY=one", "OPENAI_API_KEY=two"}), ","); got != "OPENAI_API_KEY" {
		t.Fatalf("forbidden environment names = %q", got)
	}
}

func TestDoctorCommandCoverageFailures(t *testing.T) {
	t.Run("missing secrets and executables", func(t *testing.T) {
		cfg := dailyCoverageConfig()
		cfg.Adapters[domain.ClientClaude] = domain.AdapterConfig{Enabled: true}
		cfg.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true}
		app := dailyCoverageApp(t, cfg)
		cmd := newDoctorCommand(app)
		cmd.SetArgs([]string{"--json"})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		output := app.Out.(*bytes.Buffer).String()
		for _, want := range []string{`"name": "secret:one"`, `"detail": "missing"`, `"detail": "enabled but executable is missing"`} {
			if !strings.Contains(output, want) {
				t.Fatalf("doctor output missing %q: %s", want, output)
			}
		}
	})

	t.Run("Codex target and Claude shim read", func(t *testing.T) {
		cfg := dailyCoverageConfig()
		cfg.Adapters[domain.ClientClaude] = domain.AdapterConfig{Enabled: true, Executable: "/claude"}
		cfg.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Executable: "/codex"}
		app := dailyCoverageApp(t, cfg)
		app.Shims.GOOS = "linux"
		app.Shims.BinDir = t.TempDir()
		if err := os.Mkdir(filepath.Join(app.Shims.BinDir, "claude"), 0o700); err != nil {
			t.Fatal(err)
		}
		cmd := newDoctorCommand(app)
		cmd.SetArgs([]string{"--json"})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		output := app.Out.(*bytes.Buffer).String()
		for _, want := range []string{"enabled but no Codex config target", "is a directory"} {
			if !strings.Contains(output, want) {
				t.Fatalf("doctor output missing %q: %s", want, output)
			}
		}
	})

	t.Run("missing Claude shim", func(t *testing.T) {
		cfg := dailyCoverageConfig()
		cfg.Adapters[domain.ClientClaude] = domain.AdapterConfig{Enabled: true, Executable: "/claude"}
		app := dailyCoverageApp(t, cfg)
		app.Shims.GOOS = "linux"
		app.Shims.BinDir = t.TempDir()
		cmd := newDoctorCommand(app)
		cmd.SetArgs([]string{"--json"})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		if output := app.Out.(*bytes.Buffer).String(); !strings.Contains(output, "AIGW managed Claude shim is missing") {
			t.Fatalf("doctor output = %s", output)
		}
	})

	t.Run("Claude activation read", func(t *testing.T) {
		cfg := dailyCoverageConfig()
		cfg.Adapters[domain.ClientClaude] = domain.AdapterConfig{Enabled: true, Executable: "/claude"}
		app := dailyCoverageApp(t, cfg)
		app.Shims = readyDailyShim(t)
		activation := filepath.Join(app.Shims.Home, ".zshrc")
		if err := os.Remove(activation); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(activation, 0o700); err != nil {
			t.Fatal(err)
		}
		cmd := newDoctorCommand(app)
		cmd.SetArgs([]string{"--json"})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		output := app.Out.(*bytes.Buffer).String()
		if !strings.Contains(output, `"name": "path:claude"`) || !strings.Contains(output, "is a directory") {
			t.Fatalf("doctor output = %s", output)
		}
	})

	t.Run("Codex route resolution", func(t *testing.T) {
		cfg := dailyCoverageConfig()
		account := cfg.Accounts["one"]
		account.Endpoints.OpenAIResponses = ""
		cfg.Accounts["one"] = account
		cfg.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Executable: "/codex", Targets: []string{filepath.Join(t.TempDir(), "config.toml")}}
		app := dailyCoverageApp(t, cfg)
		cmd := newDoctorCommand(app)
		cmd.SetArgs([]string{"--json"})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		output := app.Out.(*bytes.Buffer).String()
		if !strings.Contains(output, `"name": "projection:codex"`) || !strings.Contains(output, "no OpenAI Responses endpoint") {
			t.Fatalf("doctor output = %s", output)
		}
	})
}
