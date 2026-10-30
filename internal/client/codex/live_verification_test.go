package codex

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform/process"
)

type recordingCaptureRunner struct {
	plans              []process.Plan
	version            string
	marker             string
	textOnly           bool
	challenge          string
	removeFinalMessage bool
	requestOutput      []byte
	stderr             []byte
	requestErr         error
	prepareOutput      func(string) error
}

func TestVerifyCodexRejectsTextWithoutToolContinuation(t *testing.T) {
	cfg, selected := configuredCodexVerification(t, filepath.Join(t.TempDir(), "config.toml"))
	runner := &recordingCaptureRunner{version: "codex-cli 9.9.9", marker: "AIGW_OK", textOnly: true}
	if _, err := VerifyInvocation(t.Context(), runner, cfg, selected, ""); err == nil || !strings.Contains(err.Error(), "native reply preceded its challenge tool read") {
		t.Fatalf("correct text without native tool evidence was not rejected by the evidence owner: %v", err)
	}
}

func TestCodexNativeEvidenceRejectsEventsOutsideTheOwnedTurn(t *testing.T) {
	const (
		session   = "{\"type\":\"thread.started\",\"thread_id\":\"00000000-0000-4000-8000-000000000001\"}"
		tool      = "{\"type\":\"item.completed\",\"item\":{\"type\":\"command_execution\",\"status\":\"completed\",\"command\":\"cat challenge.txt\",\"exit_code\":0,\"aggregated_output\":\"challenge\"}}"
		message   = "{\"type\":\"item.completed\",\"item\":{\"type\":\"agent_message\",\"text\":\"challenge\"}}"
		completed = "{\"type\":\"turn.completed\"}"
		reasoning = "{\"type\":\"item.completed\",\"item\":{\"type\":\"reasoning\"}}"
		previous  = "00000000-0000-4000-8000-000000000001"
		private   = "/private/operator token=must-not-leak"
	)
	for name, test := range map[string]struct {
		events   []string
		previous string
		rejected bool
	}{
		"items before session":               {events: []string{tool, message, session, completed}, rejected: true},
		"completion before items":            {events: []string{session, completed, tool, message}, rejected: true},
		"items after completion":             {events: []string{session, tool, message, completed, message}, rejected: true},
		"reply before tool read":             {events: []string{session, message, tool, completed}, rejected: true},
		"reasoning preserves tool order":     {events: []string{session, reasoning, tool, reasoning, message, completed}},
		"same session recalls without tools": {events: []string{session, reasoning, message, completed}, previous: previous},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := turnEvidence([]byte(strings.Join(test.events, "\n")), test.previous, "challenge"); (err != nil) != test.rejected {
				t.Fatalf("native turn rejection=%t, want %t: %v", err != nil, test.rejected, err)
			}
		})
	}
	for name, diagnostic := range map[string]struct{ message, want string }{
		"metadata":       {"Model metadata for grok-4.7 not found. Defaulting to fallback metadata; " + private, "model-metadata warning"},
		"native failure": {"Native capability failed " + private, "native-client warning or error"},
	} {
		for phase, prior := range map[string]string{"initial": "", "resume": previous} {
			t.Run(name+"/"+phase, func(t *testing.T) {
				item := fmt.Sprintf("{\"type\":\"item.completed\",\"item\":{\"id\":\"item_0\",\"type\":\"error\",\"message\":%q}}", diagnostic.message)
				_, err := turnEvidence([]byte(strings.Join([]string{session, item, tool, message, completed}, "\n")), prior, "challenge")
				if err == nil || !strings.Contains(err.Error(), diagnostic.want) || strings.Contains(err.Error(), private) {
					t.Fatalf("native error item classification is missing or unsafe: %v", err)
				}
			})
		}
	}
}

func (runner *recordingCaptureRunner) RunCaptureStreams(ctx context.Context, plan process.Plan) ([]byte, []byte, error) {
	output, err := runner.RunCapture(ctx, plan)
	if slices.Equal(plan.Args, []string{"--version"}) {
		return output, nil, err
	}
	if err != nil {
		return output, runner.stderr, err
	}
	return output, runner.stderr, err
}

