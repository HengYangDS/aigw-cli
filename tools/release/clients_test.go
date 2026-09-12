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

func requiredClientInput(key string, directory bool) (string, error) {
	path := os.Getenv(key)
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("%s requires an explicit absolute path", key)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("%s: %w", key, err)
	}
	if info.IsDir() != directory || (!directory && !info.Mode().IsRegular()) {
		return "", fmt.Errorf("%s has the wrong input kind", key)
	}
	return path, nil
}

func TestNativeClientStreamEnvelope(t *testing.T) {
	for _, client := range configuration.AdmittedClientIDs() {
		t.Run(client, func(t *testing.T) {
			var completions atomic.Int64
			path := map[string]string{"claude": "/v1/messages", "codex": "/v1/responses"}[client]
			request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"stream":true}`))
			request.Header.Set("Authorization", "Bearer synthetic")
			response := httptest.NewRecorder()
			clientResponseHandler(client, "synthetic", &completions).ServeHTTP(response, request)
			if response.Code != http.StatusOK || completions.Load() != 1 {
				t.Fatalf("status=%d completions=%d", response.Code, completions.Load())
			}
			for _, test := range []struct {
				method, path, credential, body string
				status                         int
			}{
				{http.MethodPost, path, "", `{"stream":true}`, http.StatusUnauthorized},
				{http.MethodPost, "/wrong", "synthetic", `{"stream":true}`, http.StatusNotFound},
				{http.MethodGet, path, "synthetic", `{"stream":true}`, http.StatusMethodNotAllowed},
				{http.MethodPost, path, "synthetic", `{"stream":false}`, http.StatusBadRequest},
			} {
				request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
				request.Header.Set("Authorization", "Bearer "+test.credential)
				response := httptest.NewRecorder()
				clientResponseHandler(client, "synthetic", &completions).ServeHTTP(response, request)
				if response.Code != test.status || completions.Load() != 1 {
					t.Fatalf("%s %s: status=%d completions=%d", test.method, test.path, response.Code, completions.Load())
				}
			}
			for _, data := range clientResponseEvents(client) {
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
	version, err := readiness.ReadProductVersion(root)
	if err != nil {
		t.Fatal(err)
	}
	candidate, archive, checksums := nativeReleaseCandidate(t, root, version)
	for _, path := range []string{candidate, archive, checksums} {
		t.Logf("artifact %s sha256=%x", filepath.Base(path), sha256.Sum256(readFile(t, path)))
	}
	for _, client := range configuration.AdmittedClientIDs() {
		t.Run(client, func(t *testing.T) {
			const token = "native-real-client-token"
			var completions atomic.Int64
			server := httptest.NewServer(clientResponseHandler(client, token, &completions))
			t.Cleanup(server.Close)
			journey := newNativeJourney(t, inputs["AIGW_ACCEPTANCE_BASELINE"], server.URL+"/v1", false)
			executable := inputs["AIGW_ACCEPTANCE_"+strings.ToUpper(client)]
			journey.prepareNativeClient(client, executable)
			journey.setEnvironment(secrets.EnvironmentKey("native-system-keyring-probe"), token)
			journey.run("setup", "--from", journey.manifest, "--account", "native-system-keyring-probe")
			journey.enableNativeClient(client, executable)
			before := journey.preserveClientFiles(client)
			oldVersion := journey.predecessorVersion(version)
			for _, step := range []struct {
				name, version, program string
				args                   []string
			}{
				{"baseline", oldVersion, journey.source, nil},
				{"candidate", version, candidate, []string{"update", "--candidate", archive, "--checksums", checksums}},
				{"rollback", oldVersion, journey.source, []string{"update", "--rollback"}},
				{"re-upgrade", version, candidate, []string{"update", "--candidate", archive, "--checksums", checksums}},
			} {
				if !t.Run(step.name, func(t *testing.T) {
					journey.testing = t
					if len(step.args) != 0 {
						journey.run("adapter", "disable", client)
						journey.run(step.args...)
						journey.enableNativeClient(client, executable)
					}
					journey.requireVersion(step.version)
					journey.requireProgramBytes(step.program)
					count := completions.Load()
					journey.run("verify", "--for", client)
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
			journey.runWith(candidate, "uninstall", "--target", journey.binary)
			journey.requireOwnedFilesAbsent()
			if err := before(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func (j *journeyFixture) prepareNativeClient(client, executable string) {
	j.testing.Helper()
	home := filepath.Join(j.root, "home")
	j.environment = environmentWithout(j.environment, "CODEX_API_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_BASE_URL")
	j.setEnvironment("CODEX_HOME", filepath.Join(home, ".codex"))
	j.setEnvironment("CLAUDE_CONFIG_DIR", filepath.Join(home, ".claude"))
	manifest, err := configuration.Parse(readFile(j.testing, j.manifest))
	if err != nil {
		j.testing.Fatal(err)
	}
	account := manifest.Accounts["native-system-keyring-probe"]
	account.Endpoints.OpenAIResponses = j.endpoint
	account.Endpoints.Anthropic = strings.TrimSuffix(j.endpoint, "/v1")
	manifest.Accounts["native-system-keyring-probe"] = account
	manifest.Profiles["native-client"] = configuration.Profile{
		Label: "Native client", Account: "native-system-keyring-probe", Client: client,
		Model: map[string]string{"codex": "gpt-5.6-sol", "claude": "claude-sonnet-5"}[client],
	}
	manifest.RecommendedRoutes = map[string]string{client: "native-client"}
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

func (j *journeyFixture) preserveClientFiles(client string) func() error {
	j.testing.Helper()
	files := map[string][]byte{}
	home := filepath.Join(j.root, "home")
	switch client {
	case configuration.ClientCodex:
		files[filepath.Join(home, ".codex", "sessions", "user.jsonl")] = []byte("{\"owner\":\"user\"}\n")
	case configuration.ClientClaude:
		files[filepath.Join(home, ".claude", "CLAUDE.md")] = []byte("# User instructions\n")
	}
	for path, data := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			j.testing.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			j.testing.Fatal(err)
		}
	}
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
			if err != nil {
				return fmt.Errorf("read preserved user file %s: %w", path, err)
			}
			if want == nil || !bytes.Equal(got, want) {
				return fmt.Errorf("client lifecycle changed user file %s", path)
			}
		}
		return nil
	}
}

func clientResponseHandler(client, token string, completions *atomic.Int64) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/models", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(response, `{"data":[],"models":[],"has_more":false}`)
	})
	path := map[string]string{"claude": "/v1/messages", "codex": "/v1/responses"}[client]
	mux.HandleFunc("POST "+path, func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Stream bool `json:"stream"`
		}
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil || !input.Stream {
			http.Error(response, "stream required", http.StatusBadRequest)
			return
		}
		response.Header().Set("Content-Type", "text/event-stream")
		for _, data := range clientResponseEvents(client) {
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
		defer func() { _ = request.Body.Close() }()
		if request.Header.Get("Authorization") != "Bearer "+token && request.Header.Get("X-Api-Key") != token {
			http.Error(response, "credential mismatch", http.StatusUnauthorized)
			return
		}
		mux.ServeHTTP(response, request)
	})
}

func clientResponseEvents(client string) []string {
	if client == configuration.ClientClaude {
		return []string{
			`{"type":"message_start","message":{"id":"msg_fixture","type":"message","role":"assistant","model":"claude-sonnet-5","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":1,"output_tokens":0}}}`,
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
