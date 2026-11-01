package main

import (
	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/claudedesktop"
	clientverification "aigw-cli/internal/adapter/verification"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform/process"
	"aigw-cli/internal/secret"
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
		journey.requireConfigContains("native-system-keyring-probe-claude", "unused-claude")
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
		if preview.Selections[claude.ID] != "native-system-keyring-probe-claude" || !bytes.Equal(beforePreview, readFile(t, journey.config)) {
			t.Fatalf("late sync preview did not plan the available Account without writing: %+v", preview)
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
		journey.run("verify", "--for", "claude")
		retained := journey.retainedCredential(claude.ID)
		journey.uninstallAndRequireInstallationRemoved()
		journey.requireWithdrawnCredentialDenied(retained, "native-journey-token")
		journey.requireConfigContains("native-system-keyring-probe-claude", "unused-claude")
	})

	runDeferredClientInstallation(t, currentProgram, server.URL+"/v1")

	t.Run("one selected account does not require every token", func(t *testing.T) {
		journey := newNativeJourney(t, currentProgram(t), server.URL+"/v1", true)
		journey.prepareCodexLifecycle()
		journey.setEnvironment(secret.EnvironmentKey("native-system-keyring-probe"), "native-journey-token")
		journey.run("setup", "--from", journey.manifest, "--account", "native-system-keyring-probe")
		journey.requireConfigContains("native-system-keyring-probe-claude", "unused-claude")
		journey.requireClaudeProjection()
		journey.run("check")
		var diagnosis struct {
			OK bool `json:"ok"`
		}
		if err := json.Unmarshal(journey.run("doctor", "--json"), &diagnosis); err != nil {
			t.Fatal(err)
		}
		if !diagnosis.OK {
			t.Fatal("doctor rejected a healthy partially connected catalog")
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

func TestNativeAccountRetirementWithoutClients(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	artifact := buildNativeProgram(t, root, "0.0.0")
	journey := newNativeJourney(t, artifact, "https://unused.example.test", false)
	journey.run("setup", "--from", journey.manifest)
	var diagnosis struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(journey.run("doctor", "--json"), &diagnosis); err != nil || !diagnosis.OK {
		t.Fatalf("deferred setup is not corrupt: %v", err)
	}
	journey.run("account", "rename", "native-system-keyring-probe", "renamed-account")
	journey.run("account", "rename", "native-system-keyring-probe", "renamed-account", "--finalize")
	journey.requireNoClaudeProjection()
	if !bytes.Equal(readFile(t, journey.config), readFile(t, journey.config+".bak")) {
		t.Fatal("client-free retirement did not converge backup")
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
		journey.config = filepath.Join(home, "Library", "Application Support", "aigw", "config.toml")
	case "linux":
		journey.config = filepath.Join(root, "config", "aigw", "config.toml")
	case "windows":
		journey.config = filepath.Join(root, "appdata", "aigw", "config.toml")
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
	manifest := fmt.Sprintf(`version = 7

[recommendations.claude.primary]
route = "native-system-keyring-probe-claude"

[accounts.native-system-keyring-probe]
label = "Native System Keyring Probe"

[accounts.native-system-keyring-probe.endpoints]
anthropic = %q

[accounts.unused]
label = "Unused"

[accounts.unused.endpoints]
anthropic = %q

[models.claude-test]
label = "Claude Test"

[routes.native-system-keyring-probe-claude]
label = "Native System Keyring Probe Claude"
account = "native-system-keyring-probe"
model = "claude-test"
upstream_model = "claude-test"
interfaces = { anthropic = ["text"] }

[routes.unused-claude]
label = "Unused Claude"
account = "unused"
model = "claude-test"
upstream_model = "claude-test"
interfaces = { anthropic = ["text"] }
`, endpoint, endpoint)
	if err := os.WriteFile(journey.manifest, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if installClient {
		journey.installClientFixture("claude")
	}
	journey.runWith(source, "install", "--path", journey.binary)
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
	if _, err := os.Stat(j.config + ".verified.json"); !os.IsNotExist(err) {
		j.testing.Fatalf("uninstall retained verified checkpoint: %v", err)
	}
}

func (j *journeyFixture) uninstallWithAndRequireInstallationRemoved(binary string) {
	j.testing.Helper()
	reader := j.captureCredentialReader()
	j.runWithContext(context.WithoutCancel(j.testing.Context()), binary, "", "uninstall", "--path", j.binary)
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
