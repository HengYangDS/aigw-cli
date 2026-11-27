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
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
	"aigw-cli/internal/platform/process"
	"aigw-cli/internal/program"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"

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
	manifest, err := store.ParseTeamFile(team, clients.Admission())
	if err != nil {
		t.Fatal(err)
	}
	plan := nativeClientJourneyPlan{team: team, manifest: manifest, baseline: baseline, candidate: candidate, publishedBaseline: true}
	plan.runCodexConnectionRetries(t)
}

func (p nativeClientJourneyPlan) runCodexConnectionRetries(t *testing.T) {
	t.Helper()
	executable, err := requiredClientInput("AIGW_ACCEPTANCE_CODEX", false)
	if err != nil {
		t.Fatal(err)
	}
	selection := p.manifest.Recommendations[codex.ID].Primary
	route := p.manifest.Models[selection.Model]
	const token = "native-retry-fixture-token"
	var failures, mode, completions atomic.Int64
	base := clientResponseHandler(domain.ProtocolResponses, map[string]*atomic.Int64{route.ID: &completions}, token, "high")
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
	journey.setEnvironment(secret.EnvironmentKey(route.Provider), token)
	journey.run("setup", "--from", p.baselineManifest(journey))
	journey.enableNativeClient(codex.ID, executable)
	retained := journey.retainedCredential(codex.ID)
	// The candidate reads aigw.toml from here on.
	journey.crossSchema(p.candidate)
	journey.runWith(p.candidate, "sync")
	// Selection may bind the successor's credential reader; establish that
	// independent transition before measuring policy-only canonical preservation.
	name := journey.modelName(selection.Model)
	journey.runWith(p.candidate, "use", codex.ID, name)
	credentials := store.NewStore(journey.config, clients.Admission())
	before, err := credentials.LoadCredentialView()
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"client", "edit", codex.ID, "--request-retries", "0", "--stream-retries", "0"}
	preview := journey.runWith(p.candidate, append(args, "--dry-run", "--json")...)
	var edited struct {
		Retries *struct {
			Request *uint64 `json:"request"`
			Stream  *uint64 `json:"stream"`
		} `json:"retries"`
		Model string `json:"model"`
	}
	if err := json.Unmarshal(preview, &edited); err != nil || edited.Retries == nil || edited.Model != name {
		t.Fatalf("retry preview omitted the budgets or model %q: %v", edited.Model, err)
	}
	journey.runWith(p.candidate, args...)
	// A retry budget is policy, which aigw.toml now holds beside the rest of the
	// configuration, so the edit rewrites that file. What a retained credential
	// reader decodes from it must not change.
	after, err := credentials.LoadCredentialView()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("retry selection changed the credential reader's view: before=%+v after=%+v", before, after)
	}
	journey.requireCredential(retained, token)
	journey.requireRetryRollbackAdmission(p.baseline, p.candidate)
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
	journey.runWith(p.candidate, "client", "edit", codex.ID, "--native-retries")
	if bytes.Contains(readFile(t, path), []byte("max_retries")) {
		t.Fatal("native reset retained retry fields")
	}
	journey.uninstallWithAndRequireInstallationRemoved(p.candidate)
}

// requireRetryRollbackAdmission installs the candidate with the baseline
// retained, as an update leaves them, rolls back to the baseline while a
// retry policy is selected, then rolls back again to the candidate; each
// rollback swaps exactly the active and retained programs, and each program
// is admitted with a file that it reads. A baseline of the candidate's schema
// reads the candidate's file. A baseline of the schema that the candidate
// replaced cannot: the rollback refuses and changes nothing, the baseline is
// admitted with the copy of aigw.toml kept before the schema edits, and the
// candidate with the file in which it selected the policy, which the rest of
// the journey edits.
func (j *journeyFixture) requireRetryRollbackAdmission(baseline, candidate string) {
	t := j.testing
	t.Helper()
	retained := program.RollbackPath(j.binary)
	previous, active := readFile(t, baseline), readFile(t, candidate)
	if bytes.Equal(previous, active) {
		t.Fatal("the baseline and the candidate are one program, so no rollback can swap them")
	}
	for path, data := range map[string][]byte{j.binary: active, retained: previous} {
		if err := platform.WriteFileAtomic(path, data, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	selected := readFile(t, j.config)
	admissions := [2][]byte{selected, selected}
	if !j.firstProgramReadsConfiguration() {
		if len(j.keptConfiguration) == 0 {
			t.Fatal("no copy of aigw.toml was kept before the schema edits")
		}
		before := j.captureState()
		if err := j.rollbackProgram(); !errors.Is(err, program.ErrRollbackConfiguration) {
			t.Fatalf("a retained program that cannot read aigw.toml did not refuse rollback: %v", err)
		}
		if !reflect.DeepEqual(before, j.captureState()) {
			t.Fatal("the refused rollback changed the configuration, its recovery state, a program or a client file")
		}
		admissions[0] = j.keptConfiguration
	}
	for index, want := range [][2][]byte{{previous, active}, {active, previous}} {
		if !bytes.Equal(readFile(t, j.config), admissions[index]) {
			if err := os.WriteFile(j.config, admissions[index], 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if err := j.rollbackProgram(); err != nil {
			t.Fatalf("a retained program that reads aigw.toml refused rollback: %v", err)
		}
		if !bytes.Equal(want[0], readFile(t, j.binary)) || !bytes.Equal(want[1], readFile(t, retained)) {
			t.Fatal("rollback did not swap exactly the active and retained programs")
		}
	}
	if err := os.Remove(retained); err != nil {
		t.Fatal(err)
	}
}

// rollbackProgram rolls the installed program back to the retained one,
// admitted with aigw.toml as it is.
func (j *journeyFixture) rollbackProgram() error {
	j.testing.Helper()
	admission, err := store.NewStore(j.config, clients.Admission()).RollbackConfiguration()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(j.testing.Context(), 30*time.Second)
	defer cancel()
	_, err = (program.Updater{Executable: j.binary, Runner: process.Runner{}}).Rollback(ctx, admission)
	return err
}
