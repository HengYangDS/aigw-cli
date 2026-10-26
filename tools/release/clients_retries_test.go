//go:build client_acceptance

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aigw-cli/internal/configuration"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secrets"
	"aigw-cli/internal/upgrade"

	"github.com/pelletier/go-toml/v2"
)

func TestNativeCodexConnectionRetriesSource(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := requiredClientInput("AIGW_ACCEPTANCE_RETRY_PROGRAM", false)
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := requiredClientInput("AIGW_ACCEPTANCE_BASELINE", false)
	if err != nil {
		t.Fatal(err)
	}
	team := readFile(t, filepath.Join(root, "manifests", "team.toml"))
	manifest, err := configuration.Parse(team)
	if err != nil {
		t.Fatal(err)
	}
	plan := nativeClientJourneyPlan{team: team, manifest: manifest, baseline: baseline, candidate: candidate}
	plan.runCodexConnectionRetries(t)
}

func (p nativeClientJourneyPlan) runCodexConnectionRetries(t *testing.T) {
	t.Helper()
	executable, err := requiredClientInput("AIGW_ACCEPTANCE_CODEX", false)
	if err != nil {
		t.Fatal(err)
	}
	selection := p.manifest.Recommendations[configuration.ClientCodex].Primary
	route := p.manifest.Routes[selection.Route]
	const token = "native-retry-fixture-token"
	var failures, mode, completions atomic.Int64
	base := clientResponseHandler(configuration.ProtocolOpenAIResponses, map[string]*atomic.Int64{route.UpstreamModel: &completions}, token, "high", nativeCodexToolShell(t))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if mode.Load() == 0 || r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/responses") {
			base.ServeHTTP(w, r)
			return
		}
		defer func() {
			if err := r.Body.Close(); err != nil {
				t.Error(err)
			}
		}()
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			t.Error(err)
			return
		}
		failures.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		if _, err := io.WriteString(w, `{"error":{"type":"upstream_outcome_unknown","code":"upstream_outcome_unknown","message":"upstream_outcome_unknown"}}`); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	journey := newNativeJourney(t, p.baseline, server.URL+"/v1", false)
	if err := journey.prepareNativeClient(configuration.ClientCodex, executable, p.team); err != nil {
		t.Fatal(err)
	}
	journey.setEnvironment(secrets.EnvironmentKey(route.Account), token)
	journey.run("setup", "--from", journey.manifest, "--account", route.Account)
	journey.enableNativeClient(configuration.ClientCodex, executable)
	retained := journey.retainedCredential(configuration.ClientCodex)
	journey.runWith(p.candidate, "sync")
	// Selection may bind the successor's credential reader; establish that
	// independent transition before measuring policy-only canonical preservation.
	journey.runWith(p.candidate, "use", "--for", configuration.ClientCodex, selection.Route)
	canonical := readFile(t, journey.config)
	args := []string{"use", "--for", configuration.ClientCodex, selection.Route, "--request-max-retries", "0", "--stream-max-retries", "0"}
	preview := journey.runWith(p.candidate, append(args, "--dry-run", "--json")...)
	var selected configuration.Runtime
	if err := json.Unmarshal(preview, &selected); err != nil || selected.RetryPolicy == nil {
		t.Fatalf("retry preview omitted policy: %v", err)
	}
	journey.runWith(p.candidate, args...)
	if !bytes.Equal(canonical, readFile(t, journey.config)) {
		var before, after configuration.Config
		if err := toml.Unmarshal(canonical, &before); err != nil {
			t.Fatal(err)
		}
		if err := toml.Unmarshal(readFile(t, journey.config), &after); err != nil {
			t.Fatal(err)
		}
		t.Fatalf("retry selection changed cached predecessor credential input: binding before=%+v after=%+v", before.Clients[configuration.ClientCodex], after.Clients[configuration.ClientCodex])
	}
	journey.requireCredential(retained, token)
	journey.requireRetryRollbackRefusal(p.baseline)
	path := filepath.Join(journey.root, "home", ".codex", "config.toml")
	var native struct {
		Provider  string `toml:"model_provider"`
		Providers map[string]struct {
			RequestBudget *uint64 `toml:"request_max_retries"`
			StreamBudget  *uint64 `toml:"stream_max_retries"`
		} `toml:"model_providers"`
	}
	if err := toml.Unmarshal(readFile(t, path), &native); err != nil {
		t.Fatal(err)
	}
	policy := native.Providers[native.Provider]
	if policy.RequestBudget == nil || policy.StreamBudget == nil || *policy.RequestBudget != 0 || *policy.StreamBudget != 0 {
		t.Fatal("native projection omitted zero budgets")
	}
	mode.Store(1)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	stdout, stderr, runErr := (process.Runner{}).RunCaptureStreams(ctx, process.Plan{
		Executable: executable, Directory: journey.root, Env: journey.environment,
		Args: []string{"exec", "--skip-git-repo-check", "--ephemeral", "--json", "--sandbox", "read-only", "Reply only OK."},
	})
	if ctx.Err() != nil || runErr == nil || failures.Load() != 1 || !bytes.Contains(append(stdout, stderr...), []byte("upstream_outcome_unknown")) {
		t.Fatalf("native no-retry boundary: requests=%d, error=%v, deadline=%v\n%s\n%s", failures.Load(), runErr, ctx.Err(), stdout, stderr)
	}
	mode.Store(0)
	journey.runWith(p.candidate, "use", "--for", configuration.ClientCodex, selection.Route, "--native-retries")
	if bytes.Contains(readFile(t, path), []byte("max_retries")) {
		t.Fatal("native reset retained retry fields")
	}
	journey.uninstallWithAndRequireInstallationRemoved(p.candidate)
}

func (j *journeyFixture) requireRetryRollbackRefusal(baseline string) {
	t := j.testing
	t.Helper()
	admission, err := configuration.NewStore(j.config).RollbackConfiguration()
	if err != nil {
		t.Fatal(err)
	}
	retained := upgrade.RollbackPath(j.binary)
	program := readFile(t, baseline)
	active := readFile(t, j.binary)
	if err := os.WriteFile(retained, program, 0o700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	_, err = (upgrade.Updater{Executable: j.binary, Runner: process.Runner{}}).Rollback(ctx, admission, nil)
	if !errors.Is(err, upgrade.ErrRollbackConfiguration) || ctx.Err() != nil {
		t.Fatalf("retained reader did not refuse policy at its native boundary: %v", err)
	}
	if !bytes.Equal(active, readFile(t, j.binary)) || !bytes.Equal(program, readFile(t, retained)) {
		t.Fatal("refused capability probe changed program bytes")
	}
	if err := os.Remove(retained); err != nil {
		t.Fatal(err)
	}
}
