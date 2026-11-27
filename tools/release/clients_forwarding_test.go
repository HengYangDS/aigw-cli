//go:build client_acceptance

package main

import (
	clientverification "aigw-cli/internal/adapter/verification"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform/process"
	"aigw-cli/internal/store"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
)

// requireNativeForwarding sends client through a forwarding service in front
// of upstream (newForwardingService): the preview names the destination that
// the service's root composes with the provider's endpoint, leaks no Token
// and changes nothing, the client then sends its requests through the
// service, and a rollback of the change withdraws it while a retained session
// still completes. A forwarding service relays only OpenAI Responses, so the
// edit of a client on another protocol, Claude Code, is refused and changes
// nothing.
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
	forwarding := newForwardingService(j.testing, upstream, func(request *http.Request, relayed string) {
		if request.Method == http.MethodPost && relayed == path && streamed(request) {
			requests.Add(1)
		}
	})
	destination := forwarding.URL + "/http/" + strings.TrimPrefix(direct.Endpoint, "http://")
	j.run("use", client, j.modelName(direct.Model), "--protocol", direct.Protocol.Name())
	args := []string{"client", "edit", client, "--via", forwarding.URL}
	before := capturePublishedSnapshot(j.testing, configStore)
	if direct.Protocol != domain.ProtocolResponses {
		j.requireForwardingRefused(append(slices.Clone(args), "--json"), configStore, before)
		return
	}
	preview := j.run(append(slices.Clone(args), "--dry-run", "--json")...)
	if !bytes.Contains(preview, []byte(destination)) || bytes.Contains(preview, []byte(token)) || !reflect.DeepEqual(before, capturePublishedSnapshot(j.testing, configStore)) {
		j.testing.Fatal("native forwarding preview lost its destination, leaked credentials or mutated state")
	}
	j.run(args...)
	cfg, err = configStore.Load()
	if err != nil {
		j.testing.Fatal(err)
	}
	resolved, err := cfg.ResolveRuntime(client, domain.ModelRef{})
	expected := direct
	expected.Endpoint = destination
	if err != nil || !resolved.Equal(expected) {
		j.testing.Fatalf("native forwarding changed Account, model or protocol ownership: %v", err)
	}
	j.requireCredential(retained, token)
	j.run("sync")
	count := requests.Load()
	j.run("check", client, "--depth", "client")
	if requests.Load() <= count {
		j.testing.Fatal("native client did not load the selected forwarding destination")
	}
	// A retained session keeps the route it was created with where the native
	// client freezes it, as Hermes does for its CLI sessions; it must still
	// complete through its original credential reference.
	continueClient()
	j.run("config", "rollback", "--last-change")
	restored := capturePublishedSnapshot(j.testing, configStore)
	if !restored.Config.Equal(before.Config) {
		j.testing.Fatal("native forwarding rollback lost the original binding bytes")
	}
	j.requireCredential(retained, token)
	count = requests.Load()
	j.run("check", client, "--depth", "client")
	continueClient()
	if requests.Load() != count {
		j.testing.Fatal("native client retained the withdrawn forwarding destination")
	}
}

// requireForwardingRefused runs a forwarding edit that the product must
// refuse, and requires that the refusal names the clients that forward and
// that the configuration and its recovery state are unchanged.
func (j *journeyFixture) requireForwardingRefused(args []string, configStore store.Store, before store.Snapshot) {
	j.testing.Helper()
	ctx, cancel := context.WithTimeout(j.testing.Context(), clientverification.ProtocolTimeout)
	defer cancel()
	stdout, _, err := (process.Runner{}).RunCaptureStreams(ctx, process.Plan{Executable: j.binary, Args: args, Env: j.environment})
	if err == nil || !bytes.Contains(stdout, []byte("--via is for")) || !reflect.DeepEqual(before, capturePublishedSnapshot(j.testing, configStore)) {
		j.testing.Fatalf("a forwarding that the client cannot use was not refused, or its refusal changed state: %v\n%s", err, stdout)
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
