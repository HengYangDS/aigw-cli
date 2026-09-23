//go:build client_acceptance

package main

import (
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
	"aigw-cli/tools/release/readiness"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestNativeClientInputs(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "program")
	if err := os.WriteFile(file, []byte("program"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name      string
		path      string
		directory bool
		valid     bool
	}{
		{"missing", "", false, false},
		{"relative", "program", false, false},
		{"absent", filepath.Join(root, "absent"), false, false},
		{"directory as executable", root, false, false},
		{"explicit file", file, false, true},
		{"file as directory", file, true, false},
		{"explicit directory", root, true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("AIGW_ACCEPTANCE_CODEX", test.path)
			_, err := requiredClientInput("AIGW_ACCEPTANCE_CODEX", test.directory)
			if (err == nil) != test.valid {
				t.Fatalf("valid=%t, error=%v", test.valid, err)
			}
		})
	}
	t.Run("isolated client capabilities", func(t *testing.T) {
		journey := &journeyFixture{
			testing: t, root: root, manifest: filepath.Join(root, "team.toml"),
			endpoint: "http://127.0.0.1:1/v1",
		}
		team := readFile(t, filepath.Join("..", "..", "manifests", "team.toml"))
		manifest, err := configuration.Parse(team)
		if err != nil {
			t.Fatal(err)
		}
		journey.prepareNativeClient(configuration.ClientCodex, file, team)
		journey.requireNativePreferences(configuration.ClientCodex)
		prepared, err := configuration.Parse(readFile(t, journey.manifest))
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(prepared.Routes, manifest.Routes) || !reflect.DeepEqual(prepared.Recommendations, manifest.Recommendations) {
			t.Fatal("native preparation substituted its own route or model for the supplied recommendation")
		}
	})
}

