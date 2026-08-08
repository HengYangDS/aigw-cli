package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/platform"
	"aigw-cli/internal/process"
)

func TestDefaultLauncherDirectoryIsAIGWOwnedNotExecutableOrSharedUserBin(t *testing.T) {
	env := map[string]string{"HOME": "/Users/alex", "APPDATA": `C:\Users\alex\AppData\Roaming`, "LOCALAPPDATA": `C:\Users\alex\AppData\Local`}
	got, err := platform.DefaultLauncherDirFor("darwin", env, "/usr/local/bin/aigw")
	if err != nil {
		t.Fatal(err)
	}
	want, err := platform.LauncherDirFor("darwin", env)
	if err != nil {
		t.Fatal(err)
	}
	if got != want || got == "/usr/local/bin" || got == "/Users/alex/.local/bin" {
		t.Fatalf("launcher dir = %q, want %q and not executable or shared user bin", got, want)
	}
}

func TestDefaultWindowsLauncherDirectorySharesThePortableInstallDirectory(t *testing.T) {
	env := map[string]string{"APPDATA": `C:\Users\alex\AppData\Roaming`, "LOCALAPPDATA": `C:\Users\alex\AppData\Local`}
	got, err := platform.DefaultLauncherDirFor("windows", env, `C:\Users\alex\AppData\Local\Programs\aigw\bin\aigw.exe`)
	if err != nil {
		t.Fatal(err)
	}
	want := `C:\Users\alex\AppData\Local\Programs\aigw\bin`
	if got != want {
		t.Fatalf("Windows default launcher dir = %q, want %q", got, want)
	}
}

func TestDefaultLauncherDirectoryHonorsExplicitOverride(t *testing.T) {
	env := map[string]string{"AIGW_LAUNCHER_DIR": "  /custom/shims  "}
	got, err := platform.DefaultLauncherDirFor("darwin", env, "/opt/bin/aigw")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/custom/shims" {
		t.Fatalf("launcher dir = %q, want the trimmed override", got)
	}
}

func TestDefaultLauncherDirectoryFallsBackToExecutableDirWhenPlatformLookupFails(t *testing.T) {
	got, err := platform.DefaultLauncherDirFor("darwin", map[string]string{}, "/opt/bin/aigw")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/opt/bin" {
		t.Fatalf("launcher dir = %q, want the executable's directory as a fallback", got)
	}
}

func TestExecutableDirectoryUsesTargetPathConvention(t *testing.T) {
	tests := []struct {
		name       string
		goos       string
		executable string
		want       string
	}{
		{name: "posix", goos: "linux", executable: "/opt/aigw/bin/aigw", want: "/opt/aigw/bin"},
		{name: "windows drive", goos: "windows", executable: `C:\Program Files\AIGW\aigw.exe`, want: `C:\Program Files\AIGW`},
		{name: "windows drive root", goos: "windows", executable: `C:\aigw.exe`, want: `C:\`},
		{name: "windows slash root", goos: "windows", executable: `/aigw.exe`, want: `/`},
		{name: "windows relative", goos: "windows", executable: `aigw.exe`, want: `.`},
		{name: "windows empty", goos: "windows", executable: ``, want: ``},
		{name: "windows trailing", goos: "windows", executable: `C:\AIGW\\`, want: `C:\`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := platform.ExecutableDirFor(test.goos, test.executable); got != test.want {
				t.Fatalf("executableDirFor(%q, %q) = %q, want %q", test.goos, test.executable, got, test.want)
			}
		})
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
