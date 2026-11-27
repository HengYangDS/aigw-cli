package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

	"github.com/pelletier/go-toml/v2"
)

// publishedNativeJourney is a lifecycle from a published predecessor to the
// candidate. predecessor holds the configuration that the predecessor wrote,
// and edited the same configuration as the candidate reads it: the copy that
// the schema edits made of it, or its own bytes when the two programs share a
// schema.
type publishedNativeJourney struct {
	journey                           *journeyFixture
	baseline, candidate               string
	archive, checksums, version       string
	predecessor, edited, sessionBytes []byte
	session                           string
	clients                           []string
	retained                          map[string]process.Plan
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
	journey.prepare(t)
	if runtime.GOOS == "darwin" {
		journey.preprojectForLinkGap(t, "native-journey-token")
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
	// inference request, both through the forwarding service, which relays
	// them to the upstream.
	requests := make(chan string, 2)
	forwarding := newForwardingService(t, upstream, func(request *http.Request, relayed string) {
		observed := request.Method + " " + relayed
		if request.Method == http.MethodPost {
			body, err := io.ReadAll(request.Body)
			request.Body = io.NopCloser(bytes.NewReader(body))
			var decoded struct {
				Model string `json:"model"`
			}
			if err != nil || json.Unmarshal(body, &decoded) != nil {
				t.Errorf("decode forwarded model: %v", err)
				return
			}
			observed += " " + decoded.Model
		}
		select {
		case requests <- observed:
		default:
			t.Error("forwarding probe sent more than two requests")
		}
	})
	journey := newNativeJourney(t, baseline, upstream.URL+"/v1", true)
	state := publishedNativeJourney{journey: journey, baseline: baseline, candidate: candidate, archive: archive, checksums: checksums, version: version, clients: deferredJourneyClientIDs()}
	state.prepare(t)
	journey.run("self", "update", "--archive", archive, "--checksums", checksums)
	journey.crossSchema(journey.binary)
	configStore := store.NewStore(journey.config, builtinclients.Admission())
	before := capturePublishedSnapshot(t, configStore)
	forwardingRoot := forwarding.URL
	destination := forwardingRoot + "/http/" + strings.TrimPrefix(upstream.URL, "http://") + "/v1"
	args := []string{"client", "edit", codex.ID, "--via", forwardingRoot}
	preview := journey.run(append(slices.Clone(args), "--dry-run", "--json")...)
	if bytes.Contains(preview, []byte("native-journey-token")) || !bytes.Contains(preview, []byte(destination)) {
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
	if err != nil || cfg.Clients[codex.ID].ForwardingRoot != forwardingRoot || cfg.Providers["native-system-keyring-probe"].Endpoints.Responses != upstream.URL+"/v1" {
		t.Fatalf("forwarding did not preserve Account ownership: %v", err)
	}
	journey.run("check", codex.ID)
	// The check has returned, so every request that it sent is buffered.
	got := []string{}
	for range len(requests) {
		got = append(got, <-requests)
	}
	if want := []string{"GET /v1/models", "POST /v1/responses gpt-test"}; !slices.Equal(got, want) {
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
	state.rollbackForwarding(t, forwardingRoot, destination, selected, baseline)
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

func (state *publishedNativeJourney) rollbackForwarding(t *testing.T, forwardingRoot, destination string, selected store.Snapshot, baseline string) {
	journey := state.journey
	configStore := store.NewStore(journey.config, builtinclients.Admission())
	journey.run("config", "rollback", "--last-change")
	cfg, err := configStore.Load()
	if err != nil || cfg.Clients[codex.ID].ForwardingRoot != "" {
		t.Fatalf("configuration rollback retained forwarding: %v", err)
	}
	journey.run("config", "rollback", "--last-change")
	restored, err := configStore.CaptureSnapshot()
	if err != nil || !restored.Config.Equal(selected.Config) {
		t.Fatalf("configuration forward restore lost exact binding bytes: %v", err)
	}
	crossing := !journey.firstProgramReadsConfiguration()
	journey.prepareProgramRollback()
	journey.run("self", "rollback")
	journey.requireProgramBytes(baseline)
	for _, client := range state.clients {
		journey.requireCredential(state.retained[client], "native-journey-token")
	}
	if crossing {
		// Before the edits the candidate refuses the predecessor's schema,
		// so it cannot project ahead of the update. The predecessor then
		// forwards the client in its own form, to an endpoint, which gives
		// the fourth edit a forwarding to delete.
		journey.requireSchemaRefusal(state.candidate)
		journey.run("client", "edit", codex.ID, "--via", destination)
	} else {
		journey.runWith(state.candidate, "sync")
	}
	journey.run("self", "update", "--archive", state.archive, "--checksums", state.checksums)
	journey.requireProgramBytes(state.candidate)
	journey.crossSchema(journey.binary)
	if crossing {
		state.requireForwardingSetAgain(t, forwardingRoot, destination)
	} else if cfg, err = configStore.Load(); err != nil || cfg.Clients[codex.ID].ForwardingRoot != forwardingRoot {
		t.Fatalf("program rollback and forward restore lost forwarding: %v", err)
	}
	for _, client := range state.clients {
		journey.requireCredential(journey.retainedCredential(client), "native-journey-token")
	}
	journey.run("client", "edit", codex.ID, "--direct")
}

// requireForwardingSetAgain checks the fourth edit after the predecessor
// forwarded the client to destination: the copy kept before the edits holds
// that forwarding, the edits delete it, and `--via` sets it again from root,
// which composes the same destination.
func (state *publishedNativeJourney) requireForwardingSetAgain(t *testing.T, root, destination string) {
	t.Helper()
	journey := state.journey
	var kept struct {
		Clients map[string]struct {
			Forwarding struct {
				Endpoint string `toml:"endpoint"`
			} `toml:"forwarding"`
		} `toml:"clients"`
	}
	if err := toml.Unmarshal(journey.keptConfiguration, &kept); err != nil || kept.Clients[codex.ID].Forwarding.Endpoint != destination {
		t.Fatalf("the copy kept before the edits lacks the predecessor's forwarding: %v", err)
	}
	configStore := store.NewStore(journey.config, builtinclients.Admission())
	cfg, err := configStore.Load()
	if err != nil || cfg.Clients[codex.ID].ForwardingRoot != "" {
		t.Fatalf("the edits kept the predecessor's forwarding: %v", err)
	}
	journey.run("client", "edit", codex.ID, "--via", root)
	cfg, err = configStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := cfg.ResolveRuntime(codex.ID, domain.ModelRef{})
	if err != nil || cfg.Clients[codex.ID].ForwardingRoot != root || resolved.Endpoint != destination {
		t.Fatalf("--via did not set the forwarding again with the destination that the predecessor used: %v", err)
	}
}

// nativeProbeTeamFile is the team file of the published journeys in schema,
// the schema of the program that applies it.
func nativeProbeTeamFile(schema int, endpoint string) string {
	return fmt.Sprintf(`version = %d

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
`, schema, endpoint)
}

func (state *publishedNativeJourney) prepare(t *testing.T) {
	journey := state.journey
	if err := os.WriteFile(journey.manifest, []byte(nativeProbeTeamFile(journey.firstProgramSchema, journey.endpoint)), 0o600); err != nil {
		t.Fatal(err)
	}
	journey.prepareCodexLifecycle()
	if slices.Contains(state.clients, claudedesktop.ID) {
		journey.installClientFixture(claudedesktop.ID)
	}
	journey.installClientFixture(hermes.ID)
	journey.setEnvironment("HERMES_HOME", filepath.Join(journey.root, "home", ".hermes"))
	configured := journey.readTeamFile()
	configured.Recommendations[hermes.ID] = domain.ClientRecommendation{
		Primary: domain.ClientSelection{Model: probeModel, Protocol: domain.ProtocolAnthropic},
	}
	if slices.Contains(state.clients, claudedesktop.ID) {
		configured.Recommendations[claudedesktop.ID] = domain.ClientRecommendation{
			Primary: domain.ClientSelection{Model: probeModel},
		}
	}
	journey.writeTeamFile(journey.manifest, configured)
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
	if !bytes.HasPrefix(predecessor, []byte(fmt.Sprintf("version = %d\n", journey.firstProgramSchema))) {
		t.Fatal("published predecessor did not create its own schema")
	}
	t.Logf("predecessor schema %d; candidate schema %d", journey.firstProgramSchema, domain.ConfigVersion)
	journey.run("check")
	state.retained = make(map[string]process.Plan, len(state.clients))
	for _, client := range state.clients {
		state.retained[client] = journey.retainedCredential(client)
		journey.requireCredential(state.retained[client], "native-journey-token")
	}
	state.predecessor, state.edited = predecessor, predecessor
	state.session = session
	state.sessionBytes = sessionBytes
}

// preprojectForLinkGap lets the candidate project its credential command
// while the predecessor is still installed. A configuration of the schema
// that the candidate replaced is edited first, which the predecessor's
// credential commands still read.
func (state *publishedNativeJourney) preprojectForLinkGap(t *testing.T, token string) {
	journey := state.journey
	journey.crossSchema(state.candidate)
	state.edited = readFile(t, journey.config)
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
	if !bytes.Equal(readFile(t, journey.config), state.edited) {
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
		referencesLink := predecessor.Executable == journey.binary || strings.Contains(strings.Join(predecessor.Args, "\x00"), journey.binary)
		if !referencesLink {
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

// upgrade installs the candidate over the predecessor. A configuration of the
// schema that the candidate replaced is refused, keeps its bytes and is then
// edited, unless the preprojection edited it already, and the predecessor's
// credential commands return the same Token from the edited file.
func (state *publishedNativeJourney) upgrade(t *testing.T) {
	journey := state.journey
	journey.run("self", "update", "--archive", state.archive, "--checksums", state.checksums)
	for _, client := range state.clients {
		journey.requireCredential(state.retained[client], "native-journey-token")
	}
	journey.requireVersion(state.version)
	journey.requireProgramBytes(state.candidate)
	journey.crossSchema(journey.binary)
	state.edited = readFile(t, journey.config)
	for _, client := range state.clients {
		journey.requireCredential(state.retained[client], "native-journey-token")
	}
	journey.run("sync")
	if !bytes.Equal(readFile(t, journey.config), state.edited) {
		t.Fatal("unchanged synchronization rewrote published configuration")
	}
	// The rollback that follows runs over the windows that this program wrote
	// into Hermes's file; the predecessor's synchronization must accept them.
	journey.requireHermesModelWindows()
	journey.run("check")
	for _, client := range state.clients {
		journey.requireCredential(state.retained[client], "native-journey-token")
		journey.requireCredential(journey.retainedCredential(client), "native-journey-token")
	}
}

// rollbackAndRecover rolls the program back to the predecessor and updates it
// again. A rollback over the edited configuration refuses until the copy kept
// before the edits is back, and the update then repeats the refusal and the
// edits.
func (state *publishedNativeJourney) rollbackAndRecover(t *testing.T, predecessorVersion string) {
	journey := state.journey
	successor := make(map[string]process.Plan, len(state.clients))
	for _, client := range state.clients {
		successor[client] = journey.retainedCredential(client)
	}
	journey.prepareProgramRollback()
	ctx, cancel := context.WithTimeout(t.Context(), clientverification.ProtocolTimeout)
	stdout, stderr, rollbackErr := (process.Runner{}).RunCaptureStreams(ctx, process.Plan{
		Executable: journey.binary, Args: []string{"self", "rollback"}, Env: journey.environment,
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

	journey.run("self", "update", "--archive", state.archive, "--checksums", state.checksums)
	for _, client := range state.clients {
		journey.requireCredential(state.retained[client], "native-journey-token")
		journey.requireCredential(successor[client], "native-journey-token")
	}
	journey.crossSchema(journey.binary)
	journey.run("sync")
	journey.requireHermesModelWindows()
	journey.run("check")
	journey.requireVersion(state.version)
	journey.requireProgramBytes(state.candidate)
	if !bytes.Equal(readFile(t, journey.config), state.edited) {
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
