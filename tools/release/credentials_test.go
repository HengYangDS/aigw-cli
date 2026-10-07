package main

import (
	clientverification "aigw-cli/internal/client/verification"
	"aigw-cli/internal/codex"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/platform"
	"aigw-cli/internal/process"
	"aigw-cli/internal/redaction"
	"aigw-cli/internal/secrets"
	"aigw-cli/internal/transaction"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

func runInstalledClientFixture(executable string, args []string) (bool, int) {
	if role := os.Getenv("AIGW_TEST_RESOURCE_ROLE"); role != "" {
		return true, runVerificationResourceRole(role, args)
	}
	client := strings.TrimSuffix(filepath.Base(executable), filepath.Ext(executable))
	if client == configuration.ClientHermes {
		if slices.Equal(args, []string{"--version"}) {
			_, _ = fmt.Fprintln(os.Stdout, "hermes 0.0.0-resource-fixture")
			return true, 0
		}
		return true, runVerificationResourceClient()
	}
	if client == configuration.ClientClaude {
		_, _ = fmt.Fprintln(os.Stdout, "AIGW_OK")
		return true, 0
	}
	if client != configuration.ClientCodex {
		return false, 0
	}
	if slices.Equal(args, []string{"--version"}) {
		_, _ = fmt.Fprintln(os.Stdout, "codex-cli 0.0.0-fixture")
		return true, 0
	}
	for index, argument := range args {
		if argument == "--output-last-message" && index+1 < len(args) {
			if slices.Contains(args, "--json") {
				return true, runCodexVerificationFixture(args, args[index+1])
			}
			if err := os.WriteFile(args[index+1], []byte("AIGW_OK\n"), 0o600); err != nil {
				return true, 3
			}
			return true, 0
		}
	}
	return true, 2
}

func runCodexVerificationFixture(args []string, outputPath string) int {
	var state struct {
		Session string `json:"session"`
		Marker  string `json:"marker"`
	}
	home := os.Getenv("CODEX_HOME")
	if home == "" {
		return 3
	}
	statePath := filepath.Join(home, "fixture-session.json")
	challengePath := filepath.Join(filepath.Dir(outputPath), "challenge.txt")
	resume := slices.Contains(args, "resume")
	switch {
	case resume:
		data, err := os.ReadFile(statePath)
		if err != nil || json.Unmarshal(data, &state) != nil || len(args) < 2 || args[len(args)-2] != state.Session || state.Marker == "" {
			return 3
		}
		if _, err := os.Stat(challengePath); !os.IsNotExist(err) {
			return 3
		}
	default:
		challenge, err := os.ReadFile(challengePath)
		if err != nil {
			return 3
		}
		state.Session = "00000000-0000-4000-8000-000000000001"
		state.Marker = strings.TrimSpace(string(challenge))
		data, err := json.Marshal(state)
		if err != nil || os.WriteFile(statePath, data, 0o600) != nil {
			return 3
		}
	}
	if err := os.WriteFile(outputPath, []byte(state.Marker+"\n"), 0o600); err != nil {
		return 3
	}
	events := fmt.Appendf(nil, "{\"type\":\"thread.started\",\"thread_id\":%q}\n", state.Session)
	if !resume {
		events = fmt.Appendf(events, "{\"type\":\"item.completed\",\"item\":{\"type\":\"command_execution\",\"status\":\"completed\",\"command\":\"cat challenge.txt\",\"exit_code\":0,\"aggregated_output\":%q}}\n", state.Marker+"\n")
	}
	events = fmt.Appendf(events, "{\"type\":\"item.completed\",\"item\":{\"type\":\"agent_message\",\"text\":%q}}\n{\"type\":\"turn.completed\"}\n", state.Marker)
	if _, err := os.Stdout.Write(events); err != nil {
		return 3
	}
	return 0
}

type verificationResourceProcess struct {
	PID     int    `json:"pid"`
	Home    string `json:"home"`
	Control string `json:"control"`
	Role    string `json:"role"`
}

func runVerificationResourceRole(role string, args []string) int {
	if role == "interrupt" {
		if len(args) != 1 {
			return 2
		}
		pid, err := strconv.Atoi(args[0])
		if err != nil {
			return 2
		}
		if err := sendVerificationConsoleInterrupt(pid); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return 2
		}
		return 0
	}
	if role != "child" && role != "unrelated" {
		return 2
	}
	if err := writeVerificationProcess(filepath.Join(os.Getenv("AIGW_TEST_RESOURCE_CONTROL"), role+".json")); err != nil {
		return 2
	}
	return holdVerificationProcess(role, 3*time.Minute)
}

