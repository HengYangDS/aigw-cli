package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"aigw-cli/internal/codex"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secrets"
	"aigw-cli/internal/transaction"

	"github.com/pelletier/go-toml/v2"
)

const forwardingAccount = "native-system-keyring-probe"
const forwardingRoute = "native-codex-forwarding"
const forwardingToken = "native-journey-token"
const forwardingComment = "# Preserve the operator's existing preferences.\n"

type publishedCodexForwardingJourney struct {
	journey                                 *journeyFixture
	baseline, candidate, archive, checksums string
	version, codexPath, hermesPath, session string
	forwarding                              string
	original                                configuration.Config
	main                                    []byte
	reader                                  [32]byte
	credentials                             map[string]process.Plan
	preserved                               map[string]transaction.FileSnapshot
	modes                                   map[string]os.FileMode
}

// The existing published-predecessor selector supplies exact release inputs.
// Client fixtures prove projection and reader execution, not client inference.
func runPublishedCodexForwarding(t *testing.T, baseline, candidate, archive, checksums, version string) {
	t.Helper()
	server := newNativeJourneyServer(t)
	state := publishedCodexForwardingJourney{
		journey:  newNativeJourney(t, baseline, server.URL+"/v1", false),
		baseline: baseline, candidate: candidate, archive: archive, checksums: checksums,
		version: version, forwarding: server.URL + "/codex/v1",
	}
	state.prepare(t)
	state.capture(t)
	state.requireState(t, state.journey.endpoint)
	state.runLifecycle(t)
}

func (state *publishedCodexForwardingJourney) prepare(t *testing.T) {
	t.Helper()
	j := state.journey
	j.requireVersion("0.3.1")
	mustWriteFile(t, j.manifest, []byte(publishedForwardingManifest(j.endpoint)), 0o600)
	home := filepath.Join(j.root, "home")
	codexHome, hermesHome := filepath.Join(home, ".codex"), filepath.Join(home, ".hermes")
	j.setEnvironment("CODEX_HOME", codexHome)
	j.setEnvironment("HERMES_HOME", hermesHome)
	j.setEnvironment(secrets.EnvironmentKey(forwardingAccount), forwardingToken)
	state.codexPath, state.hermesPath = filepath.Join(codexHome, "config.toml"), filepath.Join(hermesHome, "config.yaml")
	state.session = filepath.Join(codexHome, "sessions", "retained.jsonl")
	for _, directory := range []string{codexHome, hermesHome, filepath.Dir(state.session)} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	mustWriteFile(t, state.codexPath, []byte(forwardingComment+"model_reasoning_effort = 'max'\nuser_preference = true\n"), 0o600)
	mustWriteFile(t, state.hermesPath, []byte(forwardingComment+"model:\n  temperature: 0.3\nproviders:\n  personal:\n    base_url: https://personal.test\nterminal:\n  backend: local\n"), 0o600)
	mustWriteFile(t, state.session, []byte("{\"owner\":\"user\",\"history\":\"unchanged\"}\n"), 0o600)
	j.run("setup", "--from", j.manifest, "--account", forwardingAccount)
	state.activateClients(t)
}

func publishedForwardingManifest(endpoint string) string {
	return fmt.Sprintf(`version = 7

[recommendations.codex.primary]
route = 'native-codex-forwarding'

[recommendations.hermes.primary]
route = 'native-codex-forwarding'
protocol = 'openai_responses'

[accounts.native-system-keyring-probe]
label = 'Native System Keyring Probe'

[accounts.native-system-keyring-probe.endpoints]
openai_responses = %q

[models.gpt-test]
label = 'Native Test Model'

[routes.native-codex-forwarding]
label = 'Native Codex Forwarding'
account = 'native-system-keyring-probe'
model = 'gpt-test'
upstream_model = 'gpt-test'
interfaces = { openai_responses = ['text'] }
`, endpoint)
}

func (state *publishedCodexForwardingJourney) activateClients(t *testing.T) {
	t.Helper()
	j := state.journey
	store := configuration.NewStore(j.config)
	cfg, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	// Fixture initialization uses the complete writer before client activation.
	// The published executable remains the explicit credential reader.
	for _, client := range []string{configuration.ClientCodex, configuration.ClientHermes} {
		cfg.SetSelectedRoute(client, forwardingRoute)
		binding := cfg.Clients[client]
		binding.CredentialCommand = state.baseline
		cfg.Clients[client] = binding
	}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	for _, client := range []string{configuration.ClientCodex, configuration.ClientHermes} {
		j.installClientFixture(client)
		name := strings.Replace(executableName(), "aigw", client, 1)
		path := state.codexPath
		if client == configuration.ClientHermes {
			path = state.hermesPath
		}
		j.run("client", "enable", client, "--executable", filepath.Join(j.clientBin, name), "--target", path)
	}
	j.run("sync")
}

