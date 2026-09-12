package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
	"aigw-cli/internal/transaction"

	"github.com/pelletier/go-toml/v2"
)

func TestNativeLifecycleBaselineSelection(t *testing.T) {
	t.Run("source fixture", func(t *testing.T) {
		t.Setenv("AIGW_ACCEPTANCE_BASELINE", "")
		got, err := nativeLifecycleBaseline("/built/fixture")
		if err != nil || got != "/built/fixture" {
			t.Fatalf("source fixture = %q, %v", got, err)
		}
	})
	t.Run("explicit released binary", func(t *testing.T) {
		baseline := filepath.Join(t.TempDir(), executableName())
		if err := os.WriteFile(baseline, []byte("baseline"), 0o700); err != nil {
			t.Fatal(err)
		}
		t.Setenv("AIGW_ACCEPTANCE_BASELINE", baseline)
		got, err := nativeLifecycleBaseline("/built/fixture")
		if err != nil || got != baseline {
			t.Fatalf("released baseline = %q, %v", got, err)
		}
	})
	t.Run("unavailable release is not replaced by fixture", func(t *testing.T) {
		t.Setenv("AIGW_ACCEPTANCE_BASELINE", filepath.Join(t.TempDir(), "missing"))
		if _, err := nativeLifecycleBaseline("/built/fixture"); err == nil {
			t.Fatal("missing requested baseline was silently replaced")
		}
	})
	t.Run("directory is not a release executable", func(t *testing.T) {
		t.Setenv("AIGW_ACCEPTANCE_BASELINE", t.TempDir())
		if _, err := nativeLifecycleBaseline("/built/fixture"); err == nil {
			t.Fatal("directory accepted as a release executable")
		}
	})
}