func TestSuccessfulClientDiagnosticsDoNotQualifyVerification(t *testing.T) {
	for _, input := range []struct {
		diagnostic, marker string
		missing            bool
	}{
		{"warning: Model metadata for grok-4.7 not found. Defaulting to fallback metadata. /private/operator token=must-not-leak\n", "AIGW_OK", false},
		{"2026-10-03T05:00:00Z WARN client_core: degraded native capability\n", "AIGW_OK", false},
		{"/private/operator/client.py:12: DeprecationWarning: unsupported behavior\n", "AIGW_OK", false},
		{"2026-10-03T05:00:00Z ERROR client_core: capability unavailable\n", "AIGW_OK", false},
		{"Traceback (most recent call last):\n/private/operator/client.py:12\n", "AIGW_OK", false},
		{"fatal: selected native client could not initialize\n", "AIGW_OK", false},
		{"warning: Model metadata unavailable; defaulting to fallback metadata\n", "wrong", false},
		{"warning: Model metadata unavailable; defaulting to fallback metadata\n", "wrong", true},
	} {
		t.Run(fmt.Sprintf("%s/missing=%t", input.diagnostic, input.missing), func(t *testing.T) {
			cfg, selected := configuredCodexVerification(t, filepath.Join(t.TempDir(), "config.toml"))
			runner := &recordingCaptureRunner{version: "codex-cli 9.9.9", marker: input.marker, removeFinalMessage: input.missing, stderr: []byte(input.diagnostic)}
			_, err := VerifyInvocation(t.Context(), runner, cfg, selected, "")
			if err == nil || !strings.Contains(err.Error(), "warning") || strings.Contains(err.Error(), "inference completed") {
				t.Fatalf("native diagnostics claimed qualified inference: %v", err)
			}
			for _, forbidden := range []string{"/private/operator", "must-not-leak", "client_core"} {
				if strings.Contains(err.Error(), forbidden) {
					t.Fatalf("verification exposed private diagnostic %q", forbidden)
				}
			}
		})
	}
	t.Run("ordinary stderr is not a warning", func(t *testing.T) {
		cfg, selected := configuredCodexVerification(t, filepath.Join(t.TempDir(), "config.toml"))
		runner := &recordingCaptureRunner{version: "codex-cli 9.9.9", marker: "AIGW_OK", stderr: []byte("OpenAI Codex\n2026-10-03T05:00:00Z INFO client initialized\n")}
		if _, err := VerifyInvocation(t.Context(), runner, cfg, selected, ""); err != nil {
			t.Fatal(err)
		}
	})
}

func (runner *recordingCaptureRunner) RunCapture(_ context.Context, plan process.Plan) ([]byte, error) {
	runner.plans = append(runner.plans, plan)
	if slices.Equal(plan.Args, []string{"--version"}) {
		return []byte(runner.version + "\n"), nil
	}
	outputPath := finalMessagePath(plan.Args)
	if outputPath == "" {
		return nil, errors.New("verification output path is missing")
	}
	if runner.prepareOutput != nil {
		if err := runner.prepareOutput(outputPath); err != nil {
			return nil, err
		}
	}
	if runner.requestErr != nil {
		return append([]byte(nil), runner.requestOutput...), runner.requestErr
	}
	if runner.removeFinalMessage {
		if err := os.Remove(outputPath); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		return []byte("non-authoritative diagnostic output\n"), nil
	}
	marker := runner.marker
	if !slices.Contains(plan.Args, "resume") {
		challenge, err := os.ReadFile(filepath.Join(filepath.Dir(outputPath), "challenge.txt"))
		if err != nil {
			return nil, err
		}
		runner.challenge = strings.TrimSpace(string(challenge))
	}
	if marker == "AIGW_OK" {
		marker = runner.challenge
	}
	if err := os.WriteFile(outputPath, []byte(marker+"\n"), 0o600); err != nil {
		return nil, err
	}
	markerJSON, err := json.Marshal(marker)
	if err != nil {
		return nil, err
	}
	output := []byte("{\"type\":\"thread.started\",\"thread_id\":\"00000000-0000-4000-8000-000000000001\"}\n")
	if !runner.textOnly && !slices.Contains(plan.Args, "resume") {
		output = fmt.Appendf(output, "{\"type\":\"item.completed\",\"item\":{\"type\":\"command_execution\",\"status\":\"completed\",\"command\":\"cat challenge.txt\",\"exit_code\":0,\"aggregated_output\":%q}}\n", runner.challenge)
	}
	return fmt.Appendf(output, "{\"type\":\"item.completed\",\"item\":{\"type\":\"agent_message\",\"text\":%s}}\n{\"type\":\"turn.completed\"}\n", markerJSON), nil
}

