package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/process"
)

func TestNewDefaultBuildsAFunctioningApp(t *testing.T) {
	app, err := NewDefault()
	if err != nil {
		t.Fatalf("NewDefault() error = %v", err)
	}
	if app.GOOS == "" || app.DataDir == "" || app.Version == "" {
		t.Fatalf("NewDefault() produced an incomplete app: %#v", app)
	}
	if app.Config.Path() == "" {
		t.Fatal("NewDefault() did not wire a config path")
	}
	if filepath.Base(app.Config.Path()) != "config.toml" {
		t.Fatalf("NewDefault() config path = %q, want the stable config.toml contract", app.Config.Path())
	}
	if app.Secrets == nil || app.Accounts == nil || app.Runner == nil || app.HTTP == nil || app.Prompt == nil || app.Discovery == nil || app.Updater == nil {
		t.Fatalf("NewDefault() left a required dependency nil: %#v", app)
	}
	if _, ok := app.Runner.(process.Runner); !ok {
		t.Fatalf("NewDefault() runner = %T, want process.Runner", app.Runner)
	}
	if app.Now == nil {
		t.Fatal("NewDefault() did not wire a clock")
	}
}

func TestExecuteReturnsBusyLockErrorWhenAnotherMutationHoldsIt(t *testing.T) {
	store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	unlock, err := store.Lock(ctx)
	if err != nil {
		t.Fatalf("acquire external lock: %v", err)
	}
	t.Cleanup(func() {
		if err := unlock(); err != nil {
			t.Error(err)
		}
	})

	app := &App{Config: store, Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	err = Execute(app, []string{"add", "dmx"})
	if err == nil || !strings.Contains(err.Error(), "retry after the other command finishes") {
		t.Fatalf("Execute() error = %v, want a busy-lock error", err)
	}
}

type failingWriter struct{ err error }

func (writer failingWriter) Write([]byte) (int, error) { return 0, writer.err }

func TestExecuteCredentialFailureStaysMachineReadable(t *testing.T) {
	app := configuredCommandApp(t, configuredCommandState())
	var stderr bytes.Buffer
	app.Err = &stderr
	err := Execute(app, []string{"credential", "unsupported"})
	if err == nil || strings.Contains(stderr.String(), "Error") {
		t.Fatalf("error=%v stderr=%q", err, stderr.String())
	}
}

func TestExecuteReturnsRendererFailureAfterSuccessfulCommand(t *testing.T) {
	want := errors.New("output unavailable")
	app := configuredCommandApp(t, configuredCommandState())
	app.Out = failingWriter{err: want}
	app.Err = io.Discard
	if err := Execute(app, []string{"status"}); err == nil || !errors.Is(err, want) {
		t.Fatalf("renderer error = %v", err)
	}
}

func TestFinishExecutionPreservesCommandAndUnlockFailures(t *testing.T) {
	commandErr := errors.New("command failed")
	unlockErr := errors.New("unlock failed")
	if err := finishExecution(commandErr, func() error { return unlockErr }); err == nil || !errors.Is(err, commandErr) || !strings.Contains(err.Error(), "release config lock") {
		t.Fatalf("combined error = %v", err)
	}
	if err := finishExecution(nil, func() error { return unlockErr }); err == nil || !errors.Is(err, unlockErr) {
		t.Fatalf("unlock error = %v", err)
	}
	if err := finishExecution(commandErr, nil); !errors.Is(err, commandErr) {
		t.Fatalf("command error = %v", err)
	}
}
