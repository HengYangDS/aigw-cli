package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"aigw-cli/internal/configuration"
	"aigw-cli/internal/platform"
	"aigw-cli/internal/process"
	"aigw-cli/internal/redaction"
	"aigw-cli/internal/secrets"

	"github.com/pelletier/go-toml/v2"
)

func runNativeCredentialJourney(t *testing.T, root, artifact, endpoint, newVersion string) {
	t.Helper()
	const (
		sourceAccount = "native-system-keyring-probe"
		targetAccount = "renamed-system-keyring-probe"
		token         = "native-system-keyring-token"
		replacement   = "native-system-keyring-replacement"
		diagnostic    = "native-diagnostic-token"
		diagnosticID  = "native-diagnostic-user"
	)
	baseline := requireNativeLifecycleBaseline(t, func() string { return artifact })
	journey := newNativeJourney(t, baseline, endpoint, true)
	oldVersion := journey.predecessorVersion(newVersion)
	journey.enableSystemCredentialStore()
	journey.prepareCodexLifecycle()
	configureNativeDiagnosticProbe(t, journey, sourceAccount, endpoint)
	candidate, archive, checksums := nativeReleaseCandidate(t, root, newVersion)
	store, err := secrets.Select(secrets.Selection{Backend: "keyring", Executable: candidate})
	if err != nil {
		t.Fatal(err)
	}
	requireUnoccupiedNativeCredentialSlots(t, store, sourceAccount, targetAccount)
	diagnosticSlots, err := secrets.ForKind(store, secrets.ProviderDiagnostic)
	if err != nil {
		t.Fatal(err)
	}
	requireUnoccupiedNativeCredentialSlots(t, diagnosticSlots, sourceAccount, targetAccount)
	diagnostics, err := secrets.NewDiagnosticCredentialStore(store)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "darwin" {
		for _, account := range []string{sourceAccount, targetAccount} {
			requireUnoccupiedLegacyKeychainSlot(t, account, "")
			requireUnoccupiedLegacyKeychainSlot(t, "diagnostic@"+account, "")
		}
	}
	backend := secrets.BackendSelection{
		Kind:         "keyring",
		Availability: "available",
		Mutability:   "read_write",
		Persistence:  "explicit",
	}
	journey.runWithInput(journey.binary, token+"\n", "setup", "--from", journey.manifest, "--account", sourceAccount, "--token-stdin")
	journey.requireCredentialBackend(token, backend)
	journey.requireClaudeCredential(token)
	journey.runWithInput(journey.binary, replacement+"\n", "rotate", sourceAccount, "--token-stdin")
	journey.requireClaudeCredential(replacement)
	if runtime.GOOS == "darwin" {
		if exists, err := store.Exists(sourceAccount); err != nil || exists {
			t.Fatalf("published predecessor occupied the candidate native Token slot: exists=%t error=%v", exists, err)
		}
		configurationBeforeStaging := readFile(t, journey.config)
		stageNativeCandidateToken(t, journey, candidate, sourceAccount, replacement)
		if !bytes.Equal(configurationBeforeStaging, readFile(t, journey.config)) {
			t.Fatal("candidate credential staging changed retained configuration")
		}
	}
	if value, err := store.Get(sourceAccount); err != nil || value != replacement {
		t.Fatalf("candidate could not read the retained native Token slot: %v", err)
	}
	output := journey.runWithInput(candidate, diagnostic+"\n", "account", "diagnostics", "enable", sourceAccount, "--system-token-stdin", "--user-id", diagnosticID)
	if bytes.Contains(output, []byte(diagnostic)) || bytes.Contains(output, []byte(diagnosticID)) {
		t.Fatal("diagnostic credential input escaped into CLI output")
	}
	wantDiagnostic := secrets.DiagnosticCredential{SystemToken: diagnostic, UserID: diagnosticID}
	if got, err := diagnostics.Get(sourceAccount); err != nil || got != wantDiagnostic {
		t.Fatalf("candidate diagnostic credential was not staged from explicit input: %v", err)
	}
	journey.requireClaudeCredential(replacement)
	var predecessorReaders []process.Plan
	if runtime.GOOS == "darwin" {
		predecessorReaders = preprojectNativeCredentialJourney(t, journey, candidate, oldVersion, replacement)
	}
	journey.requireStoredCredentialAcrossUpdate(candidate, archive, checksums, newVersion, replacement, backend, predecessorReaders...)
	if got, err := diagnostics.Get(sourceAccount); err != nil || got != wantDiagnostic {
		t.Fatalf("candidate diagnostic credential did not survive update and rollback: %v", err)
	}
	activeToken := replacement
	if runtime.GOOS == "darwin" && oldVersion == "0.3.1" {
		bridge := publishedNativeJourney{journey: journey, candidate: candidate, archive: archive, checksums: checksums}
		activeToken = bridge.requireLegacyRotationRestaging(t, sourceAccount, oldVersion, replacement, store, backend)
	}
	finishNativeCredentialJourney(journey, store, sourceAccount, targetAccount, activeToken, wantDiagnostic, backend)
}

