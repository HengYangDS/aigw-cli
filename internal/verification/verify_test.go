package verification

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/codex"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/process"
)

type basicRunner struct{ err error }

func (runner basicRunner) Run(context.Context, process.Plan) error { return runner.err }

type captureRunner struct {
	output []byte
	err    error
}

func (runner captureRunner) Run(context.Context, process.Plan) error { return runner.err }
func (runner captureRunner) RunCapture(context.Context, process.Plan) ([]byte, error) {
	return runner.output, runner.err
}

type recordingCaptureRunner struct {
	plans              []process.Plan
	version            string
	marker             string
	removeFinalMessage bool
	requestOutput      []byte
	requestErr         error
}

func (runner *recordingCaptureRunner) Run(context.Context, process.Plan) error {
	return runner.requestErr
}

func (runner *recordingCaptureRunner) RunCapture(_ context.Context, plan process.Plan) ([]byte, error) {
	runner.plans = append(runner.plans, plan)
	if slices.Equal(plan.Args, []string{"--version"}) {
		return []byte(runner.version + "\n"), nil
	}
	if runner.requestErr != nil {
		return append([]byte(nil), runner.requestOutput...), runner.requestErr
	}
	outputPath := argumentValue(plan.Args, "--output-last-message")
	if outputPath == "" {
		return nil, errors.New("verification output path is missing")
	}
	if runner.removeFinalMessage {
		if err := os.Remove(outputPath); err != nil {
			return nil, err
		}
		return []byte("non-authoritative diagnostic output\n"), nil
	}
	if err := os.WriteFile(outputPath, []byte(runner.marker+"\n"), 0o600); err != nil {
		return nil, err
	}
	return []byte("non-authoritative diagnostic output\n"), nil
}

func argumentValue(arguments []string, name string) string {
	for index, argument := range arguments {
		if argument == name && index+1 < len(arguments) {
			return arguments[index+1]
		}
	}
	return ""
}

func environmentValue(environment []string, name string) string {
	prefix := name + "="
	for _, value := range environment {
		if strings.HasPrefix(value, prefix) {
			return strings.TrimPrefix(value, prefix)
		}
	}
	return ""
}

func verificationConfig() configuration.Config {
	cfg := configuration.NewConfig()
	cfg.Accounts["one"] = configuration.Account{Label: "One", Endpoints: configuration.Endpoints{
		OpenAIResponses: "https://one.test/v1",
		Anthropic:       "https://one.test",
	}}
	cfg.Profiles["codex"] = configuration.Profile{Label: "Codex", Account: "one", Client: configuration.ClientCodex, Model: "gpt-test"}
	cfg.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "one", Client: configuration.ClientClaude, Model: "claude-test"}
	cfg.Routes[configuration.ClientCodex] = "codex"
	cfg.Routes[configuration.ClientClaude] = "claude"
	return cfg
}

func TestVerifyCodexUsesConfiguredClientAndOneSynchronizedTarget(t *testing.T) {
	cfg := verificationConfig()
	root := t.TempDir()
	first := filepath.Join(root, "a", "config.toml")
	second := filepath.Join(root, "z", "config.toml")
	for _, target := range []string{first, second} {
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		runtime, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
		if err != nil {
			t.Fatal(err)
		}
		if err := codex.SyncConfig(target, runtime); err != nil {
			t.Fatal(err)
		}
	}
	executable := filepath.Join(t.TempDir(), "codex")
	if goruntime.GOOS == "windows" {
		executable += ".exe"
	}
	executableBytes := []byte("codex fixture")
	if err := os.WriteFile(executable, executableBytes, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{
		Enabled:    true,
		Executable: executable,
		Targets:    []string{first, second},
	}
	runtime, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	runner := &recordingCaptureRunner{version: "codex-cli 9.9.9", marker: "AIGW_OK"}
	identity, err := VerifyCodexInvocation(context.Background(), runner, cfg, runtime)
	if err != nil {
		t.Fatal(err)
	}
	wantSHA256 := sha256.Sum256(executableBytes)
	if identity.Version != "codex-cli 9.9.9" || identity.SHA256 != fmt.Sprintf("%x", wantSHA256) {
		t.Fatalf("identity = %#v", identity)
	}
	if len(runner.plans) != 2 {
		t.Fatalf("plans = %#v", runner.plans)
	}
	if !slices.Equal(runner.plans[0].Args, []string{"--version"}) {
		t.Fatalf("identity plan = %#v", runner.plans[0])
	}
	plan := runner.plans[1]
	outputPath := argumentValue(plan.Args, "--output-last-message")
	wantArgs := []string{"exec", "--ephemeral", "--ignore-rules", "--skip-git-repo-check", "--strict-config", "--sandbox", "read-only", "--color", "never", "--cd", filepath.Dir(outputPath), "--output-last-message", outputPath, "--model", "gpt-test", "Reply with exactly: AIGW_OK"}
	if plan.Executable != executable || outputPath == "" || !slices.Equal(plan.Args, wantArgs) {
		t.Fatalf("plan = %#v", plan)
	}
	if got := environmentValue(plan.Env, "CODEX_HOME"); got != filepath.Dir(first) {
		t.Fatalf("CODEX_HOME = %q", got)
	}
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("verification output remains after success: %v", err)
	}
}

