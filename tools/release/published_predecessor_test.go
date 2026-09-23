package main

import (
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
	"aigw-cli/tools/release/readiness"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type publishedNativeJourney struct {
	journey                     *journeyFixture
	baseline, candidate         string
	archive, checksums, version string
	predecessor, sessionBytes   []byte
	session                     string
}

func TestNativePublishedPredecessorJourney(t *testing.T) {
	baseline := os.Getenv("AIGW_ACCEPTANCE_BASELINE")
	if baseline == "" {
		t.Skip("published predecessor was not selected")
	}
	if !filepath.IsAbs(baseline) {
		t.Fatal("published predecessor must be an explicit absolute executable path")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	version, err := readiness.ReadProductVersion(root)
	if err != nil {
		t.Fatal(err)
	}
	candidate, archive, checksums := nativeReleaseCandidate(t, root, version)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"data":[]}`))
	}))
	t.Cleanup(server.Close)
	journey := publishedNativeJourney{
		journey:  newNativeJourney(t, baseline, server.URL+"/v1", true),
		baseline: baseline, candidate: candidate, archive: archive, checksums: checksums, version: version,
	}
	journey.prepare(t, server.URL+"/v1")
	journey.upgrade(t)
	journey.rollbackAndRecover(t)
}

func (state *publishedNativeJourney) prepare(t *testing.T, endpoint string) {
	journey := state.journey
	manifest := fmt.Sprintf(`version = 4

[recommended_routes]
claude = "native-system-keyring-probe-claude"

[accounts.native-system-keyring-probe]
label = "Native System Keyring Probe"

[accounts.native-system-keyring-probe.endpoints]
anthropic = %q

[profiles.native-system-keyring-probe-claude]
label = "Native System Keyring Probe Claude"
account = "native-system-keyring-probe"
client = "claude"
model = "claude-test"
`, endpoint)
	if err := os.WriteFile(journey.manifest, []byte(manifest), 0o600); err != nil {
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
	if !bytes.HasPrefix(predecessor, []byte("version = 3\n")) {
		t.Fatal("published predecessor did not create its own schema")
	}
	journey.run("check")
	journey.requireClaudeCredential("native-journey-token")
	state.predecessor = predecessor
	state.session = session
	state.sessionBytes = sessionBytes
}

func (state *publishedNativeJourney) upgrade(t *testing.T) {
	journey := state.journey
	candidate, archive, checksums, version := state.candidate, state.archive, state.checksums, state.version
	predecessor := state.predecessor
	journey.run("update", "--candidate", archive, "--checksums", checksums)
	journey.requireVersion(version)
	journey.requireProgramBytes(candidate)
	var preview struct {
		Required    bool `json:"required"`
		FromVersion int  `json:"from_version"`
		ToVersion   int  `json:"to_version"`
	}
	if err := json.Unmarshal(journey.run("config", "migrate", "--dry-run", "--json"), &preview); err != nil {
		t.Fatal(err)
	}
	if !preview.Required || preview.FromVersion != 3 || preview.ToVersion != configuration.ConfigVersion {
		t.Fatalf("published predecessor migration preview = %#v", preview)
	}
	if !bytes.Equal(readFile(t, journey.config), predecessor) {
		t.Fatal("published predecessor preview changed configuration")
	}
	journey.run("config", "migrate")
	if backup := readFile(t, journey.config+".bak"); !bytes.Equal(backup, predecessor) {
		t.Fatal("published predecessor rollback bytes were lost")
	}
	migrated, err := configuration.NewStore(journey.config).Load()
	if err != nil || migrated.Clients[configuration.ClientClaude].Route != "native-system-keyring-probe-claude" {
		t.Fatalf("published predecessor selection did not migrate: %v", err)
	}
	beforeSync := readFile(t, journey.config)
	journey.run("sync")
	if !bytes.Equal(readFile(t, journey.config), beforeSync) {
		t.Fatal("unchanged sync rewrote the migrated configuration")
	}
	if backup := readFile(t, journey.config+".bak"); !bytes.Equal(backup, predecessor) {
		t.Fatal("sync replaced the published predecessor rollback input")
	}
	journey.run("check")
	if backup := readFile(t, journey.config+".bak"); !bytes.Equal(backup, predecessor) {
		t.Fatal("check replaced the published predecessor rollback input")
	}
	journey.requireClaudeCredential("native-journey-token")
}

func (state *publishedNativeJourney) rollbackAndRecover(t *testing.T) {
	journey := state.journey
	baseline, candidate, archive, checksums, version := state.baseline, state.candidate, state.archive, state.checksums, state.version
	predecessor, session, sessionBytes := state.predecessor, state.session, state.sessionBytes
	journey.run("config", "migrate", "--rollback")
	if !bytes.Equal(readFile(t, journey.config), predecessor) {
		t.Fatal("published predecessor configuration was not restored exactly")
	}
	journey.run("update", "--rollback")
	journey.requireVersion("0.1.0")
	journey.requireProgramBytes(baseline)
	journey.run("check")

	journey.run("update", "--candidate", archive, "--checksums", checksums)
	journey.run("config", "migrate")
	journey.run("sync")
	journey.run("check")
	journey.requireVersion(version)
	journey.requireProgramBytes(candidate)
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
	journey.runWith(candidate, "uninstall", "--target", journey.binary)
	journey.requireOwnedFilesAbsent()
	if !bytes.Equal(readFile(t, session), sessionBytes) {
		t.Fatal("published predecessor uninstall changed user session history")
	}
}
