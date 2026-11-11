package main

import (
	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/claudedesktop"
	clientverification "aigw-cli/internal/adapter/verification"
	builtinclients "aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform/process"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
	"aigw-cli/tools/release/identity"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestNativeProductJourney(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	newVersion, err := identity.ReadProductVersion(root)
	if err != nil {
		t.Fatal(err)
	}
	sourceBaseline := func(t *testing.T) string { return buildNativeProgram(t, root, "0.0.0") }
	currentProgram := func(t *testing.T) string {
		program, _, _ := nativeReleaseCandidate(t, root, newVersion)
		return program
	}

	server := newNativeJourneyServer(t)

	t.Run("ephemeral endpoint credentials", func(t *testing.T) {
		runNativeEphemeralCredentials(t, currentProgram(t))
	})

	t.Run("delayed token and client activation", func(t *testing.T) {
		journey := newNativeJourney(t, currentProgram(t), server.URL+"/v1", false)
		if runtime.GOOS == "linux" {
			journey.setEnvironment(
				"DBUS_SESSION_BUS_ADDRESS",
				"unix:path="+filepath.Join(journey.root, "missing-session-bus.sock"),
			)
		}
		journey.run("setup", "--from", journey.manifest)
		journey.requireConfigContains(`id = 'native-system-keyring-probe'`, `id = 'unused'`)
		journey.requireNoClaudeProjection()

		journey.installClientFixture("claude")
		journey.run("sync")
		journey.requireNoClaudeProjection()
		journey.setEnvironment(secret.EnvironmentKey("native-system-keyring-probe"), "native-journey-token")
		beforePreview := readFile(t, journey.config)
		var preview struct {
			Selections map[string]string `json:"selections"`
		}
		if err := json.Unmarshal(journey.run("sync", "--dry-run", "--json"), &preview); err != nil {
			t.Fatal(err)
		}
		// sync names a client's model as provider/model, which the configured
		// model's provider and ID make.
		configured, err := store.NewStore(journey.config, builtinclients.Admission()).Load()
		if err != nil {
			t.Fatal(err)
		}
		want := probeModel.String()
		if _, found := configured.Models[probeModel]; !found {
			t.Fatalf("the journey configuration has no %s", probeModel)
		}
		if preview.Selections[claude.ID] != want || !bytes.Equal(beforePreview, readFile(t, journey.config)) {
			t.Fatalf("late sync preview did not plan the available Account without writing: %+v, want %s", preview, want)
		}
		journey.run("sync")
		journey.requireClaudeProjection()
		journey.requireCredentialBackend("native-journey-token", secret.BackendSelection{
			Kind:         "env",
			Availability: "available",
			Mutability:   "read_only",
			Persistence:  "explicit",
		})
		journey.requireClaudeCredential("native-journey-token")
		journey.run("check")
		journey.run("check", "claude", "--depth", "client")
		retained := journey.retainedCredential(claude.ID)
		journey.uninstallAndRequireInstallationRemoved()
		journey.requireWithdrawnCredentialDenied(retained, "native-journey-token")
		journey.requireConfigContains(`id = 'native-system-keyring-probe'`, `id = 'unused'`)
	})

	runDeferredClientInstallation(t, currentProgram, server.URL+"/v1")

	t.Run("one selected account does not require every token", func(t *testing.T) {
		journey := newNativeJourney(t, currentProgram(t), server.URL+"/v1", true)
		journey.prepareCodexLifecycle()
		journey.setEnvironment(secret.EnvironmentKey("native-system-keyring-probe"), "native-journey-token")
		journey.run("setup", "--from", journey.manifest)
		journey.requireConfigContains(`id = 'native-system-keyring-probe'`, `id = 'unused'`)
		journey.requireClaudeProjection()
		journey.run("check")
		var diagnosis struct {
			OK bool `json:"ok"`
		}
		if err := json.Unmarshal(journey.run("check", "--depth", "local", "--json"), &diagnosis); err != nil {
			t.Fatal(err)
		}
		if !diagnosis.OK {
			t.Fatal("the local check rejected a healthy partially connected catalog")
		}
		journey.requireCredentialBackend("native-journey-token", secret.BackendSelection{
			Kind: "env", Availability: "available", Mutability: "read_only", Persistence: "explicit",
		})
		journey.requireClaudeCredential("native-journey-token")
		journey.uninstallAndRequireInstallationRemoved()
	})

	t.Run("portable artifact lifecycle", func(t *testing.T) {
		retainedBaseline := requireNativeLifecycleBaseline(t, func() string { return sourceBaseline(t) })
		runNativeReleaseLifecycle(t, root, retainedBaseline, newVersion, server.URL+"/v1")
	})

	if runtime.GOOS == "linux" {
		t.Run("secure file fallback without session bus", func(t *testing.T) {
			candidate, _, _ := nativeReleaseCandidate(t, root, newVersion)
			runLinuxSecureFileFallback(t, candidate, server.URL+"/v1")
		})
	}

	// macOS qualification uses the published predecessor and its retained item;
	// an ad-hoc current-source fixture does not prove that authorization boundary.
	if os.Getenv("AIGW_VERIFY_SYSTEM_KEYRING") == "1" && runtime.GOOS != "darwin" {
		t.Run("system credential store", func(t *testing.T) {
			retainedBaseline := requireNativeLifecycleBaseline(t, func() string { return sourceBaseline(t) })
			runNativeCredentialJourney(t, root, retainedBaseline, server.URL+"/v1", newVersion)
		})
	}
}

func TestNativeProviderRetirementWithoutClients(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	artifact := buildNativeProgram(t, root, "0.0.0")
	journey := newNativeJourney(t, artifact, "https://unused.example.test", false)
	journey.run("setup", "--from", journey.manifest)
	// No client is enabled, which check refuses, so status reads the setup.
	var status struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(journey.run("status", "--json"), &status); err != nil || status.State != "deferred" {
		t.Fatalf("deferred setup reads as %q: %v", status.State, err)
	}
	journey.run("provider", "remove", "native-system-keyring-probe")
	journey.requireNoClaudeProjection()
	retired, err := store.NewStore(journey.config, builtinclients.Admission()).Load()
	if _, kept := retired.Providers["native-system-keyring-probe"]; err != nil || kept || len(retired.Models) != 1 {
		t.Fatalf("client-free retirement kept the provider or its model: %v", err)
	}
	journey.uninstallAndRequireInstallationRemoved()
}

type journeyFixture struct {
	testing              *testing.T
	source               string
	binary               string
	root                 string
	clientBin            string
	manifest             string
	config               string
	settings             string
	endpoint             string
	environment          []string
	sensitiveInputs      []string
	retainedProgramNames []string
}

func TestNativeJourneyCleanupSurvivesTestCancellation(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	program := buildNativeProgram(t, root, "0.0.0")
	t.Run("registered cleanup", func(t *testing.T) {
		journey := newNativeJourney(t, program, "https://unused.example.test", false)
		t.Cleanup(journey.uninstallAndRequireInstallationRemoved)
	})
}

// modelName is the provider/model name of a configured model, as aigw use
// takes it.
func (j *journeyFixture) modelName(ref domain.ModelRef) string {
	j.testing.Helper()
	cfg, err := store.NewStore(j.config, builtinclients.Admission()).Load()
	if err != nil {
		j.testing.Fatal(err)
	}
	if _, found := cfg.Models[ref]; !found {
		j.testing.Fatalf("no configured model %s", ref)
	}
	return ref.String()
}

// verifiedSnapshot is the verified snapshot beside a configuration file.
func verifiedSnapshot(config string) string {
	return strings.TrimSuffix(config, filepath.Ext(config)) + ".verified.json"
}

// probeModel is the model of the native journey's provider that Claude Code
// is recommended.
var probeModel = domain.NewModelRef("native-system-keyring-probe", "claude-test")

func newNativeJourney(t *testing.T, source, endpoint string, installClient bool) *journeyFixture {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	clientBin := filepath.Join(root, "client bin")
	for _, directory := range []string{home, clientBin} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(root)
	journey := &journeyFixture{
		testing:   t,
		source:    source,
		binary:    filepath.Join(root, "installed", executableName()),
		root:      root,
		clientBin: clientBin,
		manifest:  filepath.Join(root, "team.toml"),
		settings:  filepath.Join(home, ".claude", "settings.json"),
		endpoint:  endpoint,
	}
	switch runtime.GOOS {
	case "darwin":
		journey.config = filepath.Join(home, "Library", "Application Support", "aigw", "aigw.toml")
	case "linux":
		journey.config = filepath.Join(root, "config", "aigw", "aigw.toml")
	case "windows":
		journey.config = filepath.Join(root, "appdata", "aigw", "aigw.toml")
	default:
		t.Fatalf("unsupported native journey platform %s", runtime.GOOS)
	}
	journey.environment = environmentWith(os.Environ(), map[string]string{
		"HOME":                home,
		"USERPROFILE":         home,
		"XDG_CONFIG_HOME":     filepath.Join(root, "config"),
		"XDG_DATA_HOME":       filepath.Join(root, "data"),
		"APPDATA":             filepath.Join(root, "appdata"),
		"LOCALAPPDATA":        filepath.Join(root, "localappdata"),
		"CODEX_HOME":          filepath.Join(home, ".codex"),
		"CLAUDE_CONFIG_DIR":   filepath.Join(home, ".claude"),
		"HERMES_HOME":         filepath.Join(home, ".hermes"),
		"PATH":                clientBin,
		"AIGW_SECRET_BACKEND": "env",
		"NO_COLOR":            "1",
	})
	manifest := fmt.Sprintf(`version = 8

[[providers]]
id = "native-system-keyring-probe"
label = "Native System Keyring Probe"
endpoints = {anthropic = %q}

[[providers.models]]
id = "claude-test"
family = "claude-test"
label = "Native System Keyring Probe Claude"
protocols = {anthropic = ["text"]}

[[providers]]
id = "unused"
label = "Unused"
endpoints = {anthropic = %q}

[[providers.models]]
id = "claude-test"
family = "claude-test"
label = "Unused Claude"
protocols = {anthropic = ["text"]}

[[families]]
id = "claude-test"
label = "Claude Test"

[recommendations.claude]
primary = {model = "native-system-keyring-probe/claude-test"}
`, endpoint, endpoint)
	if err := os.WriteFile(journey.manifest, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if installClient {
		journey.installClientFixture("claude")
	}
	journey.runWith(source, "self", "install", "--path", journey.binary)
	journey.run("--version")
	return journey
}

func (j *journeyFixture) installClientFixture(client string) {
	j.testing.Helper()
	// Synchronous copied clients need no race-runtime exit grace; keep all other options.
	j.setEnvironment("GORACE", strings.TrimSpace(os.Getenv("GORACE")+" atexit_sleep_ms=0"))
	program, err := os.Executable()
	if err != nil {
		j.testing.Fatal(err)
	}
	name := client
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	target := filepath.Join(j.clientBin, name)
	if client == claudedesktop.ID {
		candidates := claudedesktop.ApplicationPathsFor(runtime.GOOS, map[string]string{
			"HOME": filepath.Join(j.root, "home"), "LOCALAPPDATA": filepath.Join(j.root, "localappdata"),
		})
		if len(candidates) > 0 {
			target = candidates[0]
		}
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		j.testing.Fatal(err)
	}
	if err := os.WriteFile(target, readFile(j.testing, program), 0o700); err != nil {
		j.testing.Fatal(err)
	}
}

func (j *journeyFixture) run(args ...string) []byte {
	j.testing.Helper()
	return j.runWithInput(j.binary, "", args...)
}

func (j *journeyFixture) runWith(binary string, args ...string) []byte {
	j.testing.Helper()
	return j.runWithInput(binary, "", args...)
}

func (j *journeyFixture) runWithInput(binary, input string, args ...string) []byte {
	j.testing.Helper()
	return j.runWithContext(j.testing.Context(), binary, input, args...)
}

func (j *journeyFixture) runWithContext(parent context.Context, binary, input string, args ...string) []byte {
	j.testing.Helper()
	stdout, _ := j.runStreamsWithContext(parent, binary, input, args...)
	return stdout
}

// runStreamsWithContext also returns the diagnostics stream, where native
// clients report session identities.
func (j *journeyFixture) runStreamsWithContext(parent context.Context, binary, input string, args ...string) ([]byte, []byte) {
	j.testing.Helper()
	if input != "" {
		j.sensitiveInputs = append(j.sensitiveInputs, input)
	}
	ctx, cancel := context.WithTimeout(parent, clientverification.ProtocolTimeout)
	defer cancel()
	stdout, stderr, err := (process.Runner{}).RunCaptureStreams(ctx, process.Plan{Executable: binary, Args: args, Env: j.environment, Stdin: input})
	if len(stderr) != 0 {
		j.testing.Logf("command stderr:\n%s", domain.Redact(string(stderr), j.sensitiveInputs...))
	}
	if err == nil && process.DiagnosticFailure(stderr) {
		err = fmt.Errorf("native product diagnostics prevent qualification")
	}
	if err != nil {
		j.testing.Fatalf("%s %s: %v\nstdout:\n%s\nstderr:\n%s", binary,
			domain.Redact(strings.Join(args, " "), j.sensitiveInputs...), err,
			domain.Redact(string(stdout), j.sensitiveInputs...), domain.Redact(string(stderr), j.sensitiveInputs...))
	}
	return stdout, stderr
}

func (j *journeyFixture) requireConfigContains(values ...string) {
	j.testing.Helper()
	requireFileContains(j.testing, j.config, values...)
}

func (j *journeyFixture) requireNoClaudeProjection() {
	j.testing.Helper()
	j.requireNoClientProjection(claude.ID)
}

func (j *journeyFixture) requireClaudeProjection() {
	j.testing.Helper()
	j.requireClientProjection(claude.ID)
	data, err := os.ReadFile(j.settings)
	if err != nil {
		j.testing.Fatal(err)
	}
	var settings struct {
		APIKeyHelper string `json:"apiKeyHelper"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		j.testing.Fatalf("decode Claude settings: %v", err)
	}
	if !strings.Contains(settings.APIKeyHelper, j.credentialEntrypoint()) {
		j.testing.Fatalf("Claude apiKeyHelper %q lacks %q", settings.APIKeyHelper, j.credentialEntrypoint())
	}
}

func (j *journeyFixture) uninstallAndRequireInstallationRemoved() {
	j.testing.Helper()
	j.uninstallWithAndRequireInstallationRemoved(j.source)
	j.requireNoClaudeProjection()
	if _, err := os.Stat(verifiedSnapshot(j.config)); !os.IsNotExist(err) {
		j.testing.Fatalf("uninstall retained verified checkpoint: %v", err)
	}
}

func (j *journeyFixture) uninstallWithAndRequireInstallationRemoved(binary string) {
	j.testing.Helper()
	reader := j.captureCredentialReader()
	j.runWithContext(context.WithoutCancel(j.testing.Context()), binary, "", "self", "uninstall", "--path", j.binary)
	j.requireInstallationRemoved()
	reader.requireUnchanged(j.testing)
}

func (j *journeyFixture) requireInstallationRemoved() {
	j.testing.Helper()
	backup := filepath.Join(filepath.Dir(j.binary), ".aigw.previous")
	if runtime.GOOS == "windows" {
		backup += ".exe"
	}
	for _, path := range []string{j.binary, backup} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			j.testing.Fatalf("uninstall retained owned file %s: %v", path, err)
		}
	}
	entries, err := os.ReadDir(filepath.Dir(j.binary))
	if err != nil {
		j.testing.Fatal(err)
	}
	if !slices.Equal(entryNames(entries), j.retainedProgramNames) {
		j.testing.Fatalf("uninstall retained lifecycle residue: %#v", entryNames(entries))
	}
}

func executableName() string {
	if runtime.GOOS == "windows" {
		return "aigw.exe"
	}
	return "aigw"
}

func requireFileContains(t *testing.T, path string, values ...string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	for _, value := range values {
		if !bytes.Contains(data, []byte(value)) {
			t.Fatalf("%s lacks %q:\n%s", path, value, data)
		}
	}
}

func entryNames(entries []os.DirEntry) []string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func TestRetainedClaudeCredentialCommandsRemainIndependent(t *testing.T) {
	root := t.TempDir()
	journey := &journeyFixture{
		testing: t, root: root, settings: filepath.Join(root, "settings.json"),
		environment: os.Environ(),
	}
	writeCommand := func(value string) {
		t.Helper()
		data, err := json.Marshal(map[string]string{"apiKeyHelper": "echo " + value})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(journey.settings, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeCommand("original-fixture-token")
	original := journey.retainedCredential(claude.ID)
	writeCommand("successor-fixture-token")
	successor := journey.retainedCredential(claude.ID)
	if err := os.Remove(journey.settings); err != nil {
		t.Fatal(err)
	}
	journey.requireCredential(original, "original-fixture-token")
	journey.requireCredential(successor, "successor-fixture-token")
}