func TestVerifyCodexRejectsUnavailableCapabilityAndWrongFinalMessage(t *testing.T) {
	cfg := verificationConfig()
	target := filepath.Join(t.TempDir(), "codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := codex.SyncConfig(target, runtime); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(t.TempDir(), "codex")
	if goruntime.GOOS == "windows" {
		executable += ".exe"
	}
	if err := os.WriteFile(executable, []byte("codex fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: executable, Targets: []string{target}}

	disabled := cfg.Clone()
	disabled.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{}
	if _, err := VerifyCodexInvocation(context.Background(), &recordingCaptureRunner{}, disabled, runtime); err == nil || !strings.Contains(err.Error(), "adapter is disabled") {
		t.Fatalf("disabled adapter error = %v", err)
	}
	missingExecutable := cfg.Clone()
	missingExecutable.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Targets: []string{target}}
	if _, err := VerifyCodexInvocation(context.Background(), &recordingCaptureRunner{}, missingExecutable, runtime); err == nil || !strings.Contains(err.Error(), "executable is not configured") {
		t.Fatalf("missing executable error = %v", err)
	}
	missingTarget := cfg.Clone()
	missingTarget.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: executable}
	if _, err := VerifyCodexInvocation(context.Background(), &recordingCaptureRunner{}, missingTarget, runtime); err == nil || !strings.Contains(err.Error(), "configuration target is missing") {
		t.Fatalf("missing target error = %v", err)
	}
	missingModel := runtime
	missingModel.Model = ""
	if _, err := VerifyCodexInvocation(context.Background(), &recordingCaptureRunner{}, cfg, missingModel); err == nil || !strings.Contains(err.Error(), "has no Codex model") {
		t.Fatalf("missing model error = %v", err)
	}
	if _, err := VerifyCodexInvocation(context.Background(), basicRunner{}, cfg, runtime); err == nil || !strings.Contains(err.Error(), "capture") {
		t.Fatalf("capture error = %v", err)
	}
	missingOnDisk := cfg.Clone()
	missingOnDisk.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: filepath.Join(t.TempDir(), "missing-codex"), Targets: []string{target}}
	if _, err := VerifyCodexInvocation(context.Background(), &recordingCaptureRunner{}, missingOnDisk, runtime); err == nil || !strings.Contains(err.Error(), "read Codex executable") {
		t.Fatalf("missing executable file error = %v", err)
	}
	if _, err := VerifyCodexInvocation(context.Background(), &recordingCaptureRunner{version: "codex-cli 9.9.9", marker: "wrong"}, cfg, runtime); err == nil || !strings.Contains(err.Error(), "expected AIGW_OK") {
		t.Fatalf("marker error = %v", err)
	}
	requestFailure := &recordingCaptureRunner{
		version:       "codex-cli 9.9.9",
		requestOutput: []byte("Error loading config.toml: unknown configuration field mcp_servers.github.disabled_reason; token=must-not-leak\n"),
		requestErr:    errors.New("exit status 1"),
	}
	_, err = VerifyCodexInvocation(context.Background(), requestFailure, cfg, runtime)
	if err == nil {
		t.Fatal("failed Codex request was accepted")
	}
	for _, want := range []string{
		"Codex minimal verification request failed",
		"unknown configuration field mcp_servers.github.disabled_reason",
		"token=[REDACTED]",
		"aigw verify --for codex",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("request error lacks %q: %v", want, err)
		}
	}
	if strings.Contains(err.Error(), "must-not-leak") {
		t.Fatalf("request error exposed a credential: %v", err)
	}
	if outputPath := argumentValue(requestFailure.plans[len(requestFailure.plans)-1].Args, "--output-last-message"); outputPath == "" {
		t.Fatal("failed request plan has no output path")
	} else if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("verification output remains after failed request: %v", err)
	}
	missing := &recordingCaptureRunner{version: "codex-cli 9.9.9", removeFinalMessage: true}
	if _, err := VerifyCodexInvocation(context.Background(), missing, cfg, runtime); err == nil || !strings.Contains(err.Error(), "read Codex final response") {
		t.Fatalf("missing final message error = %v", err)
	}
	if outputPath := argumentValue(missing.plans[len(missing.plans)-1].Args, "--output-last-message"); outputPath == "" {
		t.Fatal("missing final message plan has no output path")
	} else if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("verification output remains after missing response: %v", err)
	}
	oversized := &recordingCaptureRunner{version: "codex-cli 9.9.9", marker: strings.Repeat("x", 1024)}
	if _, err := VerifyCodexInvocation(context.Background(), oversized, cfg, runtime); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized final message error = %v", err)
	}
	if outputPath := argumentValue(oversized.plans[len(oversized.plans)-1].Args, "--output-last-message"); outputPath == "" {
		t.Fatal("oversized final message plan has no output path")
	} else if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("verification output remains after oversized response: %v", err)
	}
	drifted := cfg.Clone()
	drifted.Profiles["codex"] = configuration.Profile{Label: "Codex", Account: "one", Client: configuration.ClientCodex, Model: "other"}
	if _, err := VerifyCodexInvocation(context.Background(), &recordingCaptureRunner{}, drifted, configuration.Runtime{ProfileID: "codex", Model: "other"}); err == nil || !strings.Contains(err.Error(), "synchronized") {
		t.Fatalf("projection error = %v", err)
	}
}