func TestNativeClientFilePreservation(t *testing.T) {
	for _, test := range []struct {
		name string
		auth []byte
	}{
		{"absent authentication", nil},
		{"empty authentication", []byte{}},
		{"existing authentication", []byte("{\"owner\":\"user\"}\n")},
	} {
		t.Run(test.name, func(t *testing.T) {
			journey := journeyFixture{testing: t, root: t.TempDir()}
			home := filepath.Join(journey.root, "home", ".codex")
			if err := os.MkdirAll(home, 0o700); err != nil {
				t.Fatal(err)
			}
			if test.auth != nil {
				if err := os.WriteFile(filepath.Join(home, "auth.json"), test.auth, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			check := journey.preserveClientFiles(configuration.ClientCodex)
			if err := check(); err != nil {
				t.Fatal(err)
			}
			auth := filepath.Join(home, "auth.json")
			if err := os.WriteFile(auth, []byte("changed"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := check(); err == nil {
				t.Fatal("changed authentication was accepted")
			}
			if err := os.Remove(auth); err != nil {
				t.Fatal(err)
			}
			if err := check(); (err == nil) != (test.auth == nil) {
				t.Fatalf("authentication absence: initial=%q, error=%v", test.auth, err)
			}
			if err := os.Mkdir(auth, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := check(); err == nil {
				t.Fatal("a directory was accepted as an authentication file")
			}
		})
	}
}

func TestNativeClientJourney(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	version, err := readiness.ReadProductVersion(root)
	if err != nil {
		t.Fatal(err)
	}
	team := readFile(t, filepath.Join(root, "manifests", "team.toml"))
	manifest, err := configuration.Parse(team)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Recommendations) != len(configuration.AdmittedClientIDs()) {
		t.Fatal("team manifest must recommend one route for every admitted client")
	}
	candidate, archive, checksums := nativeReleaseCandidate(t, root, version)
	baseline := buildNativeProgram(t, root, "0.0.0")
	for _, path := range []string{candidate, archive, checksums} {
		t.Logf("artifact %s sha256=%x", filepath.Base(path), sha256.Sum256(readFile(t, path)))
	}
	plan := nativeClientJourneyPlan{
		version: version, team: team, manifest: manifest,
		baseline: baseline, candidate: candidate, archive: archive, checksums: checksums,
	}
	for _, client := range []string{configuration.ClientClaude, configuration.ClientCodex, configuration.ClientHermes} {
		t.Run(client, func(t *testing.T) { plan.run(t, client) })
	}
	t.Run("codex-general-routes", plan.runCodexGeneralRoutes)
}

type nativeClientJourneyPlan struct {
	version                                 string
	team                                    []byte
	manifest                                configuration.Manifest
	baseline, candidate, archive, checksums string
}

func (p nativeClientJourneyPlan) run(t *testing.T, client string) {
	t.Helper()
	executable, err := requiredClientInput("AIGW_ACCEPTANCE_"+strings.ToUpper(client), false)
	if err != nil {
		t.Fatal(err)
	}
	selection := p.manifest.Recommendations[client].Primary
	route := p.manifest.Routes[selection.Route]
	const token = "native-real-client-token"
	var completions atomic.Int64
	protocol := selection.Protocol
	if protocol == "" {
		spec, _ := configuration.ClientSpecFor(client)
		protocol = spec.EndpointProtocols[0]
	}
	requiredEffort := "high"
	if client == configuration.ClientHermes {
		requiredEffort = ""
	}
	handler := clientResponseHandler(protocol, map[string]*atomic.Int64{route.UpstreamModel: &completions}, token, requiredEffort)
	requests := map[string]int{}
	var requestsMu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requestsMu.Lock()
		requests[request.Method+" "+request.URL.Path]++
		requestsMu.Unlock()
		handler.ServeHTTP(response, request)
	}))
	t.Cleanup(server.Close)
	t.Cleanup(func() {
		if t.Failed() {
			requestsMu.Lock()
			defer requestsMu.Unlock()
			t.Logf("client request paths: %v", requests)
		}
	})
	journey := newNativeJourney(t, p.baseline, server.URL+"/v1", false)
	if client == configuration.ClientHermes {
		journey.run("update", "--candidate", p.archive, "--checksums", p.checksums)
		journey.requireVersion(p.version)
		journey.requireProgramBytes(p.candidate)
	}
	journey.prepareNativeClient(client, executable, p.team)
	journey.isolateNativeClientManifest(client)
	journey.setEnvironment(secrets.EnvironmentKey(route.Account), token)
	journey.run("setup", "--from", journey.manifest, "--account", route.Account)
	journey.enableNativeClient(client, executable)
	retainedCredential := journey.retainedCredential(client)
	before := journey.preserveClientFiles(client)
	steps := p.clientLifecycle(journey, client)
	for _, step := range steps {
		if !t.Run(step.name, func(t *testing.T) {
			journey.testing = t
			configurationBefore := readFile(t, journey.config)
			journey.run(step.args...)
			if !bytes.Equal(readFile(t, journey.config), configurationBefore) {
				t.Fatal("lifecycle operation changed the retained client configuration")
			}
			journey.requireVersion(step.version)
			journey.requireProgramBytes(step.program)
			journey.requireCredential(retainedCredential, token)
			count := completions.Load()
			journey.run("verify", "--for", client)
			journey.requireNativePreferences(client)
			if completions.Load() <= count {
				t.Fatal("client returned without an authenticated streaming request")
			}
			if err := before(); err != nil {
				t.Fatal(err)
			}
		}) {
			return
		}
	}
	journey.testing = t
	journey.verifyNativeConfigEditing(client, executable)
	const renamedAccount = "renamed-client-account"
	journey.setEnvironment(secrets.EnvironmentKey(renamedAccount), token)
	journey.run("account", "rename", route.Account, renamedAccount)
	count := completions.Load()
	journey.run("verify", "--for", "all")
	if completions.Load() <= count {
		t.Fatal("bulk verification did not invoke the enabled native client")
	}
	checkpoint, err := configuration.NewStore(journey.config).LoadVerifiedCheckpoint()
	if err != nil || len(checkpoint.Clients) != 1 || checkpoint.Clients[0] != client {
		t.Fatalf("single-client checkpoint = %v: %v", checkpoint.Clients, err)
	}
	journey.environment = environmentWithout(journey.environment, secrets.EnvironmentKey(route.Account))
	journey.run("account", "rename", route.Account, renamedAccount, "--finalize")
	var retirement struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(journey.run("account", "rename", route.Account, renamedAccount, "--finalize", "--dry-run", "--json"), &retirement); err != nil || retirement.Status != "already-finalized" {
		t.Fatalf("repeated retirement = %q: %v", retirement.Status, err)
	}
	journey.requireExternalCredentialClient(client, executable, renamedAccount, completions.Load)
	journey.runWith(p.candidate, "uninstall", "--target", journey.binary)
	journey.requireOwnedFilesAbsent()
	journey.requireNativePreferences(client)
	if err := before(); err != nil {
		t.Fatal(err)
	}
}

func (j *journeyFixture) isolateNativeClientManifest(client string) {
	j.testing.Helper()
	manifest, err := configuration.Parse(readFile(j.testing, j.manifest))
	if err != nil {
		j.testing.Fatal(err)
	}
	for recommendedClient := range manifest.Recommendations {
		if recommendedClient != client {
			delete(manifest.Recommendations, recommendedClient)
		}
	}
	team, err := toml.Marshal(manifest)
	if err != nil {
		j.testing.Fatal(err)
	}
	if err := os.WriteFile(j.manifest, team, 0o600); err != nil {
		j.testing.Fatal(err)
	}
}

func (p nativeClientJourneyPlan) clientLifecycle(journey *journeyFixture, client string) []struct {
	name, version, program string
	args                   []string
} {
	if client == configuration.ClientHermes {
		return []struct {
			name, version, program string
			args                   []string
		}{{"first-adoption", p.version, p.candidate, []string{"status", "--json"}}}
	}
	oldVersion := journey.predecessorVersion(p.version)
	return []struct {
		name, version, program string
		args                   []string
	}{
		{"baseline", oldVersion, journey.source, []string{"status", "--json"}},
		{"candidate", p.version, p.candidate, []string{"update", "--candidate", p.archive, "--checksums", p.checksums}},
		{"rollback", oldVersion, journey.source, []string{"update", "--rollback"}},
		{"re-upgrade", p.version, p.candidate, []string{"update", "--candidate", p.archive, "--checksums", p.checksums}},
	}
}

func (j *journeyFixture) verifyNativeConfigEditing(client, executable string) {
	j.testing.Helper()
	if client != configuration.ClientCodex {
		return
	}
	j.runWith(executable, "mcp", "add", "aigw-acceptance", "--", j.binary, "--version")
	before := j.runWith(executable, "mcp", "get", "aigw-acceptance", "--json")
	j.run("check")
	j.run("sync")
	if after := j.runWith(executable, "mcp", "get", "aigw-acceptance", "--json"); !bytes.Equal(before, after) {
		j.testing.Fatal("AIGW synchronization changed the native MCP configuration")
	}
	j.requireNativePreferences(client)
	j.runWith(executable, "mcp", "remove", "aigw-acceptance")
	j.run("check")
}

func (j *journeyFixture) prepareNativeClient(client, executable string, team []byte) {
	j.testing.Helper()
	home := filepath.Join(j.root, "home")
	j.environment = environmentWithout(j.environment, "CODEX_API_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_BASE_URL")
	j.setEnvironment("CODEX_HOME", filepath.Join(home, ".codex"))
	j.setEnvironment("CLAUDE_CONFIG_DIR", filepath.Join(home, ".claude"))
	j.setEnvironment("HERMES_HOME", filepath.Join(home, ".hermes"))
	preferences := map[string]string{
		// This journey owns client inference and projection, not marketplace synchronization.
		configuration.ClientCodex: `model_reasoning_effort = 'high'
model_context_window = 500000
model_auto_compact_token_limit = 450000
model_auto_compact_token_limit_scope = 'body_after_prefix'
[features]
plugins = false
[features.multi_agent_v2]
max_concurrent_threads_per_session = 16
`,
		configuration.ClientClaude: `{"effortLevel":"high","autoCompactWindow":180000}`,
		configuration.ClientHermes: "model:\n  temperature: 0.3\nproviders:\n  personal:\n    base_url: https://personal.test\nterminal:\n  backend: local\n",
	}
	path := map[string]string{
		configuration.ClientClaude: j.settings,
		configuration.ClientCodex:  filepath.Join(home, ".codex", "config.toml"),
		configuration.ClientHermes: filepath.Join(home, ".hermes", "config.yaml"),
	}[client]
	if path == "" {
		j.testing.Fatalf("unsupported native client %q", client)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		j.testing.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(preferences[client]), 0o600); err != nil {
		j.testing.Fatal(err)
	}
	manifest, err := configuration.Parse(team)
	if err != nil {
		j.testing.Fatal(err)
	}
	for name, account := range manifest.Accounts {
		account.Endpoints.OpenAIResponses = j.endpoint
		account.Endpoints.Anthropic = strings.TrimSuffix(j.endpoint, "/v1")
		if account.AccountProbe != nil {
			account.AccountProbe.BaseURL = j.endpoint
		}
		manifest.Accounts[name] = account
	}
	data, err := toml.Marshal(manifest)
	if err != nil {
		j.testing.Fatal(err)
	}
	if err := os.WriteFile(j.manifest, data, 0o600); err != nil {
		j.testing.Fatal(err)
	}
	j.testing.Logf("client=%s executable=%s sha256=%x", client, executable, sha256.Sum256(readFile(j.testing, executable)))
}

func (j *journeyFixture) enableNativeClient(client, executable string) {
	j.testing.Helper()
	path := strings.Join([]string{j.clientBin, os.Getenv("AIGW_ACCEPTANCE_CLIENT_PATH")}, string(os.PathListSeparator))
	j.setEnvironment("PATH", strings.TrimRight(path, string(os.PathListSeparator)))
	args := []string{"client", "enable", client, "--executable", executable}
	if client == configuration.ClientCodex {
		args = append(args, "--target", filepath.Join(j.root, "home", ".codex", "config.toml"))
	}
	j.run(args...)
	j.run("sync")
	j.testing.Logf("client %s version=%s", client, strings.TrimSpace(string(j.runWith(executable, "--version"))))
}

func (j *journeyFixture) requireNativePreferences(client string) {
	j.testing.Helper()
	switch client {
	case configuration.ClientCodex:
		var preferences struct {
			Effort   string `toml:"model_reasoning_effort"`
			Window   int    `toml:"model_context_window"`
			Compact  int    `toml:"model_auto_compact_token_limit"`
			Scope    string `toml:"model_auto_compact_token_limit_scope"`
			Features struct {
				Plugins *bool `toml:"plugins"`
			} `toml:"features"`
		}
		path := filepath.Join(j.root, "home", ".codex", "config.toml")
		if err := toml.Unmarshal(readFile(j.testing, path), &preferences); err != nil {
			j.testing.Fatal(err)
		}
		if preferences.Effort != "high" || preferences.Window != 500000 || preferences.Compact != 450000 || preferences.Scope != "body_after_prefix" || preferences.Features.Plugins == nil || *preferences.Features.Plugins {
			j.testing.Fatalf("Codex preferences changed: %+v", preferences)
		}
	case configuration.ClientClaude:
		var preferences struct {
			Effort  string `json:"effortLevel"`
			Compact int    `json:"autoCompactWindow"`
		}
		if err := json.Unmarshal(readFile(j.testing, j.settings), &preferences); err != nil {
			j.testing.Fatal(err)
		}
		if preferences.Effort != "high" || preferences.Compact != 180000 {
			j.testing.Fatalf("Claude preferences changed: %+v", preferences)
		}
	case configuration.ClientHermes:
		requireFileContains(
			j.testing,
			filepath.Join(environmentValues(j.environment)["HERMES_HOME"], "config.yaml"),
			"temperature: 0.3",
			"personal:",
			"backend: local",
		)
	default:
		j.testing.Fatalf("unsupported native client %q", client)
	}
}

func (j *journeyFixture) preserveClientFiles(client string) func() error {
	j.testing.Helper()
	home := filepath.Join(j.root, "home")
	path, content := filepath.Join(home, ".claude", "CLAUDE.md"), "# User instructions\n"
	switch client {
	case configuration.ClientCodex:
		path, content = filepath.Join(home, ".codex", "sessions", "user.jsonl"), "{\"owner\":\"user\"}\n"
	case configuration.ClientHermes:
		path, content = filepath.Join(home, ".hermes", "sessions", "user.jsonl"), "{\"owner\":\"user\"}\n"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		j.testing.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		j.testing.Fatal(err)
	}
	files := map[string][]byte{path: []byte(content)}
	if client == configuration.ClientCodex {
		auth := filepath.Join(home, ".codex", "auth.json")
		data, err := os.ReadFile(auth)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			j.testing.Fatal(err)
		}
		files[auth] = data // nil records absence; an empty file has non-nil bytes.
	}
	return func() error {
		for path, want := range files {
			got, err := os.ReadFile(path)
			if want == nil && errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil || want == nil || !bytes.Equal(got, want) {
				return errors.Join(fmt.Errorf("client lifecycle changed user file %s", path), err)
			}
		}
		return nil
	}
}
