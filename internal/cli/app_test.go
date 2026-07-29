package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/config"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/platform"
)

func TestDefaultShimDirectoryIsAIGWOwnedNotExecutableOrSharedUserBin(t *testing.T) {
	env := map[string]string{"HOME": "/Users/alex", "APPDATA": `C:\Users\alex\AppData\Roaming`, "LOCALAPPDATA": `C:\Users\alex\AppData\Local`}
	got, err := defaultShimDirFor("darwin", env, "/usr/local/bin/aigw")
	if err != nil {
		t.Fatal(err)
	}
	want, err := platform.ShimDirFor("darwin", env)
	if err != nil {
		t.Fatal(err)
	}
	if got != want || got == "/usr/local/bin" || got == "/Users/alex/.local/bin" {
		t.Fatalf("shim dir = %q, want %q and not executable or shared user bin", got, want)
	}
}

func TestDefaultWindowsShimDirectorySharesThePortableInstallDirectory(t *testing.T) {
	env := map[string]string{"APPDATA": `C:\Users\alex\AppData\Roaming`, "LOCALAPPDATA": `C:\Users\alex\AppData\Local`}
	got, err := defaultShimDirFor("windows", env, `C:\Users\alex\AppData\Local\Programs\aigw\bin\aigw.exe`)
	if err != nil {
		t.Fatal(err)
	}
	want := `C:\Users\alex\AppData\Local\Programs\aigw\bin`
	if got != want {
		t.Fatalf("Windows default shim dir = %q, want %q", got, want)
	}
}

func TestDefaultShimDirectoryHonorsExplicitOverride(t *testing.T) {
	env := map[string]string{"AIGW_SHIM_DIR": "  /custom/shims  "}
	got, err := defaultShimDirFor("darwin", env, "/opt/bin/aigw")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/custom/shims" {
		t.Fatalf("shim dir = %q, want the trimmed override", got)
	}
}

func TestDefaultShimDirectoryFallsBackToExecutableDirWhenPlatformLookupFails(t *testing.T) {
	got, err := defaultShimDirFor("darwin", map[string]string{}, "/opt/bin/aigw")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/opt/bin" {
		t.Fatalf("shim dir = %q, want the executable's directory as a fallback", got)
	}
}

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
	if app.Secrets == nil || app.Accounts == nil || app.Runner == nil || app.HTTP == nil || app.Prompt == nil || app.Discovery == nil || app.Updater == nil {
		t.Fatalf("NewDefault() left a required dependency nil: %#v", app)
	}
	if _, ok := app.Runner.(ProcessRunner); !ok {
		t.Fatalf("NewDefault() runner = %T, want ProcessRunner", app.Runner)
	}
	if app.Now == nil {
		t.Fatal("NewDefault() did not wire a clock")
	}
}

func TestLimitedBufferWriteEnforcesLimitAndFlagsOverflow(t *testing.T) {
	buf := &limitedBuffer{limit: 8}
	n, err := buf.Write([]byte("1234"))
	if err != nil || n != 4 || buf.overflow {
		t.Fatalf("first write = (%d, %v), overflow=%v", n, err, buf.overflow)
	}
	n, err = buf.Write([]byte("567890"))
	if n != 4 || !errors.Is(err, errCapturedProcessOutputLimit) || !buf.overflow {
		t.Fatalf("partial overflow write = (%d, %v), overflow=%v", n, err, buf.overflow)
	}
	if buf.String() != "12345678" {
		t.Fatalf("buffer contents = %q, want truncated at the limit", buf.String())
	}
	n, err = buf.Write([]byte("x"))
	if n != 0 || !errors.Is(err, errCapturedProcessOutputLimit) {
		t.Fatalf("write past a full buffer = (%d, %v)", n, err)
	}
}

