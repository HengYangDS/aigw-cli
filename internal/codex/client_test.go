package codex

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/process"
)

type identityCaptureRunner struct {
	output []byte
	err    error
	plan   process.Plan
}

func (runner *identityCaptureRunner) RunCapture(_ context.Context, plan process.Plan) ([]byte, error) {
	runner.plan = plan
	return runner.output, runner.err
}

func TestIdentifyExecutableRejectsIncompleteOrUnobservableIdentity(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing-codex")
	for _, test := range []struct {
		name       string
		executable string
		runner     process.CaptureRunner
		want       string
	}{
		{name: "missing executable", runner: &identityCaptureRunner{}, want: "not configured"},
		{name: "missing runner", executable: missing, want: "runner is unavailable"},
		{name: "unreadable executable", executable: missing, runner: &identityCaptureRunner{}, want: "read Codex executable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := IdentifyExecutable(context.Background(), test.runner, test.executable, t.TempDir())
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("IdentifyExecutable() error = %v, want %q", err, test.want)
			}
		})
	}

	executable := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	failed := &identityCaptureRunner{err: errors.New("version failed")}
	if _, err := IdentifyExecutable(context.Background(), failed, executable, t.TempDir()); err == nil || !strings.Contains(err.Error(), "inspect Codex version") {
		t.Fatalf("version command error = %v", err)
	}
	empty := &identityCaptureRunner{output: []byte(" \n")}
	if _, err := IdentifyExecutable(context.Background(), empty, executable, t.TempDir()); err == nil || !strings.Contains(err.Error(), "reported no version") {
		t.Fatalf("empty version error = %v", err)
	}
}

func TestIdentifyExecutableUsesTheConfiguredHome(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	runner := &identityCaptureRunner{output: []byte(" codex-cli 1.2.3 \n")}
	identity, err := IdentifyExecutable(context.Background(), runner, executable, home)
	if err != nil {
		t.Fatal(err)
	}
	if identity.Version != "codex-cli 1.2.3" || identity.SHA256 == "" {
		t.Fatalf("identity = %#v", identity)
	}
	if runner.plan.Executable != executable || !slices.Equal(runner.plan.Args, []string{"--version"}) {
		t.Fatalf("version plan = %#v", runner.plan)
	}
	found := false
	for _, value := range runner.plan.Env {
		if value == "CODEX_HOME="+home {
			found = true
		}
	}
	if !found {
		t.Fatalf("version environment does not contain CODEX_HOME=%s", home)
	}
}

func TestFileSHA256RejectsADirectory(t *testing.T) {
	if _, err := fileSHA256(t.TempDir()); err == nil {
		t.Fatal("fileSHA256() accepted a directory")
	}
}

func TestVerificationPlanRejectsIncompleteInputs(t *testing.T) {
	runtime := configuration.Runtime{ProfileID: "codex", Model: "gpt-test"}
	for _, test := range []struct {
		name       string
		executable string
		configPath string
		outputPath string
		runtime    configuration.Runtime
		want       string
	}{
		{name: "missing executable", configPath: "config.toml", outputPath: "output.txt", runtime: runtime, want: "not configured"},
		{name: "missing config", executable: "codex", outputPath: "output.txt", runtime: runtime, want: "target is not configured"},
		{name: "missing model", executable: "codex", configPath: "config.toml", outputPath: "output.txt", runtime: configuration.Runtime{ProfileID: "codex"}, want: "has no Codex model"},
		{name: "missing output", executable: "codex", configPath: "config.toml", runtime: runtime, want: "output path is not configured"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := VerificationPlan(test.executable, test.configPath, test.outputPath, test.runtime)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("VerificationPlan() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestLoginPlanRejectsIncompleteInputsAndSetsHome(t *testing.T) {
	_, err := LoginPlan("", "home", "tok")
	if err == nil || err.Error() != "Codex executable is not configured" {
		t.Errorf("expected executable error, got %v", err)
	}
	_, err = LoginPlan("bin", "home", "")
	if err == nil || err.Error() != "Codex token is empty" {
		t.Errorf("expected token error, got %v", err)
	}
	plan, err := LoginPlan("bin", "home", "tok")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range plan.Env {
		if e == "CODEX_HOME=home" {
			found = true
			break
		}
	}
	if !found {
		t.Error("CODEX_HOME not found in env")
	}
}