func finalMessagePath(arguments []string) string {
	for index, argument := range arguments {
		if argument == "--output-last-message" && index+1 < len(arguments) {
			return arguments[index+1]
		}
	}
	return ""
}

func configuredCodexVerification(t *testing.T, targets ...string) (domain.Config, domain.Runtime) {
	t.Helper()
	cfg := verificationConfig()
	runtime, err := cfg.ResolveRuntime(ID, "")
	if err != nil {
		t.Fatal(err)
	}
	runtime.CredentialCommand = filepath.Join(t.TempDir(), "aigw")
	for _, target := range targets {
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := SyncConfig(target, runtime); err != nil {
			t.Fatal(err)
		}
	}
	executable := filepath.Join(t.TempDir(), "codex")
	if goruntime.GOOS == "windows" {
		executable += ".exe"
	}
	if err := os.WriteFile(executable, []byte("codex fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg.SetClientActivation(ID, true, executable, targets)
	return cfg, runtime
}

func TestVerifyCodexUsesConfiguredClientAndOneSynchronizedTarget(t *testing.T) {
	root := t.TempDir()
	first, second := filepath.Join(root, "a", "config.toml"), filepath.Join(root, "z", "config.toml")
	cfg, runtime := configuredCodexVerification(t, first, second)
	executable := cfg.Clients[ID].Executable
	runner := &recordingCaptureRunner{version: "codex-cli 9.9.9", marker: "AIGW_OK"}
	identity, err := VerifyInvocation(context.Background(), runner, cfg, runtime, "")
	if err != nil {
		t.Fatal(err)
	}
	wantSHA256 := sha256.Sum256([]byte("codex fixture"))
	if identity.Version != "codex-cli 9.9.9" || identity.SHA256 != fmt.Sprintf("%x", wantSHA256) {
		t.Fatalf("identity = %#v", identity)
	}
	if len(runner.plans) != 3 || !slices.Equal(runner.plans[0].Args, []string{"--version"}) {
		t.Fatalf("identity/tool/recall plans = %#v", runner.plans)
	}
	plan := runner.plans[1]
	outputPath := finalMessagePath(plan.Args)
	if plan.Executable != executable || outputPath == "" || slices.Contains(plan.Args, "--ephemeral") || !slices.Contains(plan.Args, "--json") {
		t.Fatalf("plan = %#v", plan)
	}
	home := environmentValue(plan.Env, "CODEX_HOME")
	if home == filepath.Dir(first) || home != filepath.Join(filepath.Dir(outputPath), "home") {
		t.Fatalf("CODEX_HOME = %q", home)
	}
	for _, invocation := range runner.plans {
		if temporary := environmentValue(invocation.Env, "TMPDIR"); temporary != filepath.Join(home, "tmp") {
			t.Fatalf("native temporary root %q contains or escapes the private home", temporary)
		}
	}
	resume := runner.plans[2]
	if !slices.Contains(resume.Args, "resume") || !slices.Contains(resume.Args, "00000000-0000-4000-8000-000000000001") || environmentValue(resume.Env, "CODEX_HOME") != home {
		t.Fatalf("continuation was not bound to the owned session: %#v", resume)
	}
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("verification output remains after success: %v", err)
	}
}

func TestVerifyCodexOwnsClientWorkspace(t *testing.T) {
	cfg, runtime := configuredCodexVerification(t, filepath.Join(t.TempDir(), "config.toml"))
	primary := errors.New("client invocation failed")
	for _, test := range []struct {
		name           string
		requestErr     error
		cleanupFailure bool
	}{
		{"none", nil, false},
		{"request", primary, false},
		{"cleanup", nil, true},
		{"request and cleanup", primary, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			scratch := t.TempDir()
			for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
				t.Setenv(name, scratch)
			}
			var workspace, removed string
			remove := removeVerificationWorkspace
			t.Cleanup(func() { removeVerificationWorkspace = remove })
			removeVerificationWorkspace = func(path string) error {
				removed = path
				if test.cleanupFailure {
					return &os.PathError{Op: "remove", Path: path, Err: os.ErrPermission}
				}
				return remove(path)
			}
			probe := &recordingCaptureRunner{version: "codex-cli 9.9.9", marker: "AIGW_OK", requestErr: test.requestErr, prepareOutput: func(path string) error {
				workspace = filepath.Dir(path)
				if workspace == scratch || filepath.Dir(workspace) != scratch {
					t.Fatalf("verification does not own a private workspace: %s", workspace)
				}
				return os.WriteFile(filepath.Join(workspace, "client-output"), []byte("owned"), 0o600)
			}}
			_, err := VerifyInvocation(t.Context(), probe, cfg, runtime, "")
			if test.requestErr != nil && !errors.Is(err, test.requestErr) {
				t.Fatalf("verification outcome = %v", err)
			}
			if removed != workspace {
				t.Fatalf("cleanup target = %q, want %q", removed, workspace)
			}
			if test.cleanupFailure {
				if !errors.Is(err, os.ErrPermission) || !strings.Contains(err.Error(), workspace) {
					t.Fatalf("verification lost cleanup cause: %v", err)
				}
				return
			}
			if test.requestErr == nil && err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(workspace); !os.IsNotExist(err) {
				t.Fatalf("client workspace survived: %s, %v", workspace, err)
			}
		})
	}
}