func (state *publishedNativeJourney) requireLegacyRotationRestaging(
	t *testing.T, account, predecessorVersion, oldToken string, store secrets.Store, backend secrets.BackendSelection,
) string {
	t.Helper()
	const rotated = "native-system-keyring-rotated-after-rollback"
	journey := state.journey
	journey.run("update", "--rollback")
	journey.requireVersion(predecessorVersion)
	configurationBefore := readFile(t, journey.config)
	journey.runWithInput(journey.binary, rotated+"\n", "rotate", account, "--token-stdin")
	if !bytes.Equal(configurationBefore, readFile(t, journey.config)) {
		t.Fatal("legacy Token rotation changed retained configuration")
	}
	if value, err := store.Get(account); err != nil || value != oldToken {
		t.Fatalf("legacy rotation silently changed the candidate Token slot: %v", err)
	}
	// The legacy and candidate Keychain slots are distinct. A rollback rotation
	// cannot make the candidate slot fresh without explicit input.
	journey.requireVersion(predecessorVersion)
	stageNativeCandidateToken(t, journey, state.candidate, account, rotated)
	if value, err := store.Get(account); err != nil || value != rotated {
		t.Fatalf("explicitly restaged candidate Token is unavailable: %v", err)
	}
	journey.runWith(state.candidate, "sync")
	if !bytes.Equal(configurationBefore, readFile(t, journey.config)) {
		t.Fatal("candidate reader preprojection changed retained configuration")
	}
	journey.run("update", "--candidate", state.archive, "--checksums", state.checksums)
	journey.run("sync")
	journey.requireCredentialBackend(rotated, backend)
	return rotated
}

func preprojectNativeCredentialJourney(t *testing.T, journey *journeyFixture, candidate, oldVersion, token string) []process.Plan {
	t.Helper()
	cfg, err := configuration.NewStore(journey.config).Load()
	if err != nil {
		t.Fatal(err)
	}
	clients := cfg.EnabledClientIDs()
	retained := make(map[string]process.Plan, len(clients))
	predecessorReaders := make([]process.Plan, 0, len(clients))
	for _, client := range clients {
		retained[client] = journey.retainedCredential(client)
		predecessorReaders = append(predecessorReaders, retained[client])
	}
	bridge := publishedNativeJourney{
		journey: journey, candidate: candidate, predecessor: readFile(t, journey.config),
		clients: clients, retained: retained,
	}
	bridge.preprojectForLinkGap(t, oldVersion, token)
	return predecessorReaders
}

