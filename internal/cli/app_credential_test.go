package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
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
	if app.CredentialPath != want {
		t.Fatalf("credential entrypoint = %q, want exact executable identity %q", app.CredentialPath, want)
	}
	if _, err := os.Lstat(want); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("app construction wrote a credential entrypoint: %v", err)
	}
}
