package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"aigw-cli/internal/client/claudedesktop"
	"aigw-cli/internal/client/codex"
	"aigw-cli/internal/client/hermes"
	clientverification "aigw-cli/internal/client/verification"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
	"aigw-cli/tools/release/readiness"

	builtinclients "aigw-cli/internal/clients"

	"github.com/pelletier/go-toml/v2"
)

type publishedNativeJourney struct {
	journey                            *journeyFixture
	baseline, candidate                string
	archive, checksums, version        string
	predecessor, sessionBytes, catalog []byte
	session                            string
	clients                            []string
	retained                           map[string]process.Plan
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
		clients: deferredJourneyClientIDs(),
	}
	predecessorVersion := journey.journey.predecessorVersion(version)
	journey.prepare(t, nativeCurrentSchemaManifest(server.URL+"/v1"))
	if runtime.GOOS == "darwin" {
		journey.preprojectForLinkGap(t, predecessorVersion, "native-journey-token")
	}
	journey.upgrade(t)
	journey.rollbackAndRecover(t, predecessorVersion)
	if os.Getenv("AIGW_VERIFY_SYSTEM_KEYRING") == "1" {
		t.Run("system credential store", func(t *testing.T) {
			runNativeCredentialJourney(t, root, baseline, server.URL+"/v1", version)
		})
	}
}

