//go:build client_acceptance

package main

import (
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/store"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
)

// requireNativeForwarding points client at a forwarding endpoint in front of
// upstream: the preview leaks no Token and changes nothing, the client then
// sends its requests through the endpoint, and a rollback of the change
// withdraws it while a retained session still completes.
func (j *journeyFixture) requireNativeForwarding(client, token string, upstream *httptest.Server, continueClient func()) {
	j.testing.Helper()
	configStore := store.NewStore(j.config, clients.Admission())
	cfg, err := configStore.Load()
	if err != nil {
		j.testing.Fatal(err)
	}
	direct, err := cfg.ResolveRuntime(client, domain.ModelRef{})
	if err != nil {
		j.testing.Fatal(err)
	}
	retained := j.retainedCredential(client)
	var requests atomic.Int64
	path, _ := streamRequest(direct.Protocol, "")
	// Only the native client's streamed requests count; AIGW's own check of
	// the model depth sends its request without a stream.
	forwarding := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPost && request.URL.Path == path && streamed(request) {
			requests.Add(1)
		}
		upstream.Config.Handler.ServeHTTP(response, request)
	}))
	defer forwarding.Close()
	endpoint := forwarding.URL + strings.TrimPrefix(direct.Endpoint, upstream.URL)
	j.run("use", client, j.modelName(direct.Model), "--protocol", direct.Protocol.Name())
	args := []string{"client", "edit", client, "--via", endpoint}
	before := capturePublishedSnapshot(j.testing, configStore)
	preview := j.run(append(slices.Clone(args), "--dry-run", "--json")...)
	if !bytes.Contains(preview, []byte(endpoint)) || bytes.Contains(preview, []byte(token)) || !reflect.DeepEqual(before, capturePublishedSnapshot(j.testing, configStore)) {
		j.testing.Fatal("native forwarding preview leaked credentials or mutated state")
	}
	j.run(args...)
	cfg, err = configStore.Load()
	if err != nil {
		j.testing.Fatal(err)
	}
	resolved, err := cfg.ResolveRuntime(client, domain.ModelRef{})
	expected := direct
	expected.Endpoint = endpoint
	if err != nil || resolved != expected {
		j.testing.Fatalf("native forwarding changed Account, model or protocol ownership: %v", err)
	}
	j.requireCredential(retained, token)
	j.run("sync")
	count := requests.Load()
	j.run("check", client, "--depth", "client")
	if requests.Load() <= count {
		j.testing.Fatal("native client did not load the selected forwarding endpoint")
	}
	// A retained session keeps the route it was created with where the native
	// client freezes it, as Hermes does for its CLI sessions; it must still
	// complete through its original credential reference.
	continueClient()
	j.run(j.configRollback(j.binary, "--last-change")...)
	restored := capturePublishedSnapshot(j.testing, configStore)
	if !restored.Config.Equal(before.Config) {
		j.testing.Fatal("native forwarding rollback lost the original binding bytes")
	}
	j.requireCredential(retained, token)
	count = requests.Load()
	j.run("check", client, "--depth", "client")
	continueClient()
	if requests.Load() != count {
		j.testing.Fatal("native client retained the withdrawn forwarding endpoint")
	}
}

// streamed reports whether request asks for a streamed answer, as a native
// client's turn does and AIGW's own check of the model depth does not, and
// leaves its body for the handler that serves it.
func streamed(request *http.Request) bool {
	body, err := io.ReadAll(request.Body)
	request.Body = io.NopCloser(bytes.NewReader(body))
	var input clientInferenceRequest
	return err == nil && json.Unmarshal(body, &input) == nil && input.Stream
}