func configureNativeDiagnosticProbe(t *testing.T, journey *journeyFixture, accountID, endpoint string) {
	t.Helper()
	manifest, err := configuration.Parse(readFile(t, journey.manifest))
	if err != nil {
		t.Fatal(err)
	}
	account := manifest.Accounts[accountID]
	account.AccountProbe = &configuration.AccountProbe{Kind: "dmxapi", BaseURL: endpoint}
	manifest.Accounts[accountID] = account
	data, err := toml.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(journey.manifest, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func stageNativeCandidateToken(t *testing.T, journey *journeyFixture, candidate, account, token string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	var stdout, stderr bytes.Buffer
	err := (process.Runner{}).RunStream(ctx, process.Plan{
		Executable: candidate, Args: []string{"rotate", account, "--token-stdin"},
		Env: journey.environment, Stdin: token + "\n",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("candidate Token staging failed: %v\nstdout:\n%s\nstderr:\n%s", err,
			redaction.Text(stdout.String(), token), redaction.Text(stderr.String(), token))
	}
}

func finishNativeCredentialJourney(
	journey *journeyFixture, store secrets.Store,
	sourceAccount, targetAccount, replacement string, wantDiagnostic secrets.DiagnosticCredential, backend secrets.BackendSelection,
) {
	t := journey.testing
	t.Helper()
	diagnostics, err := secrets.NewDiagnosticCredentialStore(store)
	if err != nil {
		t.Fatal(err)
	}
	journey.run("account", "rename", sourceAccount, targetAccount)
	for _, account := range []string{sourceAccount, targetAccount} {
		if value, err := store.Get(account); err != nil || value != replacement {
			t.Fatalf("renamed credential %q = %q, %v", account, value, err)
		}
		if got, err := diagnostics.Get(account); err != nil || got != wantDiagnostic {
			t.Fatalf("renamed diagnostic credential %q was not retained: %v", account, err)
		}
	}
	journey.requireClaudeCredential(replacement)
	journey.run("verify", "--for", "all")
	journey.run("account", "rename", sourceAccount, targetAccount, "--finalize")
	if exists, err := store.Exists(sourceAccount); err != nil || exists {
		t.Fatalf("finalized source credential remains: exists=%t error=%v", exists, err)
	}
	if exists, err := diagnostics.Exists(sourceAccount); err != nil || exists {
		t.Fatalf("finalized source diagnostic credential remains: exists=%t error=%v", exists, err)
	}
	if value, err := store.Get(targetAccount); err != nil || value != replacement {
		t.Fatalf("finalized target credential = %q, %v", value, err)
	}
	if got, err := diagnostics.Get(targetAccount); err != nil || got != wantDiagnostic {
		t.Fatalf("finalized target diagnostic credential was not retained: %v", err)
	}
	journey.uninstallAndRequireInstallationRemoved()
	if exists, err := store.Exists(targetAccount); err != nil || !exists {
		t.Fatalf("uninstall removed the retained credential: exists=%t error=%v", exists, err)
	}
	if got, err := diagnostics.Get(targetAccount); err != nil || got != wantDiagnostic {
		t.Fatalf("uninstall removed the retained diagnostic credential: %v", err)
	}
	journey.runWith(journey.source, "install", "--target", journey.binary)
	journey.run("sync")
	journey.requireNoClaudeProjection()
	journey.run("use", "--for", configuration.ClientClaude, "native-system-keyring-probe-claude")
	journey.requireCredentialBackend(replacement, backend)
	journey.requireClaudeCredential(replacement)
	journey.uninstallAndRequireInstallationRemoved()
	if err := store.Delete(targetAccount); err != nil {
		t.Fatal(err)
	}
	if err := diagnostics.Delete(targetAccount); err != nil {
		t.Fatal(err)
	}
	if exists, err := store.Exists(targetAccount); err != nil || exists {
		t.Fatalf("deleted credential remains: exists=%t error=%v", exists, err)
	}
	if exists, err := diagnostics.Exists(targetAccount); err != nil || exists {
		t.Fatalf("deleted diagnostic credential remains: exists=%t error=%v", exists, err)
	}
}

func requireUnoccupiedNativeCredentialSlots(t *testing.T, store secrets.Store, accounts ...string) {
	t.Helper()
	for _, account := range accounts {
		if exists, err := store.Exists(account); err != nil || exists {
			t.Fatalf("native credential test requires an unoccupied %q slot: exists=%t error=%v", account, exists, err)
		}
		t.Cleanup(func() {
			if err := store.Delete(account); err != nil {
				t.Errorf("clean system credential store %q: %v", account, err)
			}
			if exists, err := store.Exists(account); err != nil || exists {
				t.Errorf("system credential slot %q remains after cleanup: exists=%t error=%v", account, exists, err)
			}
		})
	}
}

func TestNativeCredentialSlotCleanupAfterEarlyExit(t *testing.T) {
	store := secrets.NewMemoryStore()
	t.Run("early exit", func(t *testing.T) {
		requireUnoccupiedNativeCredentialSlots(t, store, "owned")
		if err := store.Set("owned", "synthetic"); err != nil {
			t.Fatal(err)
		}
		t.SkipNow()
	})
	if exists, err := store.Exists("owned"); err != nil || exists {
		t.Fatalf("early exit left its native credential slot: exists=%t error=%v", exists, err)
	}
}

func TestSystemCredentialEnvironmentPreservesDarwinLoginHome(t *testing.T) {
	temporaryHome := filepath.Join(t.TempDir(), "isolated-home")
	hostHome := "/Users/runner"
	environment, err := systemCredentialEnvironment(
		"darwin",
		[]string{"HOME=" + temporaryHome, "USERPROFILE=" + temporaryHome, "AIGW_SECRET_BACKEND=env"},
		hostHome,
		"ephemeral-host",
	)
	if err != nil {
		t.Fatal(err)
	}
	values := environmentValues(environment)
	if values["HOME"] != hostHome || values["USERPROFILE"] != hostHome {
		t.Fatalf("Darwin system credential environment = %#v, want login home %q", values, hostHome)
	}
	if _, present := values["AIGW_SECRET_BACKEND"]; present {
		t.Fatal("system credential environment retained the environment backend")
	}
}

func TestSystemCredentialEnvironmentRequiresEphemeralDarwinHost(t *testing.T) {
	for _, scope := range []string{"", "persistent-host"} {
		if _, err := systemCredentialEnvironment("darwin", []string{"HOME=/tmp/isolated"}, "/Users/runner", scope); err == nil {
			t.Fatalf("Darwin system credential environment admitted scope %q", scope)
		}
	}
	if _, err := systemCredentialEnvironment("darwin", nil, "", "ephemeral-host"); err == nil {
		t.Fatal("Darwin system credential environment admitted an empty login home")
	}
}

func TestSystemCredentialJourneyUsesItsEffectiveHome(t *testing.T) {
	hostHome := t.TempDir()
	t.Setenv("HOME", hostHome)
	t.Setenv("AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE", "ephemeral-host")
	root := t.TempDir()
	journey := &journeyFixture{
		testing:  t,
		config:   filepath.Join(root, "config", "aigw", "config.toml"),
		settings: filepath.Join(root, "home", ".claude", "settings.json"),
		environment: []string{
			"HOME=" + filepath.Join(root, "home"),
			"USERPROFILE=" + filepath.Join(root, "home"),
			"XDG_CONFIG_HOME=" + filepath.Join(root, "config"),
			"XDG_DATA_HOME=" + filepath.Join(root, "data"),
			"APPDATA=" + filepath.Join(root, "config"),
			"LOCALAPPDATA=" + filepath.Join(root, "data"),
			"AIGW_SECRET_BACKEND=env",
		},
	}
	wantHome := filepath.Join(root, "home")
	wantConfig := filepath.Join(root, "config", "aigw", "config.toml")
	if runtime.GOOS == "darwin" {
		wantHome = hostHome
		wantConfig = filepath.Join(hostHome, "Library", "Application Support", "aigw", "config.toml")
	}
	for range 2 {
		t.Run("isolated native paths", func(t *testing.T) {
			journey.testing = t
			journey.enableSystemCredentialStore()
			if backend := environmentValues(journey.environment)["AIGW_SECRET_BACKEND"]; backend != "keyring" {
				t.Fatalf("native credential journey selected backend %q, want explicit keyring", backend)
			}
			if journey.settings != filepath.Join(wantHome, ".claude", "settings.json") || journey.config != wantConfig {
				t.Fatalf("credential journey paths = %q, %q; want effective home %q and config %q", journey.settings, journey.config, wantHome, wantConfig)
			}
			if runtime.GOOS == "darwin" {
				for _, path := range []string{journey.config, journey.settings} {
					if err := os.WriteFile(path, []byte("test-owned state"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
			}
		})
		if runtime.GOOS == "darwin" {
			for _, path := range []string{journey.config, journey.settings} {
				if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
					t.Fatalf("native test directory remains after cleanup: %s, %v", path, err)
				}
			}
		}
	}
}

func (j *journeyFixture) enableSystemCredentialStore() {
	j.testing.Helper()
	environment, err := systemCredentialEnvironment(runtime.GOOS, j.environment, os.Getenv("HOME"), os.Getenv("AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE"))
	if err != nil {
		j.testing.Fatal(err)
	}
	j.environment = environment
	j.setEnvironment("AIGW_SECRET_BACKEND", "keyring")
	paths, err := platform.PathsFor(runtime.GOOS, environmentValues(environment))
	if err != nil {
		j.testing.Fatal(err)
	}
	j.config = filepath.FromSlash(paths.Config)
	j.settings = filepath.FromSlash(paths.ClaudeSettings)
	if runtime.GOOS == "darwin" {
		for _, directory := range []string{filepath.Dir(j.config), filepath.Dir(j.settings)} {
			if err := os.MkdirAll(filepath.Dir(directory), 0o700); err != nil {
				j.testing.Fatal(err)
			}
			if err := os.Mkdir(directory, 0o700); err != nil {
				j.testing.Fatalf("native credential test requires an unoccupied directory %s: %v", directory, err)
			}
			j.testing.Cleanup(func() {
				if err := os.RemoveAll(directory); err != nil {
					j.testing.Errorf("clean owned credential test directory %s: %v", directory, err)
				}
			})
		}
	}
}

func systemCredentialEnvironment(goos string, current []string, hostHome, scope string) ([]string, error) {
	environment := environmentWithout(current, "AIGW_SECRET_BACKEND")
	if goos != "darwin" {
		return environment, nil
	}
	if scope != "ephemeral-host" {
		return nil, fmt.Errorf("Darwin system credential verification requires an explicitly ephemeral host")
	}
	if hostHome == "" {
		return nil, fmt.Errorf("Darwin system credential verification requires the login HOME")
	}
	return environmentWith(environment, map[string]string{
		"HOME":        hostHome,
		"USERPROFILE": hostHome,
		"CODEX_HOME":  filepath.Join(hostHome, ".codex"),
	}), nil
}

func environmentValues(environment []string) map[string]string {
	values := make(map[string]string, len(environment))
	for _, entry := range environment {
		key, value, present := strings.Cut(entry, "=")
		if present {
			values[key] = value
		}
	}
	return values
}

func (j *journeyFixture) requireStoredCredentialAcrossUpdate(candidate, archive, checksums, newVersion, token string, backend secrets.BackendSelection, prior ...process.Plan) {
	j.testing.Helper()
	oldVersion := j.predecessorVersion(newVersion)
	j.testing.Logf("credential baseline version=%s sha256=%x; candidate version=%s sha256=%x", oldVersion, sha256.Sum256(readFile(j.testing, j.source)), newVersion, sha256.Sum256(readFile(j.testing, candidate)))
	retained := append(prior, j.retainedCredentials()...)
	for _, step := range []struct {
		version string
		program string
		args    []string
	}{
		{newVersion, candidate, []string{"update", "--candidate", archive, "--checksums", checksums}},
		{oldVersion, j.source, []string{"update", "--rollback"}},
		{newVersion, candidate, []string{"update", "--candidate", archive, "--checksums", checksums}},
	} {
		configurationBefore := readFile(j.testing, j.config)
		j.run(step.args...)
		if !bytes.Equal(readFile(j.testing, j.config), configurationBefore) {
			j.testing.Fatal("credential lifecycle changed the retained client configuration")
		}
		for _, credential := range retained {
			j.requireCredential(credential, token)
		}
		j.run("sync")
		current := j.retainedCredentials()
		if step.version == newVersion {
			j.requireCurrentVersionedReaders(current, step.version)
		}
		retained = append(retained, current...)
		for _, credential := range retained {
			j.requireCredential(credential, token)
		}
		j.requireVersion(step.version)
		j.requireProgramBytes(step.program)
		if step.version == newVersion {
			j.requireCredentialBackend(token, backend)
		}
		j.requireClaudeCredential(token)
	}
	j.source = candidate
}

func (j *journeyFixture) requireCurrentVersionedReaders(current []process.Plan, version string) {
	j.testing.Helper()
	cfg, err := configuration.NewStore(j.config).Load()
	if err != nil {
		j.testing.Fatal(err)
	}
	clientIDs := cfg.EnabledClientIDs()
	reader := j.credentialEntrypoint()
	for index, credential := range current {
		command := credential.Executable + "\x00" + strings.Join(credential.Args, "\x00")
		if runtime.GOOS == "windows" && strings.HasSuffix(strings.ToLower(credential.Executable), ".cmd") {
			command += string(readFile(j.testing, credential.Executable))
		}
		if !strings.Contains(command, reader) {
			j.testing.Fatalf("synchronized client %q did not select the current versioned reader after %s", clientIDs[index], version)
		}
	}
}

func runLinuxSecureFileFallback(t *testing.T, candidate, endpoint string) {
	t.Helper()
	journey := newNativeJourney(t, candidate, endpoint, true)
	journey.enableSystemCredentialStore()
	journey.setEnvironment("AIGW_SECRET_BACKEND", "")
	journey.setEnvironment(
		"DBUS_SESSION_BUS_ADDRESS",
		"unix:path="+filepath.Join(journey.root, "missing-session-bus.sock"),
	)
	const token = "native-secure-file-token"
	journey.runWithInput(journey.binary, token+"\n", "setup", "--from", journey.manifest, "--account", "native-system-keyring-probe", "--token-stdin")
	journey.requireCredentialBackend(token, secrets.BackendSelection{
		Kind:         "file",
		Availability: "available",
		Mutability:   "read_write",
		Persistence:  "persisted",
	})
	journey.requireClaudeCredential(token)
	backend := filepath.Join(journey.root, "data", "aigw", "secrets", "backend")
	if got := strings.TrimSpace(string(readFile(t, backend))); got != "file" {
		t.Fatalf("persisted backend = %q, want file", got)
	}
	journey.uninstallAndRequireInstallationRemoved()
}