func runVerificationResourceClient() int {
	control := os.Getenv("AIGW_TEST_RESOURCE_CONTROL")
	child := exec.Command(os.Args[0])
	prepareVerificationInterrupt(child)
	child.Env = environmentWith(os.Environ(), map[string]string{"AIGW_TEST_RESOURCE_ROLE": "child"})
	if err := child.Start(); err != nil {
		return 2
	}
	if err := awaitVerificationFile(filepath.Join(control, "child.json")); err != nil {
		_ = child.Process.Kill()
		_ = child.Wait()
		return 2
	}
	if err := writeVerificationProcess(filepath.Join(control, "parent.json")); err != nil {
		return 2
	}
	if err := awaitVerificationFile(filepath.Join(control, "release")); err != nil {
		return 2
	}
	switch os.Getenv("AIGW_TEST_RESOURCE_CASE") {
	case "success":
		_, _ = fmt.Fprintln(os.Stdout, "AIGW_OK")
		return 0
	case "failure":
		return 17
	case "parent-exit":
		return 0
	case "interrupt", "deadline":
		return holdVerificationProcess("parent", 2*time.Minute)
	default:
		return 2
	}
}

func writeVerificationProcess(path string) error {
	role := os.Getenv("AIGW_TEST_RESOURCE_ROLE")
	if role == "" {
		role = "parent"
	}
	data, err := json.Marshal(verificationResourceProcess{
		PID: os.Getpid(), Home: os.Getenv("HERMES_HOME"),
		Control: os.Getenv("AIGW_TEST_RESOURCE_CONTROL"), Role: role,
	})
	if err != nil {
		return err
	}
	return transaction.WriteFileAtomicExactMode(path, data, 0o600)
}

func TestVerificationProcessPublicationDoesNotMutateObservedBytes(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "parent.json")
	if err := writeVerificationProcess(path); err != nil {
		t.Fatal(err)
	}
	observed := filepath.Join(root, "observed.json")
	if err := os.Link(path, observed); err != nil {
		t.Fatal(err)
	}
	original := readFile(t, observed)
	t.Setenv("AIGW_TEST_RESOURCE_ROLE", "child")
	if err := writeVerificationProcess(path); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(readFile(t, observed), original) {
		t.Fatal("publishing process readiness changed bytes already observed by a reader")
	}
	var process verificationResourceProcess
	if err := json.Unmarshal(readFile(t, path), &process); err != nil || process.Role != "child" {
		t.Fatalf("published process = %#v, error = %v", process, err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 2 {
		t.Fatalf("publication left temporary entries: %v, %v", entries, err)
	}
}

func holdVerificationProcess(role string, limit time.Duration) int {
	stop := filepath.Join(os.Getenv("AIGW_TEST_RESOURCE_CONTROL"), role+".stop")
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(stop); err == nil {
			return 0
		} else if !errors.Is(err, os.ErrNotExist) {
			return 2
		}
		time.Sleep(10 * time.Millisecond)
	}
	return 2
}

func awaitVerificationFile(path string) error {
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("verification fixture did not reach %s", filepath.Base(path))
}