func nativeLifecycleBaseline(fixture string) (string, error) {
	path := os.Getenv("AIGW_ACCEPTANCE_BASELINE")
	if path == "" {
		return fixture, nil
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve acceptance baseline: %w", err)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", fmt.Errorf("inspect acceptance baseline: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("acceptance baseline must be a regular executable file: %s", absolute)
	}
	return absolute, nil
}

func nativeReleaseCandidate(t *testing.T, root, version string) (program, archive, checksums string) {
	t.Helper()
	if directory := os.Getenv("AIGW_ACCEPTANCE_RELEASE"); directory != "" {
		baseName, archiveName := nativeArchiveNames(version)
		return filepath.Join(directory, baseName, executableName()), filepath.Join(directory, archiveName), filepath.Join(directory, "checksums.txt")
	}
	program = buildNativeProgram(t, root, version)
	archive, checksums = writeNativeArchive(t, program, version)
	return program, archive, checksums
}

func runNativeReleaseLifecycle(t *testing.T, root, oldArtifact, newVersion, endpoint string) {
	t.Helper()
	baseline, err := nativeLifecycleBaseline(oldArtifact)
	if err != nil {
		t.Fatal(err)
	}
	newArtifact, archive, checksums := nativeReleaseCandidate(t, root, newVersion)
	journey := newNativeJourney(t, baseline, endpoint, true)
	codexConfig, originalCodex := journey.prepareCodexLifecycle()
	userFiles := map[string][]byte{
		filepath.Join(journey.root, "home", ".codex", "sessions", "session.jsonl"): []byte("{\"owner\":\"user\",\"history\":\"unchanged\"}\n"),
		filepath.Join(filepath.Dir(journey.settings), "CLAUDE.md"):                 []byte("# User instructions\n\nPreserve this document.\n"),
		filepath.Join(filepath.Dir(journey.binary), "user-notes.txt"):              []byte("Keep neighboring user files.\n"),
	}
	journey.retainedProgramNames = []string{"user-notes.txt"}
	for path, content := range userFiles {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	requireUserFiles := func() {
		t.Helper()
		for path, want := range userFiles {
			if got := readFile(t, path); !bytes.Equal(got, want) {
				t.Fatalf("lifecycle changed user file %s: %q", path, got)
			}
		}
	}
	oldVersion := journey.predecessorVersion(newVersion)
	t.Logf("baseline version=%s sha256=%x; candidate version=%s sha256=%x", oldVersion, sha256.Sum256(readFile(t, baseline)), newVersion, sha256.Sum256(readFile(t, newArtifact)))
	journey.setEnvironment(secrets.EnvironmentKey("native-system-keyring-probe"), "native-journey-token")
	journey.run("setup", "--from", journey.manifest, "--account", "native-system-keyring-probe")
	before, err := configuration.NewStore(journey.config).Load()
	if err != nil {
		t.Fatal(err)
	}

	journey.requireVersion(oldVersion)
	journey.updateTo(archive, checksums, newVersion, newArtifact)
	journey.requireRepairPreservesUserSettings("", "user-dark")
	requireUserFiles()
	if output := journey.run("update", "--candidate", archive, "--checksums", checksums); !strings.Contains(string(output), "already matches the current program") {
		t.Fatalf("exact candidate was not a verified no-op: %s", output)
	}
	journey.requireInvalidSuccessorPreservesInstallation(newVersion)
	journey.run("adapter", "disable", configuration.ClientClaude)
	journey.run("adapter", "disable", configuration.ClientCodex)
	journey.requireUserTheme("user-dark")
	if got := readFile(t, codexConfig); string(got) != originalCodex {
		t.Fatalf("disable changed original Codex configuration: %q", got)
	}
	journey.run("update", "--rollback")
	journey.run("sync")
	journey.requireHealthyVersion(oldVersion)
	journey.requireProgramBytes(baseline)
	requireUserFiles()
	journey.updateTo(archive, checksums, newVersion, newArtifact)
	journey.requireRepairPreservesUserSettings("user-dark", "user-dark-after-upgrade")
	requireUserFiles()

	journey.runWith(newArtifact, "uninstall", "--target", journey.binary)
	journey.requireOwnedFilesAbsent()
	journey.requireUserTheme("user-dark-after-upgrade")
	if got := readFile(t, codexConfig); string(got) != originalCodex {
		t.Fatalf("uninstall changed original Codex configuration: %q", got)
	}
	for _, path := range []string{journey.settings + ".aigw-state.json", journey.config + ".verified.json", codexConfig + ".aigw-state.json", codexConfig + ".aigw-model-catalog.json"} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("uninstall retained owned state %s: %v", path, err)
		}
	}
	requireUserFiles()
	clear(before.Adapters)
	if retained, err := configuration.NewStore(journey.config).Load(); err != nil || !reflect.DeepEqual(retained, before) {
		t.Fatalf("capability configuration changed across program lifecycle\nwant: %#v\ngot: %#v\nerror: %v", before, retained, err)
	}
}

func (j *journeyFixture) prepareCodexLifecycle() (string, string) {
	j.testing.Helper()
	j.installClientFixture("codex")
	manifest, err := configuration.Parse(readFile(j.testing, j.manifest))
	if err != nil {
		j.testing.Fatal(err)
	}
	account := manifest.Accounts["native-system-keyring-probe"]
	account.Endpoints.OpenAIResponses = j.endpoint
	manifest.Accounts["native-system-keyring-probe"] = account
	manifest.Profiles["native-codex"] = configuration.Profile{Label: "Native Codex", Account: "native-system-keyring-probe", Client: configuration.ClientCodex, Model: "gpt-test"}
	manifest.RecommendedRoutes[configuration.ClientCodex] = "native-codex"
	manifestData, err := toml.Marshal(manifest)
	if err != nil {
		j.testing.Fatal(err)
	}
	if err := os.WriteFile(j.manifest, manifestData, 0o600); err != nil {
		j.testing.Fatal(err)
	}
	codexConfig := filepath.Join(j.root, "home", ".codex", "config.toml")
	originalCodex := "# User configuration\nmodel = 'user-selected'\nuser_preference = true\n"
	if err := os.MkdirAll(filepath.Dir(codexConfig), 0o700); err != nil {
		j.testing.Fatal(err)
	}
	if err := os.WriteFile(codexConfig, []byte(originalCodex), 0o600); err != nil {
		j.testing.Fatal(err)
	}
	return codexConfig, originalCodex
}

