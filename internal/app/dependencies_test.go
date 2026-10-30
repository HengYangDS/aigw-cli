package app

import (
	"context"
	"reflect"
	"testing"

	"aigw-cli/internal/client/codex"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
)

func TestDiscoverRequiresAndUsesTheBoundDiscoverer(t *testing.T) {
	if _, err := (Dependencies{}).Discover(); err == nil {
		t.Fatal("Discover() error = nil, want unavailable error")
	}

	want := discovery.Result{Executables: map[string]string{codex.ID: "/portable/codex"}}
	got, err := Dependencies{Discovery: fixedDiscoverer{result: want}}.Discover()
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if got.Executable(codex.ID) != want.Executable(codex.ID) {
		t.Fatalf("Discover().Executable(codex) = %q, want %q", got.Executable(codex.ID), want.Executable(codex.ID))
	}
}

func TestSynchronizerPreservesInvocationCapabilities(t *testing.T) {
	config := store.NewStore("configuration.toml", clients.Admission())
	secretStore := secret.NewMemoryStore()
	runner := fakeRunner{}
	discoverer := fakeDiscoverer{}

	dependencies := Dependencies{
		Config:    config,
		Secrets:   secretStore,
		Runner:    runner,
		Discovery: discoverer,
	}
	got := dependencies.Synchronizer()

	if !reflect.DeepEqual(got.Config, config) {
		t.Fatalf("Synchronizer().Config = %#v, want invocation config", got.Config)
	}
	if got.Secrets != secretStore {
		t.Fatalf("Synchronizer().Secrets = %#v, want invocation secrets", got.Secrets)
	}
	if got.Runner != runner {
		t.Fatalf("Synchronizer().Runner = %#v, want invocation runner", got.Runner)
	}
	if got.Discovery != discoverer {
		t.Fatalf("Synchronizer().Discovery = %#v, want invocation discovery", got.Discovery)
	}
}

type fakeRunner struct{}

func (fakeRunner) RunCapture(context.Context, process.Plan) ([]byte, error) { return nil, nil }

func (fakeRunner) RunCaptureStreams(context.Context, process.Plan) ([]byte, []byte, error) {
	return nil, nil, nil
}

type fakeDiscoverer struct{}

func (fakeDiscoverer) Discover() discovery.Result { return discovery.Result{} }

type fixedDiscoverer struct{ result discovery.Result }

func (d fixedDiscoverer) Discover() discovery.Result { return d.result }
