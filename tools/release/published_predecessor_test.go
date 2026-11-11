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

	"aigw-cli/internal/adapter/claudedesktop"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/hermes"
	clientverification "aigw-cli/internal/adapter/verification"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform/process"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
	"aigw-cli/tools/release/identity"

	builtinclients "aigw-cli/internal/clients"
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
	version, err := identity.ReadProductVersion(root)
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
	version, err := identity.ReadProductVersion(root)
	if err != nil {
		t.Fatal(err)
	}
	candidate, archive, checksums := nativeReleaseCandidate(t, root, version)
	upstream := newNativeJourneyServer(t)
	// A check at the model depth sends one request without a model and one
	// inference request, both through the forwarding endpoint.
	requests := make(chan string, 2)
	forwarding := newNativeJourneyServer(t, func(request *http.Request) {
		observed := request.Method + " " + request.URL.Path
		if request.Method == http.MethodPost {
			var body struct {
				Model string `json:"model"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Errorf("decode forwarded model: %v", err)
				return
			}
			observed += " " + body.Model
		}
		select {
		case requests <- observed:
		default:
			t.Error("forwarding probe sent more than two requests")
		}
	})
	journey := newNativeJourney(t, baseline, upstream.URL+"/v1", true)
	state := publishedNativeJourney{journey: journey, baseline: baseline, candidate: candidate, archive: archive, checksums: checksums, version: version, clients: deferredJourneyClientIDs()}
	state.prepare(t, nativeCurrentSchemaManifest(upstream.URL+"/v1"))
	journey.run(journey.lifecycle(journey.binary, "update", "--archive", archive, "--checksums", checksums)...)
	configStore := store.NewStore(journey.config, builtinclients.Admission())
	before := capturePublishedSnapshot(t, configStore)
	endpoint := forwarding.URL + "/selected/v1"
	args := []string{"client", "edit", codex.ID, "--via", endpoint}
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
	if err != nil || cfg.Clients[codex.ID].ForwardingEndpoint != endpoint || cfg.Providers["native-system-keyring-probe"].Endpoints.Responses != upstream.URL+"/v1" {
		t.Fatalf("forwarding did not preserve Account ownership: %v", err)
	}
	journey.run("check", codex.ID)
	// The check has returned, so every request that it sent is buffered.
	got := []string{}
	for range len(requests) {
		got = append(got, <-requests)
	}
	if want := []string{"GET /selected/v1/models", "POST /selected/v1/responses gpt-test"}; !slices.Equal(got, want) {
		t.Fatalf("forwarded requests = %q, want %q", got, want)
	}
	for _, client := range state.clients {
		resolved, err := cfg.ResolveRuntime(client, domain.ModelRef{})
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
	journey.run(journey.configRollback(journey.binary, "--last-change")...)
	cfg, err := configStore.Load()
	if err != nil || cfg.Clients[codex.ID].ForwardingEndpoint != "" {
		t.Fatalf("configuration rollback retained forwarding: %v", err)
	}
	journey.run(journey.configRollback(journey.binary, "--last-change")...)
	restored, err := configStore.CaptureSnapshot()
	if err != nil || !restored.Config.Equal(selected.Config) {
		t.Fatalf("configuration forward restore lost exact binding bytes: %v", err)
	}
	journey.run(journey.lifecycle(journey.binary, "rollback")...)
	journey.requireProgramBytes(baseline)
	for _, client := range state.clients {
		journey.requireCredential(state.retained[client], "native-journey-token")
	}
	journey.runWith(state.candidate, "sync")
	journey.run(journey.lifecycle(journey.binary, "update", "--archive", state.archive, "--checksums", state.checksums)...)
	journey.requireProgramBytes(state.candidate)
	cfg, err = configStore.Load()
	if err != nil || cfg.Clients[codex.ID].ForwardingEndpoint != endpoint {
		t.Fatalf("program rollback and forward restore lost forwarding: %v", err)
	}
	for _, client := range state.clients {
		journey.requireCredential(journey.retainedCredential(client), "native-journey-token")
	}
	journey.run("client", "edit", codex.ID, "--direct")
}

func nativeCurrentSchemaManifest(endpoint string) string {
	return fmt.Sprintf(`version = 8

[[providers]]
id = "native-system-keyring-probe"
label = "Native System Keyring Probe"
endpoints = {anthropic = %q}

[[providers.models]]
id = "claude-test"
family = "claude-test"
label = "Native System Keyring Probe Claude"
protocols = {anthropic = []}

[[families]]
id = "claude-test"
label = "Claude Test"

[recommendations.claude]
primary = {model = "native-system-keyring-probe/claude-test"}
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
	configured, err := store.ParseTeamFile(readFile(t, journey.manifest), builtinclients.Admission())
	if err != nil {
		t.Fatal(err)
	}
	configured.Recommendations[hermes.ID] = domain.ClientRecommendation{
		Primary: domain.ClientSelection{Model: probeModel, Protocol: domain.ProtocolAnthropic},
	}
	if slices.Contains(state.clients, claudedesktop.ID) {
		configured.Recommendations[claudedesktop.ID] = domain.ClientRecommendation{
			Primary: domain.ClientSelection{Model: probeModel},
		}
	}
	data, err := store.ExportTeamFile(configured)
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
	journey.run("setup", "--from", journey.manifest)
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
	journey.run(journey.lifecycle(journey.binary, "update", "--archive", state.archive, "--checksums", state.checksums)...)
	for _, client := range state.clients {
		journey.requireCredential(state.retained[client], "native-journey-token")
	}
	journey.requireVersion(state.version)
	journey.requireProgramBytes(state.candidate)
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

func (state *publishedNativeJourney) rollbackAndRecover(t *testing.T, predecessorVersion string) {
	journey := state.journey
	successor := make(map[string]process.Plan, len(state.clients))
	for _, client := range state.clients {
		successor[client] = journey.retainedCredential(client)
	}
	ctx, cancel := context.WithTimeout(t.Context(), clientverification.ProtocolTimeout)
	stdout, stderr, rollbackErr := (process.Runner{}).RunCaptureStreams(ctx, process.Plan{
		Executable: journey.binary, Args: journey.lifecycle(journey.binary, "rollback"), Env: journey.environment,
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
	if !bytes.Equal(readFile(t, journey.config), state.predecessor) {
		t.Fatal("current-schema rollback changed the published configuration")
	}
	journey.run("sync")
	journey.run("check")
	if !bytes.Equal(readFile(t, journey.config), state.predecessor) {
		t.Fatal("published predecessor synchronization changed the configuration")
	}
	for _, client := range state.clients {
		journey.requireCredential(journey.retainedCredential(client), "native-journey-token")
	}

	journey.run(journey.lifecycle(journey.binary, "update", "--archive", state.archive, "--checksums", state.checksums)...)
	for _, client := range state.clients {
		journey.requireCredential(state.retained[client], "native-journey-token")
		journey.requireCredential(successor[client], "native-journey-token")
	}
	journey.run("sync")
	journey.run("check")
	journey.requireVersion(state.version)
	journey.requireProgramBytes(state.candidate)
	if !bytes.Equal(readFile(t, journey.config), state.predecessor) {
		t.Fatal("current-schema recovery rewrote the published configuration")
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
