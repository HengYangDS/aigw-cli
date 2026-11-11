package store

import (
	"aigw-cli/internal/domain"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestRetryBudgetsAreInlineInTheirClient keeps a client's retry budgets in
// its own table of aigw.toml, where an explicit zero survives a round trip.
func TestRetryBudgetsAreInlineInTheirClient(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "aigw.toml"), testAdmission())
	cfg := validConfig()
	zero, three := uint64(0), uint64(3)
	binding := cfg.Clients[ClientCodex]
	binding.RetryPolicy = &domain.RetryPolicy{RequestMaxRetries: &zero, StreamMaxRetries: &three}
	cfg.Clients[ClientCodex] = binding
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(store.Path())
	if err != nil || !bytes.Contains(data, []byte("[clients.codex]\nenabled = false\nmodel = 'backup/gpt-test'\nretries = { request = 0, stream = 3 }\n")) {
		t.Fatalf("retries are not inline in their client:\n%s\n%v", data, err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	policy := loaded.Clients[ClientCodex].RetryPolicy
	if policy == nil || *policy.RequestMaxRetries != 0 || *policy.StreamMaxRetries != 3 {
		t.Fatalf("retries after a round trip = %#v", policy)
	}
}

func TestConnectionRetryPolicyRoundTripsAndRestores(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "aigw.toml"), testAdmission())
	cfg := validConfig()
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	before, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	declared := bytes.Replace(before.Config.Data, []byte("model = 'backup/gpt-test'\n"), []byte("model = 'backup/gpt-test'\nretries = {request = 0, stream = 0}\n"), 1)
	selected, err := decodeTOMLConfig(declared, testAdmission())
	if err != nil {
		t.Fatalf("explicit native connection policy refused: %v", err)
	}
	after, err := store.Commit(before, selected)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := loaded.ResolveRuntime(ClientCodex, domain.ModelRef{})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(runtime)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(encoded, &document); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"request_max_retries": float64(0), "stream_max_retries": float64(0)}
	if !reflect.DeepEqual(document["retry_policy"], want) {
		t.Fatalf("resolved policy = %#v, want %#v", document["retry_policy"], want)
	}
	if err := store.RestoreSnapshot(before, after); err != nil {
		t.Fatal(err)
	}
	restored, err := store.CaptureSnapshot()
	if err != nil || !before.equal(restored) {
		t.Fatalf("policy rollback did not restore exact preimages: %v", err)
	}
}

func TestConnectionRetryPolicyCloneAndRuntimeDoNotAlias(t *testing.T) {
	cfg := validConfig()
	zero := uint64(0)
	binding := cfg.Clients[ClientCodex]
	binding.RetryPolicy = &domain.RetryPolicy{RequestMaxRetries: &zero}
	cfg.Clients[ClientCodex] = binding
	clone := cfg.Clone()
	*clone.Clients[ClientCodex].RetryPolicy.RequestMaxRetries = 2
	runtime, err := cfg.ResolveRuntime(ClientCodex, domain.ModelRef{})
	if err != nil {
		t.Fatal(err)
	}
	*runtime.RetryPolicy.RequestMaxRetries = 3
	if *cfg.Clients[ClientCodex].RetryPolicy.RequestMaxRetries != 0 {
		t.Fatal("immutable policy aliased its declaration")
	}
}

func TestConnectionRetryPolicyRejectsUnsupportedOrEmptyDeclarations(t *testing.T) {
	zero := uint64(0)
	for _, test := range []struct {
		client string
		policy *domain.RetryPolicy
	}{
		{ClientCodex, &domain.RetryPolicy{}},
		{ClientClaude, &domain.RetryPolicy{RequestMaxRetries: &zero}},
		{ClientHermes, &domain.RetryPolicy{StreamMaxRetries: &zero}},
	} {
		cfg := validConfig()
		binding := cfg.Clients[test.client]
		binding.RetryPolicy = test.policy
		cfg.Clients[test.client] = binding
		if err := cfg.Validate(); err == nil {
			t.Fatalf("invalid policy for %s was accepted", test.client)
		}
	}
}