func TestVerifyClaude(t *testing.T) {
	cfg := verificationConfig()
	runtime, err := cfg.ResolveRuntime(configuration.ClientClaude, "")
	if err != nil {
		t.Fatal(err)
	}
	want := errors.New("launcher failed")
	if err := VerifyClaudeRuntime(context.Background(), nil, "claude", configuration.Runtime{ProfileID: "one"}, "token"); err == nil || !strings.Contains(err.Error(), "no Claude model") {
		t.Fatalf("model error = %v", err)
	}
	if err := VerifyClaudeRuntime(context.Background(), nil, "", runtime, "token"); err == nil || !strings.Contains(err.Error(), "executable is not configured") {
		t.Fatalf("plan error = %v", err)
	}
	if err := VerifyClaudeRuntime(context.Background(), basicRunner{}, "claude", runtime, "token"); err == nil || !strings.Contains(err.Error(), "runner is unavailable") {
		t.Fatalf("runner error = %v", err)
	}
	if err := VerifyClaudeRuntime(context.Background(), captureRunner{err: want}, "claude", runtime, "token"); !errors.Is(err, want) {
		t.Fatalf("capture error = %v", err)
	}
	if err := VerifyClaudeRuntime(context.Background(), captureRunner{output: []byte("wrong")}, "claude", runtime, "token"); err == nil || !strings.Contains(err.Error(), "expected AIGW_OK") {
		t.Fatalf("sentinel error = %v", err)
	}
	executable := filepath.Join(t.TempDir(), "claude")
	if goruntime.GOOS == "windows" {
		executable += ".exe"
	}
	if err := os.WriteFile(executable, []byte("fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := VerifyClaudeRuntime(context.Background(), captureRunner{output: []byte(" AIGW_OK \n")}, executable, runtime, "token"); err != nil {
		t.Fatal(err)
	}
}