func TestVerifyCodexRequiresAvailableCapability(t *testing.T) {
	target := filepath.Join(t.TempDir(), "codex", "config.toml")
	cfg, runtime := configuredCodexVerification(t, target)
	executable := cfg.Clients[ID].Executable

	disabled := cfg.Clone()
	disabled.Clients[ID] = domain.ClientBinding{}
	if _, err := VerifyInvocation(context.Background(), &recordingCaptureRunner{}, disabled, runtime, ""); err == nil || !strings.Contains(err.Error(), "adapter is disabled") {
		t.Fatalf("disabled adapter error = %v", err)
	}
	missingExecutable := cfg.Clone()
	missingExecutable.SetClientActivation(ID, true, "", []string{target})
	if _, err := VerifyInvocation(context.Background(), &recordingCaptureRunner{}, missingExecutable, runtime, ""); err == nil || !strings.Contains(err.Error(), "executable is not configured") {
		t.Fatalf("missing executable error = %v", err)
	}
	missingTarget := cfg.Clone()
	missingTarget.SetClientActivation(ID, true, executable, nil)
	if _, err := VerifyInvocation(context.Background(), &recordingCaptureRunner{}, missingTarget, runtime, ""); err == nil || !strings.Contains(err.Error(), "configuration target is missing") {
		t.Fatalf("missing target error = %v", err)
	}
	missingModel := runtime
	missingModel.Model = ""
	if _, err := VerifyInvocation(context.Background(), &recordingCaptureRunner{}, cfg, missingModel, ""); err == nil || !strings.Contains(err.Error(), "has no Codex model") {
		t.Fatalf("missing model error = %v", err)
	}
	if _, err := VerifyInvocation(context.Background(), nil, cfg, runtime, ""); err == nil || !strings.Contains(err.Error(), "capture") {
		t.Fatalf("capture error = %v", err)
	}
	missingOnDisk := cfg.Clone()
	missingOnDisk.SetClientActivation(ID, true, filepath.Join(t.TempDir(), "missing-codex"), []string{target})
	if _, err := VerifyInvocation(context.Background(), &recordingCaptureRunner{}, missingOnDisk, runtime, ""); err == nil || !strings.Contains(err.Error(), "read Codex executable") {
		t.Fatalf("missing executable file error = %v", err)
	}
	drifted := cfg.Clone()
	drifted.Routes["codex"] = domain.Route{Label: "Codex", Account: "one", Model: "other"}
	if _, err := VerifyInvocation(context.Background(), &recordingCaptureRunner{}, drifted, domain.Runtime{RouteID: "codex", Model: "other"}, ""); err == nil || !strings.Contains(err.Error(), "synchronized") {
		t.Fatalf("projection error = %v", err)
	}
}