func TestCodexFixtureWritesItsFinalResponse(t *testing.T) {
	fixture := &journeyFixture{testing: t, clientBin: t.TempDir()}
	fixture.installClientFixture(configuration.ClientCodex)
	executable := filepath.Join(fixture.clientBin, configuration.ClientCodex)
	if runtime.GOOS == "windows" {
		executable += ".exe"
	}
	if version, err := exec.Command(executable, "--version").Output(); err != nil || !strings.Contains(string(version), "codex-cli") {
		t.Fatalf("Codex fixture version = %q, %v", version, err)
	}
	response := filepath.Join(t.TempDir(), "response.txt")
	if output, err := exec.Command(executable, "exec", "--output-last-message", response).Output(); err != nil || len(output) != 0 {
		t.Fatalf("Codex fixture output = %q, %v", output, err)
	}
	if got := strings.TrimSpace(string(readFile(t, response))); got != "AIGW_OK" {
		t.Fatalf("Codex fixture final response = %q", got)
	}
}

func TestCodexFixtureCompletesCurrentVerification(t *testing.T) {
	fixture := &journeyFixture{testing: t, clientBin: t.TempDir()}
	fixture.installClientFixture(configuration.ClientCodex)
	executable := filepath.Join(fixture.clientBin, configuration.ClientCodex)
	if runtime.GOOS == "windows" {
		executable += ".exe"
	}
	reader, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	selected := configuration.Runtime{
		RouteID: "fixture", RouteLabel: "Fixture", AccountID: "fixture",
		Client: configuration.ClientCodex, Endpoint: "https://api.example.invalid/v1",
		Model: "gpt-test", CredentialCommand: reader,
	}
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, []byte("model_provider = 'native'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := codex.SyncConfig(target, selected); err != nil {
		t.Fatal(err)
	}
	cfg := configuration.Config{}
	cfg.SetClientActivation(configuration.ClientCodex, true, executable, []string{target})
	if _, err := clientverification.VerifyCodexInvocation(t.Context(), process.Runner{}, cfg, selected); err != nil {
		t.Fatalf("copied Codex fixture cannot satisfy the current verifier: %v", err)
	}
}

func (j *journeyFixture) requireExternalCredentialClient(client, executable, account string, completed func() int64) {
	j.testing.Helper()
	count := completed()
	program, err := os.Executable()
	if err != nil {
		j.testing.Fatal(err)
	}
	helper := filepath.Join(j.root, "credential helper")
	if runtime.GOOS == "windows" {
		helper += ".exe"
	}
	if err := os.WriteFile(helper, readFile(j.testing, program), 0o700); err != nil {
		j.testing.Fatal(err)
	}
	store := configuration.NewStore(j.config)
	cfg, err := store.Load()
	if err != nil {
		j.testing.Fatal(err)
	}
	adapter := cfg.Clients[client]
	adapter.CredentialCommand = helper
	cfg.Clients[client] = adapter
	if err := store.Save(cfg); err != nil {
		j.testing.Fatal(err)
	}
	selected, err := cfg.ResolveRuntime(client, "")
	if err != nil {
		j.testing.Fatal(err)
	}
	j.environment = environmentWithout(j.environment, secrets.EnvironmentKey(account))
	j.setEnvironment("AIGW_TEST_EXTERNAL_CREDENTIAL", "1")
	j.setEnvironment("AIGW_TEST_EXTERNAL_CLIENT", client)
	j.setEnvironment("AIGW_TEST_EXTERNAL_FINGERPRINT", selected.CredentialProjectionFingerprint(client))
	before := readFile(j.testing, j.config)
	j.run("sync", "--dry-run", "--json")
	if !bytes.Equal(before, readFile(j.testing, j.config)) {
		j.testing.Fatal("external helper dry-run changed host configuration")
	}
	j.run("sync")
	j.run("check", "--json")
	j.run("verify", "--for", client)
	j.run("client", "disable", client)
	j.run("sync")
	args := []string{"client", "enable", client, "--executable", executable}
	if client == configuration.ClientCodex {
		args = append(args, "--target", filepath.Join(j.root, "home", ".codex", "config.toml"))
	}
	j.run(args...)
	j.run("sync")
	j.run("verify", "--for", client)
	retained, err := store.Load()
	if err != nil || retained.Clients[client].CredentialCommand != helper {
		j.testing.Fatalf("native lifecycle discarded explicit helper: %v", err)
	}
	if completed() < count+2 {
		j.testing.Fatal("external helper did not authenticate both real-client invocations")
	}
}

