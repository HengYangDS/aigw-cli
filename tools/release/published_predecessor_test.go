package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"aigw-cli/internal/configuration"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secrets"
	"aigw-cli/tools/release/readiness"

	"github.com/pelletier/go-toml/v2"
)

type publishedNativeJourney struct {
	journey                     *journeyFixture
	baseline, candidate         string
	archive, checksums, version string
	predecessor, sessionBytes   []byte
	session                     string
	clients                     []string
	retained                    map[string]process.Plan
}

func TestNativePublishedPredecessorJourney(t *testing.T) {
	baseline := os.Getenv("AIGW_ACCEPTANCE_BASELINE")
	if baseline == "" {
		t.Skip("published predecessor was not selected")
	}
	if !filepath.IsAbs(baseline) {
		t.Fatal("published predecessor must be an explicit absolute executable path")
	}
	baseline = requireNativeLifecycleBaseline(t, func() string {
		t.Fatal("published predecessor may not fall back to a source fixture")
		return ""
	})
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	version, err := readiness.ReadProductVersion(root)
	if err != nil {
		t.Fatal(err)
	}
	candidate, archive, checksums := nativeReleaseCandidate(t, root, version)
	server := newNativeJourneyServer(t)
	journey := publishedNativeJourney{
		journey:  newNativeJourney(t, baseline, server.URL+"/v1", true),
		baseline: baseline, candidate: candidate, archive: archive, checksums: checksums, version: version,
		clients: []string{configuration.ClientClaude, configuration.ClientCodex, configuration.ClientHermes},
	}
	predecessorVersion := journey.journey.predecessorVersion(version)
	journey.prepare(t, nativeCurrentSchemaManifest(server.URL+"/v1"))
	if runtime.GOOS == "darwin" {
		journey.preprojectForLinkGap(t, predecessorVersion)
	}
	journey.upgrade(t)
	journey.rollbackAndRecover(t, predecessorVersion)
	if runtime.GOOS == "darwin" && os.Getenv("AIGW_VERIFY_SYSTEM_KEYRING") == "1" {
		t.Run("published_keychain", func(t *testing.T) {
			runNativeCredentialJourney(t, root, baseline, server.URL+"/v1", version)
		})
	}
}

func nativeCurrentSchemaManifest(endpoint string) string {
	return fmt.Sprintf(`version = 7

[recommendations.claude.primary]
route = "native-system-keyring-probe-claude"

[accounts.native-system-keyring-probe]
label = "Native System Keyring Probe"

[accounts.native-system-keyring-probe.endpoints]
anthropic = %q

[models.claude-test]
label = "Claude Test"

[routes.native-system-keyring-probe-claude]
label = "Native System Keyring Probe Claude"
account = "native-system-keyring-probe"
model = "claude-test"
upstream_model = "claude-test"
interfaces = { anthropic = [] }
`, endpoint)
}