func (state *publishedCodexForwardingJourney) capture(t *testing.T) {
	t.Helper()
	j := state.journey
	cfg, err := configuration.NewStore(j.config).Load()
	if err != nil {
		t.Fatal(err)
	}
	state.original, state.main = cfg.Clone(), readFile(t, j.config)
	state.reader = sha256.Sum256(readFile(t, state.baseline))
	state.credentials = map[string]process.Plan{}
	for _, client := range []string{configuration.ClientCodex, configuration.ClientHermes} {
		state.credentials[client] = j.retainedCredential(client)
	}
	if state.credentials[configuration.ClientCodex].Executable != state.baseline {
		t.Fatal("Codex did not retain the immutable predecessor credential reader")
	}
	state.preserved = map[string]transaction.FileSnapshot{}
	for _, path := range []string{state.session, state.hermesPath, state.hermesPath + ".aigw-state.json"} {
		snapshot, err := transaction.CaptureFileSnapshot(path)
		if err != nil {
			t.Fatal(err)
		}
		state.preserved[path] = snapshot
	}
	state.modes = map[string]os.FileMode{}
	for _, path := range []string{j.config, state.codexPath, state.hermesPath} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		state.modes[path] = info.Mode().Perm()
	}
	t.Logf("immutable predecessor sha256=%x; candidate sha256=%x", state.reader, sha256.Sum256(readFile(t, state.candidate)))
}

func (state *publishedCodexForwardingJourney) requireState(t *testing.T, endpoint string) {
	t.Helper()
	j := state.journey
	current, err := configuration.NewStore(j.config).Load()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(readFile(t, j.config), state.main) || !reflect.DeepEqual(current.Accounts, state.original.Accounts) || !reflect.DeepEqual(current.Routes, state.original.Routes) || !reflect.DeepEqual(current.Clients[configuration.ClientHermes], state.original.Clients[configuration.ClientHermes]) {
		t.Fatal("forwarding changed predecessor-readable configuration, upstream or enabled Hermes")
	}
	if binding := current.Clients[configuration.ClientCodex]; !binding.Enabled || binding.CredentialCommand != state.baseline {
		t.Fatal("forwarding changed the enabled Codex credential owner")
	}
	selected, err := current.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil || selected.Endpoint != endpoint || selected.UpstreamEndpoint != j.endpoint {
		t.Fatalf("selected Codex endpoint=%q upstream=%q: %v", selected.Endpoint, selected.UpstreamEndpoint, err)
	}
	state.requireProjection(t, endpoint)
	for path, before := range state.preserved {
		after, err := transaction.CaptureFileSnapshot(path)
		if err != nil || !before.Equal(after) {
			t.Fatalf("forwarding changed preserved file %s: %v", path, err)
		}
	}
	for path, mode := range state.modes {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != mode {
			t.Fatalf("forwarding changed native configuration mode %s: %v", path, err)
		}
	}
	state.requireReaders(t)
}

func (state *publishedCodexForwardingJourney) requireProjection(t *testing.T, endpoint string) {
	t.Helper()
	var projection struct {
		Provider   string `toml:"model_provider"`
		Model      string `toml:"model"`
		Effort     string `toml:"model_reasoning_effort"`
		Preference bool   `toml:"user_preference"`
		Providers  map[string]struct {
			BaseURL string `toml:"base_url"`
		} `toml:"model_providers"`
	}
	data := readFile(t, state.codexPath)
	if err := toml.Unmarshal(data, &projection); err != nil {
		t.Fatal(err)
	}
	if projection.Provider != "aigw" || projection.Model != "gpt-test" || projection.Effort != "max" || !projection.Preference || projection.Providers[projection.Provider].BaseURL != endpoint || !bytes.Contains(data, []byte(forwardingComment)) {
		t.Fatal("Codex projection changed its provider, model, effort or user preferences")
	}
}