func TestVerifyCodexRequiresSuccessfulFinalMessage(t *testing.T) {
	target := filepath.Join(t.TempDir(), "codex", "config.toml")
	cfg, runtime := configuredCodexVerification(t, target)
	for name, test := range map[string]struct {
		marker, want string
		missing      bool
	}{
		"wrong":     {marker: "wrong", want: "private verification challenge"},
		"missing":   {missing: true, want: "read Codex final response"},
		"oversized": {marker: strings.Repeat("x", 1024), want: "exceeds"},
	} {
		t.Run(name, func(t *testing.T) {
			runner := &recordingCaptureRunner{version: "codex-cli 9.9.9", marker: test.marker, removeFinalMessage: test.missing}
			if _, err := VerifyInvocation(t.Context(), runner, cfg, runtime, ""); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("final message error = %v, want %q", err, test.want)
			}
			if outputPath := finalMessagePath(runner.plans[len(runner.plans)-1].Args); outputPath == "" {
				t.Fatal("request plan has no output path")
			} else if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
				t.Fatalf("verification output remains: %v", err)
			}
		})
	}
	requestFailure := &recordingCaptureRunner{
		version:       "codex-cli 9.9.9",
		requestOutput: []byte(`{"type":"thread.started","thread_id":"secret-session"}` + "\n" + `{"type":"error","message":"model gpt-next is unavailable at https://gateway.example/v1 (request id: secret-request); token=must-not-leak"}` + "\n"),
		stderr:        []byte("workdir: /Users/operator/private\n"),
		requestErr:    errors.New("exit status 1"),
	}
	_, err := VerifyInvocation(context.Background(), requestFailure, cfg, runtime, "")
	if err == nil {
		t.Fatal("failed Codex request was accepted")
	}
	for _, want := range []string{
		"Codex minimal verification request failed",
		"selected model is unavailable through the client or endpoint",
		"aigw verify --for codex",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("request error lacks %q: %v", want, err)
		}
	}
	for _, forbidden := range []string{"must-not-leak", "/Users/operator", "secret-session", "secret-request", "https://gateway.example"} {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatalf("request error exposed %q: %v", forbidden, err)
		}
	}
	if outputPath := finalMessagePath(requestFailure.plans[len(requestFailure.plans)-1].Args); outputPath == "" {
		t.Fatal("failed request plan has no output path")
	} else if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("verification output remains after failed request: %v", err)
	}
}

func verificationConfig() domain.Config {
	cfg := domain.NewConfig(testAdmission())
	cfg.Accounts["one"] = domain.Account{Label: "One", Endpoints: domain.Endpoints{
		OpenAIResponses: "https://one.test/v1",
		Anthropic:       "https://one.test",
	}}
	cfg.Routes["codex"] = domain.Route{Label: "Codex", Account: "one", Model: "gpt-test", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
	cfg.Routes["claude"] = domain.Route{Label: "Claude", Account: "one", Model: "claude-test", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.SetSelectedRoute(ID, "codex", "")
	cfg.SetSelectedRoute(otherClient.ID, "claude", "")
	return cfg
}
