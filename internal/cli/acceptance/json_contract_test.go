package cli_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/program"
)

// applicationOutput returns the buffer that a test application writes to.
func applicationOutput(t *testing.T, app *cli.App) *bytes.Buffer {
	t.Helper()
	out, ok := app.Out.(*bytes.Buffer)
	if !ok {
		t.Fatalf("application output is %T", app.Out)
	}
	return out
}

func TestUpdateWritesItsTransitionAsJSON(t *testing.T) {
	cleanup := errors.New("remove update workspace: busy")
	for _, test := range []struct {
		name     string
		args     []string
		updater  *fakeUpdater
		homebrew bool
		failed   bool
		want     string
	}{
		{name: "update", updater: &fakeUpdater{updateResult: updated("0.2.0", "updated to v0.2.0")},
			want: `{"schema_version":1,"ok":true,"state":"updated","version":"0.2.0","summary":"updated to v0.2.0","next_action":"aigw sync"}`},
		{name: "current candidate", args: []string{"--candidate", "aigw_0.1.0_darwin_arm64.tar.gz", "--checksums", "checksums.txt"},
			updater: &fakeUpdater{candidateResult: program.Transition{State: program.TransitionCurrent, Version: "0.1.0", Summary: "already matches"}},
			want:    `{"schema_version":1,"ok":true,"state":"current","version":"0.1.0","summary":"already matches","next_action":"aigw sync"}`},
		{name: "rollback", args: []string{"--rollback"}, updater: &fakeUpdater{rollbackResult: restored("restored the previous program version.")},
			want: `{"schema_version":1,"ok":true,"state":"restored","version":"0.0.9","summary":"restored the previous program version.","next_action":"aigw sync"}`},
		{name: "Homebrew rollback", args: []string{"--rollback"}, updater: &fakeUpdater{}, homebrew: true,
			want: `{"schema_version":1,"ok":true,"state":"prepared","summary":"Credentials are ready for an earlier AIGW release.",` +
				`"next_detail":"Install the earlier release with Homebrew, then run ` + "`aigw sync` and `aigw check`" + `."}`},
		{name: "incomplete cleanup", updater: &fakeUpdater{updateResult: updated("0.2.0", "updated to v0.2.0"), updateErr: cleanup}, failed: true,
			want: `{"schema_version":1,"ok":false,"state":"updated","version":"0.2.0","summary":"updated to v0.2.0","problem":"Program updated; cleanup is incomplete",` +
				`"next_detail":"Run ` + "`aigw installation` and `aigw doctor`" + ` before retrying cleanup or another program transition."}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			app, out, _, _, _ := testApp(t, "")
			if test.homebrew {
				homebrewProgram(t, app)
				keychainRecorder(t, app, "0")
			}
			app.Updater = test.updater
			err := cli.Execute(app, append([]string{"update", "--json"}, test.args...))
			if (err != nil) != test.failed || (test.failed && !errors.Is(err, cleanup)) {
				t.Fatalf("update error = %v, want failed %v", err, test.failed)
			}
			var document bytes.Buffer
			if err := json.Compact(&document, out.Bytes()); err != nil || document.String() != test.want {
				t.Fatalf("document = %s, %v; want %s", out.String(), err, test.want)
			}
		})
	}
}

func TestUninstallWritesTheWithdrawnClientsAsJSON(t *testing.T) {
	for _, incomplete := range []bool{false, true} {
		t.Run(fmt.Sprintf("incomplete removal %v", incomplete), func(t *testing.T) {
			app, out, _, _, _ := testApp(t, "")
			writeFile(t, app.Executable, []byte("program"), 0o700)
			configureUninstallClients(t, app, filepath.Join(t.TempDir(), "config.toml"))
			if incomplete {
				writeFile(t, filepath.Join(program.RollbackPath(app.Executable), "foreign-child"), []byte("preserve"), 0o600)
			}
			out.Reset()
			err := cli.Execute(app, []string{"uninstall", "--json"})
			var document struct {
				OK               bool     `json:"ok"`
				ProgramFound     bool     `json:"program_found"`
				WithdrawnClients []string `json:"withdrawn_clients"`
				Problem          string   `json:"problem"`
			}
			if decodeErr := json.Unmarshal(out.Bytes(), &document); decodeErr != nil {
				t.Fatalf("document %q: %v", out.String(), decodeErr)
			}
			if (err != nil) != incomplete || document.OK == incomplete || !document.ProgramFound || (document.Problem != "") != incomplete ||
				!slices.Equal(document.WithdrawnClients, []string{claude.ID, codex.ID}) {
				t.Fatalf("uninstall error = %v, document %s", err, out.String())
			}
		})
	}
}

func TestVerifyAllWritesEachClientAsJSON(t *testing.T) {
	for _, test := range []struct {
		name    string
		warning bool
		states  []string
	}{
		{name: "completed", states: []string{"completed", "completed"}},
		{name: "failed", warning: true, states: []string{"failed", "not_run"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			app, runner := readyVerificationApp(t)
			if test.warning {
				runner.stderr = []byte("warning: native client capability is incomplete\n")
			}
			out := applicationOutput(t, app)
			out.Reset()
			err := cli.Execute(app, []string{"verify", "--for", "all", "--json"})
			var document struct {
				OK      bool `json:"ok"`
				Clients []struct {
					Client string `json:"client"`
					State  string `json:"state"`
				} `json:"clients"`
				CheckpointWritten bool `json:"checkpoint_written"`
			}
			if decodeErr := json.Unmarshal(out.Bytes(), &document); decodeErr != nil {
				t.Fatalf("document %q: %v", out.String(), decodeErr)
			}
			var clients, states []string
			for _, client := range document.Clients {
				clients, states = append(clients, client.Client), append(states, client.State)
			}
			completed := !test.warning
			_, checkpointErr := app.Config.LoadVerifiedCheckpoint()
			if (err == nil) != completed || document.OK != completed || document.CheckpointWritten != completed || (checkpointErr == nil) != completed ||
				!slices.Equal(clients, []string{claude.ID, codex.ID}) || !slices.Equal(states, test.states) {
				t.Fatalf("verify error = %v, checkpoint %v, document %s", err, checkpointErr, out.String())
			}
		})
	}
}

func TestJSONCommandsNeverPrompt(t *testing.T) {
	for _, test := range []struct {
		name  string
		args  []string
		ready bool
		want  string
	}{
		{name: "use without a Route", args: []string{"use", "--json"}, want: "requires a Route"},
		{name: "use without a client", args: []string{"use", "gpt", "--json"}, want: "requires --for"},
		{name: "use without a Token", args: []string{"use", "--for", "codex", "gpt", "--json"}, ready: true, want: "missing a token"},
		{name: "Account rename without IDs", args: []string{"account", "rename", "--json"}, want: "requires <old> <new>"},
		{name: "Route rename without its new ID", args: []string{"route", "rename", "gpt", "--json"}, want: "requires <old> <new>"},
		{name: "setup without a Token", args: []string{"setup", "--route", "gpt", "--account", "dmx", "--label", "DMX", "--for", "codex",
			"--openai-url", "https://example.test/v1", "--model", "gpt-test", "--json"}, want: "--json reads the Token only from --token-stdin"},
	} {
		t.Run(test.name, func(t *testing.T) {
			app, _, _, _, _ := testApp(t, "")
			if test.ready {
				app, _ = readyVerificationApp(t)
				if err := app.Secrets.Delete("dmx"); err != nil {
					t.Fatal(err)
				}
			}
			out := applicationOutput(t, app)
			out.Reset()
			prompt := &scriptedPrompt{}
			app.Interactive, app.Prompt = true, prompt
			err := cli.Execute(app, test.args)
			if err == nil || !strings.Contains(err.Error(), test.want) || prompt.callCount() != 0 {
				t.Fatalf("error = %v after %d prompts; want a refusal naming %q", err, prompt.callCount(), test.want)
			}
			var document struct {
				OK *bool `json:"ok"`
			}
			if decodeErr := json.Unmarshal(out.Bytes(), &document); decodeErr != nil || document.OK == nil || *document.OK {
				t.Fatalf("output is not one failure document: %q, %v", out.String(), decodeErr)
			}
		})
	}
}