func (j *journeyFixture) credentialEntrypoint() string {
	j.testing.Helper()
	paths, err := platform.PathsFor(runtime.GOOS, environmentValues(j.environment))
	if err != nil {
		j.testing.Fatal(err)
	}
	path, err := credential.VersionedEntrypointPath(paths.Data, j.binary, paths.InstallName)
	if err != nil {
		j.testing.Fatal(err)
	}
	return path
}

func (j *journeyFixture) requireClaudeCredential(want string) {
	j.testing.Helper()
	j.requireCredential(j.retainedCredential(configuration.ClientClaude), want)
}

func (j *journeyFixture) requireCredential(plan process.Plan, want string) {
	j.testing.Helper()
	ctx, cancel := context.WithTimeout(j.testing.Context(), 10*time.Second)
	defer cancel()
	stdout, stderr, err := (process.Runner{}).RunCaptureStreams(ctx, plan)
	sensitive := append(slices.Clone(j.sensitiveInputs), want)
	if len(stderr) != 0 {
		j.testing.Logf("retained credential stderr:\n%s", redaction.Text(string(stderr), sensitive...))
	}
	if process.DiagnosticFailure(stderr) {
		err = errors.Join(err, fmt.Errorf("retained credential diagnostics prevent qualification"))
	}
	if err != nil {
		j.testing.Fatalf("execute retained credential command: %v\nstderr:\n%s", err,
			redaction.Text(string(stderr), sensitive...))
	}
	if strings.TrimSpace(string(stdout)) != want {
		j.testing.Fatal("retained credential command returned unexpected content")
	}
}

func (j *journeyFixture) requireCredentialBackend(token string, want secrets.BackendSelection) {
	j.testing.Helper()
	for _, command := range [][]string{{"status", "--json"}, {"doctor", "--json"}} {
		output := j.run(command...)
		if bytes.Contains(output, []byte(token)) {
			j.testing.Fatalf("%s disclosed the credential", strings.Join(command, " "))
		}
		var result struct {
			CredentialBackend secrets.BackendSelection `json:"credential_backend"`
		}
		if err := json.Unmarshal(output, &result); err != nil {
			j.testing.Fatalf("decode %s: %v", strings.Join(command, " "), err)
		}
		if result.CredentialBackend != want {
			j.testing.Fatalf("%s credential backend = %#v, want %#v", strings.Join(command, " "), result.CredentialBackend, want)
		}
	}
}

func runNativeEphemeralCredentials(t *testing.T, artifact string) {
	t.Helper()
	const token = "native-ephemeral-token"
	requests := 0
	endpoint := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests++
		if request.Header.Get("X-Api-Key") != token {
			response.WriteHeader(http.StatusUnauthorized)
			return
		}
		response.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(endpoint.Close)
	journey := newNativeJourney(t, artifact, endpoint.URL, false)
	journey.run("setup", "--from", journey.manifest)
	before := readFile(t, journey.config)
	for _, format := range []string{"raw", "go-keyring-base64"} {
		input := token + "\r\n"
		if format == "go-keyring-base64" {
			input = "go-keyring-base64:" + base64.StdEncoding.EncodeToString([]byte(token))
		}
		output := journey.runWithInput(journey.binary, input, "test", "--for", "claude", "--route", "native-system-keyring-probe-claude", "--token-stdin", "--token-format", format, "--config", journey.config)
		if !bytes.Contains(output, []byte("not model inference")) || bytes.Contains(output, []byte(token)) {
			t.Fatal("endpoint result lost its evidence or secret boundary")
		}
	}
	if requests != 2 || !bytes.Equal(before, readFile(t, journey.config)) {
		t.Fatalf("ephemeral endpoint requests=%d or configuration changed", requests)
	}
	journey.requireNoClaudeProjection()
	journey.uninstallAndRequireInstallationRemoved()
}

func requiredClientInput(key string, directory bool) (string, error) {
	path := os.Getenv(key)
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("%s requires an explicit absolute path", key)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("%s: %w", key, err)
	}
	if info.IsDir() != directory || (!directory && !info.Mode().IsRegular()) {
		return "", fmt.Errorf("%s has the wrong input kind", key)
	}
	return path, nil
}

