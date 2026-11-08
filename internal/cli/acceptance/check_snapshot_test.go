package cli_test

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli"
)

// outputOf is the buffer to which the fixture's app writes, after the
// provider is set to answer the requests of the endpoint and model depths,
// which the client depth runs first.
func outputOf(t *testing.T, app *cli.App) *bytes.Buffer {
	t.Helper()
	out, ok := app.Out.(*bytes.Buffer)
	if !ok {
		t.Fatalf("the app writes to a %T", app.Out)
	}
	app.HTTP = &fakeHTTP{status: http.StatusOK, handler: func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(fixtureResponseBody(req))), Request: req}, nil
	}}
	return out
}

// A check of every enabled client to the client depth runs each client's
// session and writes the verified snapshot, which records the depth that each
// client reached and which a rollback reads.
func TestCheckToTheClientDepthWritesTheVerifiedSnapshot(t *testing.T) {
	app, runner := readyVerificationApp(t)
	out := outputOf(t, app)
	out.Reset()
	if err := cli.Execute(app, []string{"check", "--depth", "client"}); err != nil {
		t.Fatalf("check --depth client: %v\n%s", err, out)
	}
	checkpoint, err := app.Config.LoadVerifiedCheckpoint()
	if err != nil || !slices.Equal(checkpoint.Clients, sessions(claude.ID, codex.ID)) {
		t.Fatalf("verified snapshot = %+v, %v", checkpoint.Clients, err)
	}
	if len(runner.plans) == 0 || !strings.Contains(out.String(), "Saved the verified snapshot") {
		t.Fatalf("the check ran %d plans and said:\n%s", len(runner.plans), out)
	}
}

// A configuration that changes while a client's session runs keeps its newer
// content, and the snapshot of the older one is refused with the same check
// as its fix.
func TestCheckRefusesTheSnapshotOfAConfigurationThatChanged(t *testing.T) {
	app, runner := readyVerificationApp(t)
	app.Runner = verificationCompletionRunner{fakeRunner: runner, completed: func() {
		cfg, err := app.Config.Load()
		if err != nil {
			t.Fatal(err)
		}
		model := cfg.Models["gpt"]
		model.Family = "newer-model"
		cfg.Models["gpt"] = model
		if err := app.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
	}}
	out := outputOf(t, app)
	out.Reset()
	if err := cli.Execute(app, []string{"check", "--depth", "client", "--json"}); err == nil {
		t.Fatal("the snapshot of a configuration that changed was written")
	}
	report := decodeCheck(t, out)
	if report.OK || !strings.Contains(report.Problem, "The verified snapshot was not written: configuration changed during the check") || report.NextAction != "aigw check --depth client" {
		t.Fatalf("refused snapshot = %+v", report)
	}
	if current, err := app.Config.Load(); err != nil || current.Models["gpt"].Family != "newer-model" {
		t.Fatalf("the newer configuration was lost: %v", err)
	}
	if _, err := os.Stat(app.Config.Path() + ".verified.json"); !os.IsNotExist(err) {
		t.Fatalf("a verified snapshot exists: %v", err)
	}
}