func (j *journeyFixture) requireUserTheme(theme string) {
	j.testing.Helper()
	var settings map[string]json.RawMessage
	if err := json.Unmarshal(readFile(j.testing, j.settings), &settings); err != nil {
		j.testing.Fatal(err)
	}
	var observed string
	if err := json.Unmarshal(settings["theme"], &observed); err != nil || len(settings) != 1 || observed != theme {
		j.testing.Fatalf("withdrawal must leave exactly the user-authored settings: %s", readFile(j.testing, j.settings))
	}
}

func (j *journeyFixture) requireRepairPreservesUserSettings(previousTheme, theme string) {
	j.testing.Helper()
	settings := map[string]json.RawMessage{}
	if err := json.Unmarshal(readFile(j.testing, j.settings), &settings); err != nil {
		j.testing.Fatal(err)
	}
	var observedTheme string
	if data, present := settings["theme"]; present {
		if err := json.Unmarshal(data, &observedTheme); err != nil {
			j.testing.Fatal(err)
		}
	}
	if observedTheme != previousTheme {
		j.testing.Fatalf("program update changed the user theme: got=%q want=%q", observedTheme, previousTheme)
	}
	themeData, err := json.Marshal(theme)
	if err != nil {
		j.testing.Fatal(err)
	}
	settings["theme"] = themeData
	userSettings, err := json.Marshal(settings)
	if err != nil {
		j.testing.Fatal(err)
	}
	if err := os.WriteFile(j.settings, userSettings, 0o600); err != nil {
		j.testing.Fatal(err)
	}
	j.run("repair")
	var repairedSettings map[string]json.RawMessage
	if err := json.Unmarshal(readFile(j.testing, j.settings), &repairedSettings); err != nil || !reflect.DeepEqual(repairedSettings, settings) {
		j.testing.Fatalf("repair changed the externally edited settings: got=%s want=%s error=%v", readFile(j.testing, j.settings), userSettings, err)
	}
	paths := []string{j.config, j.config + ".bak", j.config + ".verified.json", j.settings, j.settings + ".aigw-state.json"}
	codexConfig := filepath.Join(j.root, "home", ".codex", "config.toml")
	paths = append(paths, codexConfig, codexConfig+".aigw-state.json", codexConfig+".aigw-model-catalog.json")
	before := make(map[string]transaction.FileSnapshot, len(paths))
	for _, path := range paths {
		content, err := transaction.CaptureFileSnapshot(path)
		if err != nil {
			j.testing.Fatal(err)
		}
		before[path] = content
	}
	for range 2 {
		var result struct {
			ConfigurationAction string `json:"configuration_action"`
			NextAction          string `json:"next_action"`
		}
		if err := json.Unmarshal(j.run("repair", "--json"), &result); err != nil {
			j.testing.Fatal(err)
		}
		if result.ConfigurationAction != "already-converged" || result.NextAction != "aigw check" {
			j.testing.Fatalf("repeated repair is not terminal: %#v", result)
		}
		for _, path := range paths {
			after, err := transaction.CaptureFileSnapshot(path)
			if err != nil {
				j.testing.Fatal(err)
			}
			if !reflect.DeepEqual(after, before[path]) {
				j.testing.Fatalf("repeated repair changed %s", path)
			}
		}
	}
}

