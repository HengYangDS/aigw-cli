package configuration

import (
	"bytes"
	"encoding/json"
	"math"
	"path/filepath"
	"reflect"
	"testing"
)

func TestConnectionRetryPolicyPreservesCachedCredentialConfiguration(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "aigw.toml"), testAdmission())
	cfg := validConfig()
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	before, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	zero := uint64(0)
	binding := cfg.Clients[ClientCodex]
	binding.RetryPolicy = &RetryPolicy{RequestMaxRetries: &zero, StreamMaxRetries: &zero}
	cfg.Clients[ClientCodex] = binding
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	after, err := store.CaptureSnapshot()
	if err != nil || !bytes.Equal(before.Config.Data, after.Config.Data) {
		t.Fatalf("retry selection changed immutable predecessor credential input: %v", err)
	}
	credentialConfig, err := store.LoadCredentialConfig()
	if err != nil || credentialConfig.Clients[ClientCodex].RetryPolicy != nil {
		t.Fatalf("transport policy entered credential authority: %v", err)
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
	declared := append(append([]byte(nil), before.Config.Data...), []byte("\n[clients.codex.retry_policy]\nrequest_max_retries = 0\nstream_max_retries = 0\n")...)
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
	runtime, err := loaded.ResolveRuntime(ClientCodex, "")
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
	binding.RetryPolicy = &RetryPolicy{RequestMaxRetries: &zero}
	cfg.Clients[ClientCodex] = binding
	clone := cfg.Clone()
	*clone.Clients[ClientCodex].RetryPolicy.RequestMaxRetries = 2
	runtime, err := cfg.ResolveRuntime(ClientCodex, "")
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
		policy *RetryPolicy
	}{
		{ClientCodex, &RetryPolicy{}},
		{ClientClaude, &RetryPolicy{RequestMaxRetries: &zero}},
		{ClientHermes, &RetryPolicy{StreamMaxRetries: &zero}},
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

func TestConnectionRetryPolicyRequiresTOMLIntegerBudgets(t *testing.T) {
	for _, value := range []uint64{0, math.MaxInt64, math.MaxInt64 + 1} {
		for _, policy := range []*RetryPolicy{{RequestMaxRetries: &value}, {StreamMaxRetries: &value}} {
			err := policy.validate(ClientCodex)
			if (err == nil) != (value <= math.MaxInt64) {
				t.Fatalf("budget %d validation = %v", value, err)
			}
		}
	}
}