func TestRetainedCredentialFailurePreservesSafeDiagnostics(t *testing.T) {
	const token = "credential-diagnostic-token"
	const marker = "credential-reader-rejected"
	if os.Getenv("AIGW_TEST_CREDENTIAL_DIAGNOSTIC") == "child" {
		if os.Getenv("AIGW_TEST_CREDENTIAL_DIAGNOSTIC_SOURCE") == "reader" {
			_, _ = fmt.Fprintln(os.Stderr, marker, token)
			os.Exit(23)
		}
		program, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		journey := &journeyFixture{testing: t, sensitiveInputs: []string{token}}
		journey.requireCredential(process.Plan{
			Executable: program,
			Args:       []string{"-test.run=^TestRetainedCredentialFailurePreservesSafeDiagnostics$"},
			Env:        append(os.Environ(), "AIGW_TEST_CREDENTIAL_DIAGNOSTIC_SOURCE=reader"),
		}, token)
		return
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := (process.Runner{}).RunCaptureStreams(t.Context(), process.Plan{
		Executable: program,
		Args:       []string{"-test.run=^TestRetainedCredentialFailurePreservesSafeDiagnostics$"},
		Env:        append(os.Environ(), "AIGW_TEST_CREDENTIAL_DIAGNOSTIC=child"),
	})
	if _, failed := errors.AsType[*exec.ExitError](err); !failed {
		t.Fatalf("failing credential fixture did not fail: %v", err)
	}
	diagnostic := string(stdout) + string(stderr)
	if !strings.Contains(diagnostic, marker) || strings.Contains(diagnostic, token) {
		t.Fatalf("credential failure lost its safe cause or leaked its Token: %q", diagnostic)
	}
}

func TestJourneyQualifiesRedactedChildDiagnostics(t *testing.T) {
	const secret = "synthetic-diagnostic-value"
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	switch mode := os.Getenv("AIGW_TEST_JOURNEY_DIAGNOSTIC"); mode {
	case "writer":
		_, _ = fmt.Fprintln(os.Stdout, secret)
		_, _ = fmt.Fprintln(os.Stderr, os.Getenv("AIGW_TEST_JOURNEY_MARKER"), secret)
		os.Exit(0)
	case "journey", "retained":
		journey := &journeyFixture{testing: t, sensitiveInputs: []string{secret}, environment: append(os.Environ(), "AIGW_TEST_JOURNEY_DIAGNOSTIC=writer")}
		args := []string{"-test.run=^TestJourneyQualifiesRedactedChildDiagnostics$"}
		if mode == "retained" {
			journey.sensitiveInputs = nil
			journey.requireCredential(process.Plan{Executable: program, Args: args, Env: journey.environment}, secret)
		} else {
			journey.runWith(program, args...)
		}
		return
	}
	for _, test := range []struct {
		name   string
		mode   string
		marker string
		fails  bool
	}{
		{name: "progress", mode: "journey", marker: "Working: successful-child-canary"},
		{name: "warning", mode: "journey", marker: "Warning: successful-child-canary", fails: true},
		{name: "retained-progress", mode: "retained", marker: "Working: successful-child-canary"},
		{name: "retained-warning", mode: "retained", marker: "Warning: successful-child-canary", fails: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, err := (process.Runner{}).RunCaptureStreams(t.Context(), process.Plan{
				Executable: program, Args: []string{"-test.run=^TestJourneyQualifiesRedactedChildDiagnostics$", "-test.v"},
				Env: append(os.Environ(), "AIGW_TEST_JOURNEY_DIAGNOSTIC="+test.mode, "AIGW_TEST_JOURNEY_MARKER="+test.marker),
			})
			_, failed := errors.AsType[*exec.ExitError](err)
			if failed != test.fails || (err != nil && !failed) || !bytes.Contains(stdout, []byte(test.marker+" [REDACTED]")) || bytes.Contains(stdout, []byte(secret)) || len(stderr) != 0 {
				t.Fatal("journey diagnostic lost qualification or safe redacted evidence")
			}
		})
	}
}