func (j *journeyFixture) requireInvalidSuccessorPreservesInstallation(currentVersion string) {
	j.testing.Helper()
	invalidProgram := filepath.Join(j.testing.TempDir(), executableName())
	if err := os.WriteFile(invalidProgram, []byte("not an executable\n"), 0o700); err != nil {
		j.testing.Fatal(err)
	}
	installedBefore := sha256.Sum256(readFile(j.testing, j.binary))
	for _, test := range []struct {
		version string
		problem string
	}{
		{currentVersion, "different program bytes"},
		{"999.0.0", "candidate program failed startup verification"},
	} {
		archive, checksums := writeNativeArchive(j.testing, invalidProgram, test.version)
		command := exec.CommandContext(j.testing.Context(), j.binary, "update", "--candidate", archive, "--checksums", checksums)
		command.Env = j.environment
		output, err := command.CombinedOutput()
		if err == nil || !strings.Contains(string(output), test.problem) {
			j.testing.Fatalf("invalid candidate %s must fail admission: %v\n%s", test.version, err, output)
		}
	}
	if installedAfter := sha256.Sum256(readFile(j.testing, j.binary)); installedAfter != installedBefore {
		j.testing.Fatalf("invalid successor changed installed program: %x -> %x", installedBefore, installedAfter)
	}
	j.requireInstalledProgramFiles()
}

func (j *journeyFixture) requireProgramBytes(expected string) {
	j.testing.Helper()
	want := sha256.Sum256(readFile(j.testing, expected))
	if got := sha256.Sum256(readFile(j.testing, j.binary)); got != want {
		j.testing.Fatalf("installed program digest = %x, want %x", got, want)
	}
}

func (j *journeyFixture) updateTo(archive, checksums, version, program string) {
	j.testing.Helper()
	j.run("update", "--candidate", archive, "--checksums", checksums)
	j.run("sync")
	j.requireHealthyVersion(version)
	j.requireCodexProjection()
	j.requireProgramBytes(program)
}

func (j *journeyFixture) requireStoredCredentialAcrossUpdate(root, newVersion, oldVersion, token string, backend secrets.BackendSelection) {
	j.testing.Helper()
	candidate, archive, checksums := nativeReleaseCandidate(j.testing, root, newVersion)
	j.testing.Logf("credential baseline version=%s sha256=%x; candidate version=%s sha256=%x", oldVersion, sha256.Sum256(readFile(j.testing, j.source)), newVersion, sha256.Sum256(readFile(j.testing, candidate)))
	for _, step := range []struct {
		version string
		program string
		args    []string
	}{
		{newVersion, candidate, []string{"update", "--candidate", archive, "--checksums", checksums}},
		{oldVersion, j.source, []string{"update", "--rollback"}},
		{newVersion, candidate, []string{"update", "--candidate", archive, "--checksums", checksums}},
	} {
		j.run("adapter", "disable", configuration.ClientClaude)
		j.run(step.args...)
		j.run("sync")
		j.requireVersion(step.version)
		j.requireProgramBytes(step.program)
		if step.version == newVersion {
			j.requireCredentialBackend(token, backend)
		}
		if got := j.claudeCredential(); got != token {
			j.testing.Fatalf("program %s lost native credential continuity: %q", step.version, got)
		}
	}
	j.source = candidate
}

func (j *journeyFixture) requireHealthyVersion(version string) {
	j.testing.Helper()
	j.requireVersion(version)
	if got := j.claudeCredential(); got != "native-journey-token" {
		j.testing.Fatalf("credential = %q", got)
	}
	j.requireInstalledProgramFiles()
	j.run("check")
}

func (j *journeyFixture) requireCodexProjection() {
	j.testing.Helper()
	var config struct {
		Model          string `toml:"model"`
		ModelProvider  string `toml:"model_provider"`
		UserPreference bool   `toml:"user_preference"`
		ModelProviders map[string]struct {
			BaseURL string `toml:"base_url"`
			Auth    struct {
				Command string   `toml:"command"`
				Args    []string `toml:"args"`
			} `toml:"auth"`
		} `toml:"model_providers"`
	}
	path := filepath.Join(j.root, "home", ".codex", "config.toml")
	if err := toml.Unmarshal(readFile(j.testing, path), &config); err != nil {
		j.testing.Fatal(err)
	}
	provider := config.ModelProviders[config.ModelProvider]
	if config.ModelProvider != "aigw" || config.Model != "gpt-test" || !config.UserPreference || provider.BaseURL != j.endpoint || provider.Auth.Command != j.binary {
		j.testing.Fatalf("Codex configuration differs from the selected product route: %#v", config)
	}
	if got := strings.TrimSpace(string(j.runWith(provider.Auth.Command, provider.Auth.Args...))); got != "native-journey-token" {
		j.testing.Fatalf("Codex projected credential helper returned %q", got)
	}
}

