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
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
		if !reflect.DeepEqual(prepared.Profiles, manifest.Profiles) || !reflect.DeepEqual(prepared.RecommendedRoutes, manifest.RecommendedRoutes) {
			t.Fatal("native preparation substituted its own profile or model for the supplied recommendation")
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

func TestNativeClientStreamEnvelope(t *testing.T) {
	for _, client := range configuration.AdmittedClientIDs() {
		t.Run(client, func(t *testing.T) {
			var completions atomic.Int64
			path := map[string]string{"claude": "/v1/messages", "codex": "/v1/responses"}[client]
			body := map[string]string{
				"codex":  `{"model":"configured-model","stream":true,"reasoning":{"effort":"high"}}`,
				"claude": `{"model":"configured-model","stream":true,"output_config":{"effort":"high"}}`,
			}[client]
			var response *httptest.ResponseRecorder
			for _, test := range []struct {
				method, path, credential, body string
				status                         int
				completed                      int64
			}{
				{http.MethodPost, path, "", `{"stream":true}`, http.StatusUnauthorized, 0},
				{http.MethodPost, "/wrong", "synthetic", `{"stream":true}`, http.StatusNotFound, 0},
				{http.MethodGet, path, "synthetic", `{"stream":true}`, http.StatusMethodNotAllowed, 0},
				{http.MethodPost, path, "synthetic", `{"stream":false}`, http.StatusBadRequest, 0},
				{http.MethodPost, path, "synthetic", `{"stream":true}`, http.StatusBadRequest, 0},
				{http.MethodPost, path, "synthetic", strings.ReplaceAll(body, "high", "low"), http.StatusBadRequest, 0},
				{http.MethodPost, path, "synthetic", strings.ReplaceAll(body, "configured-model", "different-model"), http.StatusBadRequest, 0},
				{http.MethodPost, path, "synthetic", body, http.StatusOK, 1},
			} {
				request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
				request.Header.Set("Authorization", "Bearer "+test.credential)
				response = httptest.NewRecorder()
				clientResponseHandler(client, "configured-model", "synthetic", &completions).ServeHTTP(response, request)
				if response.Code != test.status || completions.Load() != test.completed {
					t.Fatalf("%s %s: status=%d completions=%d", test.method, test.path, response.Code, completions.Load())
				}
			}
			for _, data := range clientResponseEvents(client, "configured-model") {
				var event struct {
					Type string `json:"type"`
				}
				if err := json.Unmarshal([]byte(data), &event); err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(response.Body.String(), "event: "+event.Type+"\ndata: "+data+"\n\n") {
					t.Fatalf("missing named %s event: %s", event.Type, response.Body.String())
				}
			}
		})
	}
}

func TestNativeClientJourney(t *testing.T) {
	inputs := map[string]string{}
	for _, key := range []string{"AIGW_ACCEPTANCE_BASELINE", "AIGW_ACCEPTANCE_RELEASE", "AIGW_ACCEPTANCE_CODEX", "AIGW_ACCEPTANCE_CLAUDE"} {
		path, err := requiredClientInput(key, key == "AIGW_ACCEPTANCE_RELEASE")
		if err != nil {
			t.Fatal(err)
		}
		inputs[key] = path
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	version, err := readiness.ReadDeliveryVersion(root, os.Getenv("AIGW_LOCAL_DELIVERY") == "true")
	if err != nil {
		t.Fatal(err)
	}
	team := readFile(t, filepath.Join(root, "manifests", "team.toml"))
	manifest, err := configuration.Parse(team)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.RecommendedRoutes) != len(configuration.AdmittedClientIDs()) {
		t.Fatal("team manifest must recommend one profile for every admitted client")
	}
	candidate, archive, checksums := nativeReleaseCandidate(t, root, version)
	for _, path := range []string{candidate, archive, checksums} {
		t.Logf("artifact %s sha256=%x", filepath.Base(path), sha256.Sum256(readFile(t, path)))
	}
	for _, client := range configuration.AdmittedClientIDs() {
		profile := manifest.Profiles[manifest.RecommendedRoutes[client]]
		t.Run(client, func(t *testing.T) {
			const token = "native-real-client-token"
			var completions atomic.Int64
			server := httptest.NewServer(clientResponseHandler(client, profile.Model, token, &completions))
			t.Cleanup(server.Close)
			journey := newNativeJourney(t, inputs["AIGW_ACCEPTANCE_BASELINE"], server.URL+"/v1", false)
			executable := inputs["AIGW_ACCEPTANCE_"+strings.ToUpper(client)]
			journey.prepareNativeClient(client, executable, team)
			journey.setEnvironment(secrets.EnvironmentKey(profile.Account), token)
			journey.run("setup", "--from", journey.manifest, "--account", profile.Account)
			journey.enableNativeClient(client, executable)
			before := journey.preserveClientFiles(client)
			oldVersion := journey.predecessorVersion(version)
			for _, step := range []struct {
				name, version, program string
				args                   []string
			}{
				{"baseline", oldVersion, journey.source, []string{"status", "--json"}},
				{"candidate", version, candidate, []string{"update", "--candidate", archive, "--checksums", checksums}},
				{"rollback", oldVersion, journey.source, []string{"update", "--rollback"}},
				{"re-upgrade", version, candidate, []string{"update", "--candidate", archive, "--checksums", checksums}},
			} {
				if !t.Run(step.name, func(t *testing.T) {
					journey.testing = t
					configurationBefore := readFile(t, journey.config)
					journey.run(step.args...)
					if !bytes.Equal(readFile(t, journey.config), configurationBefore) {
						t.Fatal("lifecycle operation changed the retained client configuration")
					}
					journey.requireVersion(step.version)
					journey.requireProgramBytes(step.program)
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
			journey.run("account", "rename", profile.Account, renamedAccount)
			count := completions.Load()
			journey.run("verify", "--for", "all")
			if completions.Load() <= count {
				t.Fatal("bulk verification did not invoke the enabled native client")
			}
			checkpoint, err := configuration.NewStore(journey.config).LoadVerifiedCheckpoint()
			if err != nil || len(checkpoint.Clients) != 1 || checkpoint.Clients[0] != client {
				t.Fatalf("single-client checkpoint = %v: %v", checkpoint.Clients, err)
			}
			journey.environment = environmentWithout(journey.environment, secrets.EnvironmentKey(profile.Account))
			journey.run("account", "rename", profile.Account, renamedAccount, "--finalize")
			var retirement struct {
				Status string `json:"status"`
			}
			if err := json.Unmarshal(journey.run("account", "rename", profile.Account, renamedAccount, "--finalize", "--dry-run", "--json"), &retirement); err != nil || retirement.Status != "already-finalized" {
				t.Fatalf("repeated retirement = %q: %v", retirement.Status, err)
			}
			journey.requireExternalCredentialClient(client, executable, renamedAccount, completions.Load)
			journey.runWith(candidate, "uninstall", "--target", journey.binary)
			journey.requireOwnedFilesAbsent()
			journey.requireNativePreferences(client)
			if err := before(); err != nil {
				t.Fatal(err)
			}
		})
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
	}
	path := j.settings
	if client == configuration.ClientCodex {
		path = filepath.Join(home, ".codex", "config.toml")
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
	args := []string{"adapter", "enable", client, "--executable", executable}
	if client == configuration.ClientCodex {
		args = append(args, "--target", filepath.Join(j.root, "home", ".codex", "config.toml"))
	}
	j.run(args...)
	j.run("sync")
	j.testing.Logf("client %s version=%s", client, strings.TrimSpace(string(j.runWith(executable, "--version"))))
}

func (j *journeyFixture) requireNativePreferences(client string) {
	j.testing.Helper()
	if client == configuration.ClientCodex {
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
		return
	}
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
}

func (j *journeyFixture) preserveClientFiles(client string) func() error {
	j.testing.Helper()
	home := filepath.Join(j.root, "home")
	path, content := filepath.Join(home, ".claude", "CLAUDE.md"), "# User instructions\n"
	if client == configuration.ClientCodex {
		path, content = filepath.Join(home, ".codex", "sessions", "user.jsonl"), "{\"owner\":\"user\"}\n"
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

func clientResponseHandler(client, model, token string, completions *atomic.Int64) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/models", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(response, `{"data":[],"models":[],"has_more":false}`)
	})
	path := map[string]string{"claude": "/v1/messages", "codex": "/v1/responses"}[client]
	mux.HandleFunc("POST "+path, func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Model     string `json:"model"`
			Stream    bool   `json:"stream"`
			Reasoning struct {
				Effort string `json:"effort"`
			} `json:"reasoning"`
			OutputConfig struct {
				Effort string `json:"effort"`
			} `json:"output_config"`
		}
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil || !input.Stream || input.Model != model {
			http.Error(response, "configured model and stream required", http.StatusBadRequest)
			return
		}
		effort := input.Reasoning.Effort
		if client == configuration.ClientClaude {
			effort = input.OutputConfig.Effort
		}
		if effort != "high" {
			http.Error(response, "configured high effort required", http.StatusBadRequest)
			return
		}
		response.Header().Set("Content-Type", "text/event-stream")
		for _, data := range clientResponseEvents(client, model) {
			var event struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				http.Error(response, "invalid fixture event", http.StatusInternalServerError)
				return
			}
			if _, err := fmt.Fprintf(response, "event: %s\ndata: %s\n\n", event.Type, data); err != nil {
				return
			}
		}
		completions.Add(1)
	})
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer "+token && request.Header.Get("X-Api-Key") != token {
			http.Error(response, "credential mismatch", http.StatusUnauthorized)
			return
		}
		mux.ServeHTTP(response, request)
	})
}