func (state *publishedNativeJourney) prepare(t *testing.T, manifest string) {
	journey := state.journey
	if err := os.WriteFile(journey.manifest, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	journey.prepareCodexLifecycle()
	journey.installClientFixture(configuration.ClientHermes)
	journey.setEnvironment("HERMES_HOME", filepath.Join(journey.root, "home", ".hermes"))
	configured, err := configuration.Parse(readFile(t, journey.manifest))
	if err != nil {
		t.Fatal(err)
	}
	configured.Recommendations[configuration.ClientHermes] = configuration.ClientRecommendation{
		Primary: configuration.ClientSelection{Route: "native-system-keyring-probe-claude", Protocol: configuration.ProtocolAnthropic},
	}
	data, err := toml.Marshal(configured)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(journey.manifest, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(journey.settings), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(journey.settings, []byte(`{"theme":"user-dark"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	session := filepath.Join(journey.root, "home", ".codex", "sessions", "session.jsonl")
	if err := os.MkdirAll(filepath.Dir(session), 0o700); err != nil {
		t.Fatal(err)
	}
	sessionBytes := []byte("{\"owner\":\"user\",\"history\":\"unchanged\"}\n")
	if err := os.WriteFile(session, sessionBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	journey.setEnvironment(secrets.EnvironmentKey("native-system-keyring-probe"), "native-journey-token")
	journey.run("setup", "--from", journey.manifest, "--account", "native-system-keyring-probe")
	predecessor := readFile(t, journey.config)
	if !bytes.HasPrefix(predecessor, []byte(fmt.Sprintf("version = %d\n", configuration.ConfigVersion))) {
		t.Fatal("published predecessor did not create its own schema")
	}
	journey.run("check")
	state.retained = make(map[string]process.Plan, len(state.clients))
	for _, client := range state.clients {
		state.retained[client] = journey.retainedCredential(client)
		journey.requireCredential(state.retained[client], "native-journey-token")
	}
	state.predecessor = predecessor
	state.session = session
	state.sessionBytes = sessionBytes
}

func (state *publishedNativeJourney) preprojectForLinkGap(t *testing.T, predecessorVersion string) {
	journey := state.journey
	journey.runWith(state.candidate, "sync")
	for _, client := range state.clients {
		predecessor := state.retained[client]
		successor := journey.retainedCredential(client)
		if successor.Executable == predecessor.Executable && strings.Join(successor.Args, "\x00") == strings.Join(predecessor.Args, "\x00") {
			t.Fatalf("candidate did not move the %s credential command before installation replacement", client)
		}
		journey.requireCredential(predecessor, "native-journey-token")
		journey.requireCredential(successor, "native-journey-token")
	}
	if !bytes.Equal(readFile(t, journey.config), state.predecessor) {
		t.Fatal("candidate preprojection changed the published configuration")
	}

	hidden := journey.binary + ".precutover"
	if err := os.Rename(journey.binary, hidden); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Rename(hidden, journey.binary); err != nil {
			t.Errorf("restore predecessor after simulated link gap: %v", err)
		}
	}()
	for _, client := range state.clients {
		journey.requireCredential(journey.retainedCredential(client), "native-journey-token")
		predecessor := state.retained[client]
		legacyMutable := predecessor.Executable == journey.binary || strings.Contains(strings.Join(predecessor.Args, "\x00"), journey.binary)
		if predecessorVersion == "0.3.1" && !legacyMutable {
			t.Fatalf("0.3.1 %s fixture did not expose the mutable credential command", client)
		}
		if !legacyMutable {
			journey.requireCredential(predecessor, "native-journey-token")
			continue
		}
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		output, err := (process.Runner{}).RunCapture(ctx, predecessor)
		cancel()
		if err == nil || bytes.Contains(output, []byte("native-journey-token")) {
			t.Fatalf("cached mutable %s credential command did not expose its bounded link-gap risk", client)
		}
	}
}

func (state *publishedNativeJourney) upgrade(t *testing.T) {
	journey := state.journey
	journey.run("update", "--candidate", state.archive, "--checksums", state.checksums)
	for _, client := range state.clients {
		journey.requireCredential(state.retained[client], "native-journey-token")
	}
	journey.requireVersion(state.version)
	journey.requireProgramBytes(state.candidate)
	var preview struct {
		Required    bool `json:"required"`
		FromVersion int  `json:"from_version"`
		ToVersion   int  `json:"to_version"`
	}
	if err := json.Unmarshal(journey.run("config", "migrate", "--dry-run", "--json"), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.Required || preview.FromVersion != configuration.ConfigVersion || preview.ToVersion != configuration.ConfigVersion {
		t.Fatalf("unchanged published configuration requires migration: %#v", preview)
	}
	if !bytes.Equal(readFile(t, journey.config), state.predecessor) {
		t.Fatal("current-schema migration preview changed published configuration")
	}
	journey.run("sync")
	if !bytes.Equal(readFile(t, journey.config), state.predecessor) {
		t.Fatal("unchanged synchronization rewrote published configuration")
	}
	journey.run("check")
	for _, client := range state.clients {
		journey.requireCredential(journey.retainedCredential(client), "native-journey-token")
	}
}

func (state *publishedNativeJourney) rollbackAndRecover(t *testing.T, predecessorVersion string) {
	journey := state.journey
	successor := make(map[string]process.Plan, len(state.clients))
	for _, client := range state.clients {
		successor[client] = journey.retainedCredential(client)
	}
	journey.run("update", "--rollback")
	journey.requireVersion(predecessorVersion)
	journey.requireProgramBytes(state.baseline)
	for _, client := range state.clients {
		journey.requireCredential(state.retained[client], "native-journey-token")
		journey.requireCredential(successor[client], "native-journey-token")
	}
	if !bytes.Equal(readFile(t, journey.config), state.predecessor) {
		t.Fatal("current-schema rollback changed published configuration")
	}
	journey.run("sync")
	journey.run("check")
	for _, client := range state.clients {
		journey.requireCredential(journey.retainedCredential(client), "native-journey-token")
	}

	journey.run("update", "--candidate", state.archive, "--checksums", state.checksums)
	for _, client := range state.clients {
		journey.requireCredential(state.retained[client], "native-journey-token")
		journey.requireCredential(successor[client], "native-journey-token")
	}
	journey.run("sync")
	journey.run("check")
	journey.requireVersion(state.version)
	journey.requireProgramBytes(state.candidate)
	if !bytes.Equal(readFile(t, journey.config), state.predecessor) {
		t.Fatal("current-schema recovery rewrote published configuration")
	}
	state.finish(t)
}

func (state *publishedNativeJourney) finish(t *testing.T) {
	journey := state.journey
	candidate, session, sessionBytes := state.candidate, state.session, state.sessionBytes
	if !bytes.Equal(readFile(t, session), sessionBytes) {
		t.Fatal("published predecessor lifecycle changed user session history")
	}
	var settings map[string]json.RawMessage
	if err := json.Unmarshal(readFile(t, journey.settings), &settings); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(settings["theme"]), "user-dark") {
		t.Fatal("published predecessor lifecycle changed user settings")
	}
	journey.uninstallWithAndRequireInstallationRemoved(candidate)
	if !bytes.Equal(readFile(t, session), sessionBytes) {
		t.Fatal("published predecessor uninstall changed user session history")
	}
}