func TestProcessRunnerRunExecutesCommandSuccessfully(t *testing.T) {
	err := (ProcessRunner{}).Run(context.Background(), adapters.ProcessPlan{
		Executable: "/usr/bin/true",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestProcessRunnerRunReportsChildProcessFailure(t *testing.T) {
	err := (ProcessRunner{}).Run(context.Background(), adapters.ProcessPlan{
		Executable: "/usr/bin/false",
	})
	if err == nil || !strings.Contains(err.Error(), "run /usr/bin/false") {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestProcessRunnerRunRejectsMissingExecutable(t *testing.T) {
	err := (ProcessRunner{}).Run(context.Background(), adapters.ProcessPlan{
		Executable: filepath.Join(t.TempDir(), "does-not-exist"),
	})
	if err == nil {
		t.Fatal("Run() with a missing executable should fail")
	}
}

func TestProcessRunnerRunReplacesProcessErrorSurfacesLookupFailure(t *testing.T) {
	err := (ProcessRunner{}).Run(context.Background(), adapters.ProcessPlan{
		Executable: "aigw-definitely-not-a-real-binary",
		Replace:    true,
	})
	if err == nil || !strings.Contains(err.Error(), "Failed to resolve") {
		t.Fatalf("Run() with Replace error = %v", err)
	}
}

func TestProcessRunnerRunCaptureRejectsReplace(t *testing.T) {
	_, err := (ProcessRunner{}).RunCapture(context.Background(), adapters.ProcessPlan{
		Executable: "/usr/bin/true",
		Replace:    true,
	})
	if err == nil || !strings.Contains(err.Error(), "cannot replace the current process") {
		t.Fatalf("RunCapture() with Replace error = %v", err)
	}
}

func TestProcessRunnerRunCaptureSurfacesStartFailure(t *testing.T) {
	_, err := (ProcessRunner{}).RunCapture(context.Background(), adapters.ProcessPlan{
		Executable: filepath.Join(t.TempDir(), "does-not-exist"),
	})
	if err == nil || !strings.Contains(err.Error(), "start ") {
		t.Fatalf("RunCapture() error = %v", err)
	}
}

func TestProcessRunnerRunCaptureSurfacesNonZeroExit(t *testing.T) {
	_, err := (ProcessRunner{}).RunCapture(context.Background(), adapters.ProcessPlan{
		Executable: "/usr/bin/false",
	})
	var exitErr *exec.ExitError
	if err == nil || !strings.Contains(err.Error(), "run /usr/bin/false") || !errors.As(err, &exitErr) {
		t.Fatalf("RunCapture() error = %v", err)
	}
}

func TestProcessRunnerRunCaptureReturnsStdout(t *testing.T) {
	output, err := (ProcessRunner{}).RunCapture(context.Background(), adapters.ProcessPlan{
		Executable: "/bin/echo",
		Args:       []string{"AIGW_OK"},
	})
	if err != nil {
		t.Fatalf("RunCapture() error = %v", err)
	}
	if strings.TrimSpace(string(output)) != "AIGW_OK" {
		t.Fatalf("RunCapture() output = %q", output)
	}
}

func TestPresentationWidthPrefersEnvironmentOverride(t *testing.T) {
	if got := presentationWidth(&bytes.Buffer{}, map[string]string{"COLUMNS": "55"}); got != 55 {
		t.Fatalf("presentationWidth() = %d, want 55", got)
	}
}

func TestPresentationWidthIsZeroForNonFileWriter(t *testing.T) {
	if got := presentationWidth(&bytes.Buffer{}, map[string]string{}); got != 0 {
		t.Fatalf("presentationWidth() = %d, want 0 for a non-file writer", got)
	}
}

func TestPresentationWidthIsZeroForNonTerminalFile(t *testing.T) {
	file, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	})
	if got := presentationWidth(file, map[string]string{}); got != 0 {
		t.Fatalf("presentationWidth() = %d, want 0 for a non-terminal file", got)
	}
}

func TestReadTokenFromStdinTrimsAndAccepts(t *testing.T) {
	app := &App{In: strings.NewReader("  a-token  \n")}
	value, err := app.readToken(true, false)
	if err != nil || value != "a-token" {
		t.Fatalf("readToken() = (%q, %v)", value, err)
	}
}

func TestReadTokenFromStdinAcceptsFinalLineWithoutNewline(t *testing.T) {
	app := &App{In: strings.NewReader("trailing-token")}
	value, err := app.readToken(true, false)
	if err != nil || value != "trailing-token" {
		t.Fatalf("readToken() = (%q, %v)", value, err)
	}
}

func TestReadTokenFromStdinRejectsEmptyInput(t *testing.T) {
	app := &App{In: strings.NewReader("\n")}
	_, err := app.readToken(true, false)
	if err == nil || !strings.Contains(err.Error(), "Empty tokens are not accepted") {
		t.Fatalf("readToken() error = %v", err)
	}
}

func TestReadTokenFromStdinSurfacesReadFailure(t *testing.T) {
	app := &App{In: errorReader{err: errors.New("broken pipe")}}
	_, err := app.readToken(true, false)
	if err == nil || !strings.Contains(err.Error(), "Failed to read token from standard input") {
		t.Fatalf("readToken() error = %v", err)
	}
}

func TestReadTokenNonStdinRequiresInteractiveTerminal(t *testing.T) {
	app := &App{Interactive: false}
	_, err := app.readToken(false, false)
	if err == nil || !strings.Contains(err.Error(), "requires an interactive terminal") {
		t.Fatalf("readToken() error = %v", err)
	}
}

func TestExecuteReturnsBusyLockErrorWhenAnotherMutationHoldsIt(t *testing.T) {
	store := config.NewStore(filepath.Join(t.TempDir(), "config.toml"))
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