func (j *journeyFixture) predecessorVersion(candidate string) string {
	j.testing.Helper()
	output := strings.TrimSpace(string(j.run("--version")))
	version, ok := strings.CutPrefix(output, "aigw version ")
	if !ok || version == "" || version == candidate {
		j.testing.Fatalf("baseline must identify a version distinct from candidate %s: %q", candidate, output)
	}
	return version
}

func (j *journeyFixture) requireVersion(version string) {
	j.testing.Helper()
	output := strings.TrimSpace(string(j.run("--version")))
	if output != "aigw version "+version {
		j.testing.Fatalf("installed version = %q, want %q", output, version)
	}
}

func (j *journeyFixture) requireInstalledProgramFiles() {
	j.testing.Helper()
	entries, err := os.ReadDir(filepath.Dir(j.binary))
	if err != nil {
		j.testing.Fatal(err)
	}
	want := map[string]bool{filepath.Base(j.binary): true, ".aigw.previous": true}
	for _, name := range j.retainedProgramNames {
		want[name] = true
	}
	if runtime.GOOS == "windows" {
		delete(want, ".aigw.previous")
		want[".aigw.previous.exe"] = true
	}
	if len(entries) != len(want) {
		j.testing.Fatalf("installed program directory contains residue: %#v", entryNames(entries))
	}
	for _, entry := range entries {
		if !want[entry.Name()] {
			j.testing.Fatalf("installed program directory contains residue %q", entry.Name())
		}
	}
}

func writeNativeArchive(t *testing.T, binary, version string) (string, string) {
	t.Helper()
	directory := t.TempDir()
	baseName, archiveName := nativeArchiveNames(version)
	archive := filepath.Join(directory, archiveName)
	payload := readFile(t, binary)
	if runtime.GOOS == "windows" {
		writeZipArchive(t, archive, baseName+"/"+executableName(), payload)
	} else {
		writeTarGzipArchive(t, archive, baseName+"/"+executableName(), payload)
	}
	digest := sha256.Sum256(readFile(t, archive))
	checksums := filepath.Join(directory, "checksums.txt")
	if err := os.WriteFile(checksums, []byte(fmt.Sprintf("%x  %s\n", digest, archiveName)), 0o600); err != nil {
		t.Fatal(err)
	}
	return archive, checksums
}

func nativeArchiveNames(version string) (string, string) {
	baseName := fmt.Sprintf("aigw_%s_%s_%s", version, runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		return baseName, baseName + ".zip"
	}
	return baseName, baseName + ".tar.gz"
}

func writeTarGzipArchive(t *testing.T, path, member string, payload []byte) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := tarWriter.WriteHeader(&tar.Header{Name: member, Mode: 0o755, Size: int64(len(payload))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(payload); err != nil {
		t.Fatal(err)
	}
	closeArchive(t, tarWriter.Close, gzipWriter.Close, file.Close)
}

func writeZipArchive(t *testing.T, path, member string, payload []byte) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	header := &zip.FileHeader{Name: member, Method: zip.Deflate}
	header.SetMode(0o755)
	entry, err := archive.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write(payload); err != nil {
		t.Fatal(err)
	}
	closeArchive(t, archive.Close, file.Close)
}

func closeArchive(t *testing.T, closers ...func() error) {
	t.Helper()
	for _, close := range closers {
		if err := close(); err != nil {
			t.Fatal(err)
		}
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}
