//go:build client_acceptance

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aigw-cli/internal/client/codex"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
	"aigw-cli/internal/upgrade"

	"aigw-cli/internal/clients"

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
	manifest, err := store.Parse(team, clients.Admission())
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
	selection := p.manifest.Recommendations[codex.ID].Primary
	route := p.manifest.Routes[selection.Route]
	const token = "native-retry-fixture-token"
	var failures, mode, completions atomic.Int64
	base := clientResponseHandler(domain.ProtocolOpenAIResponses, map[string]*atomic.Int64{route.UpstreamModel: &completions}, token, "high", nativeCodexToolShell(t))
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
	if err := journey.prepareNativeClient(codex.ID, executable, p.team); err != nil {
		t.Fatal(err)
	}
	journey.setEnvironment(secret.EnvironmentKey(route.Account), token)
	journey.run("setup", "--from", journey.manifest, "--account", route.Account)
	journey.enableNativeClient(codex.ID, executable)
	retained := journey.retainedCredential(codex.ID)
	journey.runWith(p.candidate, "sync")
	// Selection may bind the successor's credential reader; establish that
	// independent transition before measuring policy-only canonical preservation.
	journey.runWith(p.candidate, "use", "--for", codex.ID, selection.Route)
	canonical := readFile(t, journey.config)
	args := []string{"use", "--for", codex.ID, selection.Route, "--request-max-retries", "0", "--stream-max-retries", "0"}
	preview := journey.runWith(p.candidate, append(args, "--dry-run", "--json")...)
	var selected domain.Runtime
	if err := json.Unmarshal(preview, &selected); err != nil || selected.RetryPolicy == nil {
		t.Fatalf("retry preview omitted policy: %v", err)
	}
	journey.runWith(p.candidate, args...)
	if !bytes.Equal(canonical, readFile(t, journey.config)) {
		var before, after domain.Config
		if err := toml.Unmarshal(canonical, &before); err != nil {
			t.Fatal(err)
		}
		if err := toml.Unmarshal(readFile(t, journey.config), &after); err != nil {
			t.Fatal(err)
		}
		t.Fatalf("retry selection changed cached predecessor credential input: binding before=%+v after=%+v", before.Clients[codex.ID], after.Clients[codex.ID])
	}
	journey.requireCredential(retained, token)
	journey.requireRetryRollbackBoundary(p.baseline)
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
	journey.runWith(p.candidate, "use", "--for", codex.ID, selection.Route, "--native-retries")
	if bytes.Contains(readFile(t, path), []byte("max_retries")) {
		t.Fatal("native reset retained retry fields")
	}
	journey.uninstallWithAndRequireInstallationRemoved(p.candidate)
}

// retryPolicyReaderFloor is the first release whose configuration reader decodes
// a client retry policy: `--request-max-retries` shipped in 0.4.17.
var retryPolicyReaderFloor = []int{0, 4, 17}

// requireRetryRollbackBoundary holds a retained baseline to the boundary it
// actually has. Program rollback probes the retained reader itself; the
// release history supplies the independent expectation.
func (j *journeyFixture) requireRetryRollbackBoundary(baseline string) {
	j.testing.Helper()
	if j.readsRetryPolicy(baseline) {
		j.requireRetryRollbackAdmission(baseline)
		return
	}
	j.requireRetryRollbackRefusal(baseline)
}

func (j *journeyFixture) readsRetryPolicy(program string) bool {
	t := j.testing
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	output, _, err := (process.Runner{}).RunCaptureStreams(ctx, process.Plan{Executable: program, Args: []string{"--version"}, Env: j.environment, Directory: j.root})
	reported, found := strings.CutPrefix(strings.TrimSpace(string(output)), "aigw version ")
	version := make([]int, 3)
	if _, scanErr := fmt.Sscanf(reported, "%d.%d.%d", &version[0], &version[1], &version[2]); err != nil || !found || scanErr != nil {
		t.Fatalf("read the retained program's version: %q, %v, %v", output, err, scanErr)
	}
	return slices.Compare(version, retryPolicyReaderFloor) >= 0
}

// requireRetryRollbackAdmission rolls back to a retained baseline that reads
// retry policies while one is selected, then rolls back again to the candidate.
// Both programs read the same declaration, and each rollback swaps exactly the
// active and retained programs.
func (j *journeyFixture) requireRetryRollbackAdmission(baseline string) {
	t := j.testing
	t.Helper()
	admission, err := store.NewStore(j.config, clients.Admission()).RollbackConfiguration()
	if err != nil {
		t.Fatal(err)
	}
	retained := upgrade.RollbackPath(j.binary)
	program, active := readFile(t, baseline), readFile(t, j.binary)
	if err := os.WriteFile(retained, program, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, want := range [][2][]byte{{program, active}, {active, program}} {
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		_, err := (upgrade.Updater{Executable: j.binary, Runner: process.Runner{}}).Rollback(ctx, admission, nil)
		cancel()
		if err != nil {
			t.Fatalf("a retained program that reads retry policies refused rollback: %v", err)
		}
		if !bytes.Equal(want[0], readFile(t, j.binary)) || !bytes.Equal(want[1], readFile(t, retained)) {
			t.Fatal("rollback did not swap exactly the active and retained programs")
		}
	}
	if err := os.Remove(retained); err != nil {
		t.Fatal(err)
	}
}

func (j *journeyFixture) requireRetryRollbackRefusal(baseline string) {
	t := j.testing
	t.Helper()
	admission, err := store.NewStore(j.config, clients.Admission()).RollbackConfiguration()
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