func TestNativePublishedPredecessorForwardingJourney(t *testing.T) {
	baseline := os.Getenv("AIGW_ACCEPTANCE_BASELINE")
	if baseline == "" {
		t.Skip("published predecessor was not selected")
	}
	if !filepath.IsAbs(baseline) {
		t.Fatal("forwarding qualification requires an absolute published predecessor path")
	}
	baseline = requireNativeLifecycleBaseline(t, func() string {
		t.Fatal("forwarding qualification cannot build a substitute predecessor")
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
	upstream := newNativeJourneyServer(t)
	requests := make(chan string, 1)
	forwarding := newNativeJourneyServer(t, func(request *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode forwarded model: %v", err)
			return
		}
		select {
		case requests <- request.Method + " " + request.URL.Path + " " + body.Model:
		default:
			t.Error("forwarding probe sent more than one request")
		}
	})
	journey := newNativeJourney(t, baseline, upstream.URL+"/v1", true)
	state := publishedNativeJourney{journey: journey, baseline: baseline, candidate: candidate, archive: archive, checksums: checksums, version: version, clients: deferredJourneyClientIDs()}
	state.prepare(t, nativeCurrentSchemaManifest(upstream.URL+"/v1"))
	journey.run("update", "--candidate", archive, "--checksums", checksums)
	configStore := store.NewStore(journey.config, builtinclients.Admission())
	before := capturePublishedSnapshot(t, configStore)
	endpoint := forwarding.URL + "/selected/v1"
	args := []string{"use", "native-codex", "--for", codex.ID, "--forwarding-endpoint", endpoint}
	preview := journey.run(append(slices.Clone(args), "--dry-run", "--json")...)
	if bytes.Contains(preview, []byte("native-journey-token")) || !bytes.Contains(preview, []byte(endpoint)) {
		t.Fatal("forwarding preview lost its destination or exposed credentials")
	}
	unchanged, err := configStore.CaptureSnapshot()
	if err != nil || !reflect.DeepEqual(before, unchanged) {
		t.Fatalf("native preview changed configuration or recovery state: %v", err)
	}
	if len(requests) != 0 {
		t.Fatal("forwarding preview sent an inference request")
	}
	journey.run(args...)
	cfg, err := configStore.Load()
	if err != nil || cfg.Clients[codex.ID].ForwardingEndpoint != endpoint || cfg.Accounts["native-system-keyring-probe"].Endpoints.OpenAIResponses != upstream.URL+"/v1" {
		t.Fatalf("forwarding did not preserve Account ownership: %v", err)
	}
	journey.run("check", "--for", codex.ID)
	var got string
	select {
	case got = <-requests:
	default:
	}
	if got != "POST /selected/v1/responses gpt-test" {
		t.Fatalf("forwarding request = %q, want POST /selected/v1/responses gpt-test", got)
	}
	for _, client := range state.clients {
		resolved, err := cfg.ResolveRuntime(client, "")
		if err != nil {
			t.Fatal(err)
		}
		original := process.Plan{Executable: baseline, Args: []string{"credential", client, resolved.CredentialProjectionFingerprint(client)}, Env: slices.Clone(journey.environment)}
		journey.requireCredential(original, "native-journey-token")
		journey.requireCredential(journey.retainedCredential(client), "native-journey-token")
	}
	selected := capturePublishedSnapshot(t, configStore)
	journey.run("sync")
	converged, err := configStore.CaptureSnapshot()
	if err != nil || !reflect.DeepEqual(selected, converged) {
		t.Fatalf("converged native sync rewrote forwarding state: %v", err)
	}
	state.rollbackForwarding(t, endpoint, selected, baseline)
	state.finish(t)
}

func capturePublishedSnapshot(t *testing.T, configStore store.Store) store.Snapshot {
	t.Helper()
	snapshot, err := configStore.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func (state *publishedNativeJourney) rollbackForwarding(t *testing.T, endpoint string, selected store.Snapshot, baseline string) {
	journey := state.journey
	configStore := store.NewStore(journey.config, builtinclients.Admission())
	journey.run("rollback", "--last-change")
	cfg, err := configStore.Load()
	if err != nil || cfg.Clients[codex.ID].ForwardingEndpoint != "" {
		t.Fatalf("configuration rollback retained forwarding: %v", err)
	}
	journey.run("rollback", "--last-change")
	restored, err := configStore.CaptureSnapshot()
	if err != nil || !restored.Config.Equal(selected.Config) || !restored.Forwarding.Equal(selected.Forwarding) {
		t.Fatalf("configuration forward restore lost exact binding bytes: %v", err)
	}
	journey.run("update", "--rollback")
	journey.requireProgramBytes(baseline)
	for _, client := range state.clients {
		journey.requireCredential(state.retained[client], "native-journey-token")
	}
	journey.runWith(state.candidate, "sync")
	journey.run("update", "--candidate", state.archive, "--checksums", state.checksums)
	journey.requireProgramBytes(state.candidate)
	cfg, err = configStore.Load()
	if err != nil || cfg.Clients[codex.ID].ForwardingEndpoint != endpoint {
		t.Fatalf("program rollback and forward restore lost forwarding: %v", err)
	}
	for _, client := range state.clients {
		journey.requireCredential(journey.retainedCredential(client), "native-journey-token")
	}
	journey.run("use", "native-codex", "--for", codex.ID, "--direct")
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
	if slices.Contains(state.clients, claudedesktop.ID) {
		journey.installClientFixture(claudedesktop.ID)
	}
	journey.installClientFixture(hermes.ID)
	journey.setEnvironment("HERMES_HOME", filepath.Join(journey.root, "home", ".hermes"))
	configured, err := store.Parse(readFile(t, journey.manifest), builtinclients.Admission())
	if err != nil {
		t.Fatal(err)
	}
	configured.Recommendations[hermes.ID] = domain.ClientRecommendation{
		Primary: domain.ClientSelection{Route: "native-system-keyring-probe-claude", Protocol: domain.ProtocolAnthropic},
	}
	if slices.Contains(state.clients, claudedesktop.ID) {
		configured.Recommendations[claudedesktop.ID] = domain.ClientRecommendation{
			Primary: domain.ClientSelection{Route: "native-system-keyring-probe-claude"},
		}
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
	journey.setEnvironment(secret.EnvironmentKey("native-system-keyring-probe"), "native-journey-token")
	journey.run("setup", "--from", journey.manifest, "--account", "native-system-keyring-probe")
	predecessor := readFile(t, journey.config)
	if !bytes.HasPrefix(predecessor, []byte(fmt.Sprintf("version = %d\n", domain.ConfigVersion))) {
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

func (state *publishedNativeJourney) preprojectForLinkGap(t *testing.T, predecessorVersion, token string) {
	journey := state.journey
	journey.runWith(state.candidate, "sync")
	for _, client := range state.clients {
		predecessor := state.retained[client]
		successor := journey.retainedCredential(client)
		if successor.Executable == predecessor.Executable && strings.Join(successor.Args, "\x00") == strings.Join(predecessor.Args, "\x00") {
			t.Fatalf("candidate did not move the %s credential command before installation replacement", client)
		}
		journey.requireCredential(predecessor, token)
		journey.requireCredential(successor, token)
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
		journey.requireCredential(journey.retainedCredential(client), token)
		predecessor := state.retained[client]
		legacyMutable := predecessor.Executable == journey.binary || strings.Contains(strings.Join(predecessor.Args, "\x00"), journey.binary)
		if predecessorVersion == "0.3.1" && !legacyMutable {
			t.Fatalf("0.3.1 %s fixture did not expose the mutable credential command", client)
		}
		if !legacyMutable {
			journey.requireCredential(predecessor, token)
			continue
		}
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		output, err := (process.Runner{}).RunCapture(ctx, predecessor)
		cancel()
		if err == nil || bytes.Contains(output, []byte(token)) {
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
	if preview.Required || preview.FromVersion != domain.ConfigVersion || preview.ToVersion != domain.ConfigVersion {
		t.Fatalf("unchanged published configuration requires migration: %#v", preview)
	}
	if !bytes.Equal(readFile(t, journey.config), state.predecessor) {
		t.Fatal("current-schema migration preview changed published configuration")
	}
	state.catalog = journey.importCatalogOrder()
	journey.run("sync")
	if !bytes.Equal(readFile(t, journey.config), state.predecessor) {
		t.Fatal("unchanged synchronization rewrote published configuration")
	}
	journey.run("check")
	for _, client := range state.clients {
		journey.requireCredential(state.retained[client], "native-journey-token")
		journey.requireCredential(journey.retainedCredential(client), "native-journey-token")
	}
}

func (j *journeyFixture) importCatalogOrder() []byte {
	j.testing.Helper()
	before := readFile(j.testing, j.config)
	ranked, err := store.Parse(readFile(j.testing, j.manifest), builtinclients.Admission())
	if err != nil {
		j.testing.Fatal(err)
	}
	account, model := ranked.Accounts["native-system-keyring-probe"], ranked.Models["claude-test"]
	account.Order, model.Order = 10, 20
	ranked.Accounts["native-system-keyring-probe"], ranked.Models["claude-test"] = account, model
	data, err := toml.Marshal(ranked)
	if err != nil {
		j.testing.Fatal(err)
	}
	if err := os.WriteFile(j.manifest, data, 0o600); err != nil {
		j.testing.Fatal(err)
	}
	j.run("config", "import", j.manifest, "--replace-account", "native-system-keyring-probe", "--replace-model", "claude-test")
	configStore := store.NewStore(j.config, builtinclients.Admission())
	updated := capturePublishedSnapshot(j.testing, configStore)
	selected, err := configStore.Load()
	if err != nil {
		j.testing.Fatal(err)
	}
	if selected.Accounts["native-system-keyring-probe"].Order != 10 || selected.Models["claude-test"].Order != 20 ||
		!updated.Catalog.Exists || !bytes.Equal(updated.Config.Data, before) {
		j.testing.Fatal("catalogue ranks were not isolated from published routing")
	}
	return updated.Catalog.Data
}

func (state *publishedNativeJourney) rollbackAndRecover(t *testing.T, predecessorVersion string) {
	journey := state.journey
	successor := make(map[string]process.Plan, len(state.clients))
	for _, client := range state.clients {
		successor[client] = journey.retainedCredential(client)
	}
	ctx, cancel := context.WithTimeout(t.Context(), clientverification.ProtocolTimeout)
	stdout, stderr, rollbackErr := (process.Runner{}).RunCaptureStreams(ctx, process.Plan{
		Executable: journey.binary, Args: []string{"update", "--rollback"}, Env: journey.environment,
	})
	cancel()
	if len(stderr) != 0 {
		t.Logf("Original public rollback stderr:\n%s", domain.Redact(string(stderr), journey.sensitiveInputs...))
	}
	if rollbackErr != nil {
		t.Fatalf("Original public rollback failed: %s\nstdout:\n%s\nstderr:\n%s",
			domain.Redact(rollbackErr.Error(), journey.sensitiveInputs...),
			domain.Redact(string(stdout), journey.sensitiveInputs...), domain.Redact(string(stderr), journey.sensitiveInputs...))
	}
	journey.requireVersion(predecessorVersion)
	journey.requireProgramBytes(state.baseline)
	for _, client := range state.clients {
		journey.requireCredential(state.retained[client], "native-journey-token")
		journey.requireCredential(successor[client], "native-journey-token")
	}
	if !bytes.Equal(readFile(t, journey.config), state.predecessor) ||
		!bytes.Equal(capturePublishedSnapshot(t, store.NewStore(journey.config, builtinclients.Admission())).Catalog.Data, state.catalog) {
		t.Fatal("current-schema rollback changed published routing or catalogue presentation")
	}
	journey.run("sync")
	journey.run("check")
	if !bytes.Equal(capturePublishedSnapshot(t, store.NewStore(journey.config, builtinclients.Admission())).Catalog.Data, state.catalog) {
		t.Fatal("published predecessor synchronization changed catalogue presentation")
	}
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
	if !bytes.Equal(readFile(t, journey.config), state.predecessor) ||
		!bytes.Equal(capturePublishedSnapshot(t, store.NewStore(journey.config, builtinclients.Admission())).Catalog.Data, state.catalog) {
		t.Fatal("current-schema recovery rewrote published routing or catalogue presentation")
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