func (state *publishedCodexForwardingJourney) requireReaders(t *testing.T) {
	t.Helper()
	if sha256.Sum256(readFile(t, state.baseline)) != state.reader {
		t.Fatal("forwarding changed the immutable credential reader")
	}
	for client, retained := range state.credentials {
		current := state.journey.retainedCredential(client)
		if current.Executable != retained.Executable || !reflect.DeepEqual(current.Args, retained.Args) {
			t.Fatalf("forwarding changed %s credential command or arguments", client)
		}
		state.journey.requireCredential(retained, forwardingToken)
	}
}

func (state *publishedCodexForwardingJourney) requirePreview(t *testing.T) {
	t.Helper()
	j := state.journey
	before := state.snapshotTree(t)
	retained := j.environment
	j.environment = environmentWithout(j.environment, secrets.EnvironmentKey(forwardingAccount))
	defer func() { j.environment = retained }()
	data := j.run("use", "--for", "codex", forwardingRoute, "--forwarding-endpoint", state.forwarding, "--dry-run", "--json")
	var preview struct {
		DryRun           bool   `json:"dry_run"`
		Endpoint         string `json:"endpoint"`
		UpstreamEndpoint string `json:"upstream_endpoint"`
		Projections      []struct {
			Client string `json:"client"`
			Target string `json:"target"`
			Action string `json:"action"`
		} `json:"projections"`
	}
	if err := json.Unmarshal(data, &preview); err != nil {
		t.Fatal(err)
	}
	if !preview.DryRun || preview.Endpoint != state.forwarding || preview.UpstreamEndpoint != j.endpoint || len(preview.Projections) != 1 {
		t.Fatal("native preview did not identify exactly one Codex transition")
	}
	projection := preview.Projections[0]
	if projection.Client != configuration.ClientCodex || projection.Target != state.codexPath || projection.Action != string(codex.ProjectionActionUpdate) {
		t.Fatalf("native preview projection = %#v", projection)
	}
	if !reflect.DeepEqual(state.snapshotTree(t), before) {
		t.Fatal("credential-free native preview wrote fixture files or directories")
	}
}

func (state *publishedCodexForwardingJourney) snapshotTree(t *testing.T) map[string]transaction.FileSnapshot {
	t.Helper()
	files := map[string]transaction.FileSnapshot{}
	if err := filepath.WalkDir(state.journey.root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			files[path] = transaction.NewFileSnapshot(nil, info.Mode())
			return nil
		}
		files[path], err = transaction.CaptureFileSnapshot(path)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return files
}

func (state *publishedCodexForwardingJourney) upgrade(t *testing.T) {
	t.Helper()
	j := state.journey
	j.run("update", "--candidate", state.archive, "--checksums", state.checksums)
	j.requireVersion(state.version)
	j.requireProgramBytes(state.candidate)
}

func (state *publishedCodexForwardingJourney) runLifecycle(t *testing.T) {
	t.Helper()
	j := state.journey
	state.upgrade(t)
	state.requireState(t, j.endpoint)
	state.requirePreview(t)
	j.run("use", "--for", "codex", forwardingRoute, "--forwarding-endpoint", state.forwarding)
	state.requireState(t, state.forwarding)
	j.run("sync")
	state.requireState(t, state.forwarding)
	j.run("rollback", "--last-change")
	state.requireState(t, j.endpoint)
	j.run("use", "--for", "codex", forwardingRoute, "--forwarding-endpoint", state.forwarding)
	j.run("use", "--for", "codex", forwardingRoute, "--direct")
	state.requireState(t, j.endpoint)
	component := strings.TrimSuffix(j.config, filepath.Ext(j.config)) + ".forwarding.toml"
	if _, err := os.Stat(component); !os.IsNotExist(err) {
		t.Fatalf("direct selection retained an active forwarding component: %v", err)
	}
	j.run("update", "--rollback")
	j.requireVersion("0.3.1")
	j.requireProgramBytes(state.baseline)
	j.run("sync")
	state.requireState(t, j.endpoint)
	state.upgrade(t)
	j.run("use", "--for", "codex", forwardingRoute, "--forwarding-endpoint", state.forwarding)
	j.run("sync")
	state.requireState(t, state.forwarding)
	j.run("use", "--for", "codex", forwardingRoute, "--direct")
	j.runWith(state.candidate, "uninstall", "--target", j.binary)
	j.requireOwnedFilesAbsent()
	if after, err := transaction.CaptureFileSnapshot(state.session); err != nil || !state.preserved[state.session].Equal(after) {
		t.Fatalf("uninstall changed preserved session: %v", err)
	}
}
