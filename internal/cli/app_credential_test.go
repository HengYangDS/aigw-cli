package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/synchronization"
)

func TestNewDefaultBindsCredentialEntrypointToExecutableBytes(t *testing.T) {
	root := t.TempDir()
	for key, value := range map[string]string{
		"HOME": root, "USERPROFILE": root,
		"XDG_CONFIG_HOME": filepath.Join(root, "config"),
		"XDG_DATA_HOME":   filepath.Join(root, "data"),
		"APPDATA":         filepath.Join(root, "roaming"),
		"LOCALAPPDATA":    filepath.Join(root, "local"),
	} {
		t.Setenv(key, value)
	}
	t.Setenv("AIGW_SECRET_BACKEND", "env")
	app, err := NewDefault()
	if err != nil {
		t.Fatal(err)
	}
	if app.CredentialPath != "" {
		t.Fatal("default app eagerly read executable bytes before any credential consumer")
	}
	executable, err := os.Open(app.Executable)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := executable.Close(); err != nil {
			t.Error(err)
		}
	}()
	hash := sha256.New()
	if _, err := io.Copy(hash, executable); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(app.DataDir, "credential", hex.EncodeToString(hash.Sum(nil)), filepath.Base(app.InstallTarget))
	path, err := invocation.Synchronizer(app.invocationContext()).CredentialEntrypointPath()
	if err != nil || path != want {
		t.Fatalf("credential entrypoint = %q, want exact executable identity %q: %v", path, want, err)
	}
	var wait sync.WaitGroup
	for range 8 {
		wait.Go(func() {
			if repeated, err := invocation.Synchronizer(app.invocationContext()).CredentialEntrypointPath(); err != nil || repeated != path {
				t.Errorf("shared invocation reader identity = %q, %v", repeated, err)
			}
		})
	}
	wait.Wait()
	app.CredentialPath = filepath.Join(root, "explicit-reader")
	app.ResolveCredentialPath = func() (string, error) {
		t.Fatal("explicit reader path invoked the default resolver")
		return "", nil
	}
	if explicit, err := invocation.Synchronizer(app.invocationContext()).CredentialEntrypointPath(); err != nil || explicit != app.CredentialPath {
		t.Fatalf("explicit reader path = %q, %v", explicit, err)
	}
	if _, err := os.Lstat(want); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("app construction wrote a credential entrypoint: %v", err)
	}
}

func TestRootHelpAndCredentialErrorsDoNotResolveReaderIdentity(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"--version"}, {"help", "sync"}, {"completion", "bash"}, {"credential", "unknown", "invalid"}} {
		t.Run(args[0], func(t *testing.T) {
			app := &App{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}, ResolveCredentialPath: func() (string, error) {
				t.Fatal("non-reader CLI invoked executable identity resolution")
				return "", nil
			}}
			err := Execute(app, args)
			if (args[0] == "credential") != (err != nil) {
				t.Fatalf("CLI result = %v", err)
			}
		})
	}
}

func TestInvocationReaderIdentityIsMemoizedWithoutGlobalState(t *testing.T) {
	for _, fail := range []bool{false, true} {
		calls := 0
		problem := errors.New("exact identity unavailable")
		app := &App{ResolveCredentialPath: sync.OnceValues(func() (string, error) {
			calls++
			if fail {
				return "", problem
			}
			return "exact-reader", nil
		})}
		for range 3 {
			path, err := invocation.Synchronizer(app.invocationContext()).CredentialEntrypointPath()
			if fail && !errors.Is(err, problem) || !fail && (path != "exact-reader" || err != nil) {
				t.Fatalf("memoized reader = %q, %v", path, err)
			}
		}
		if calls != 1 {
			t.Fatalf("identity reads = %d, want one for this invocation", calls)
		}
		other := synchronization.Synchronizer{CredentialPath: "independent-reader"}
		if path, err := other.CredentialEntrypointPath(); path != "independent-reader" || err != nil {
			t.Fatalf("independent reader was contaminated: %q, %v", path, err)
		}
	}
}

func BenchmarkDefaultAppStartup(b *testing.B) {
	root := b.TempDir()
	for key, value := range map[string]string{"HOME": root, "USERPROFILE": root, "XDG_CONFIG_HOME": filepath.Join(root, "config"), "XDG_DATA_HOME": filepath.Join(root, "data"), "APPDATA": filepath.Join(root, "roaming"), "LOCALAPPDATA": filepath.Join(root, "local"), "AIGW_SECRET_BACKEND": "env"} {
		b.Setenv(key, value)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := NewDefault(); err != nil {
			b.Fatal(err)
		}
	}
}