func clientResponseEvents(client, model string) []string {
	if client == configuration.ClientClaude {
		return []string{
			fmt.Sprintf(`{"type":"message_start","message":{"id":"msg_fixture","type":"message","role":"assistant","model":%q,"content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":1,"output_tokens":0}}}`, model),
			`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"AIGW_OK"}}`,
			`{"type":"content_block_stop","index":0}`,
			`{"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":1}}`,
			`{"type":"message_stop"}`,
		}
	}
	return []string{
		`{"type":"response.created","response":{"id":"resp_fixture","object":"response","status":"in_progress","output":[]}}`,
		`{"type":"response.output_item.added","output_index":0,"item":{"id":"msg_fixture","type":"message","role":"assistant","status":"in_progress","content":[]}}`,
		`{"type":"response.content_part.added","item_id":"msg_fixture","output_index":0,"content_index":0,"part":{"type":"output_text","text":"","annotations":[]}}`,
		`{"type":"response.output_text.delta","item_id":"msg_fixture","output_index":0,"content_index":0,"delta":"AIGW_OK"}`,
		`{"type":"response.output_text.done","item_id":"msg_fixture","output_index":0,"content_index":0,"text":"AIGW_OK"}`,
		`{"type":"response.output_item.done","output_index":0,"item":{"id":"msg_fixture","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"AIGW_OK","annotations":[]}]}}`,
		`{"type":"response.completed","response":{"id":"resp_fixture","object":"response","status":"completed","output":[{"id":"msg_fixture","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"AIGW_OK","annotations":[]}]}],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
	}
}
