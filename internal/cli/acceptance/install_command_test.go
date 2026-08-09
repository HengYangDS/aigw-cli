package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPortableInstallAndUninstallCommandsOwnOnlyProgramFiles(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	app.Executable = filepath.Join(t.TempDir(), "download", "aigw")
	app.InstallTarget = filepath.Join(t.TempDir(), "bin", "aigw")
	if err := os.MkdirAll(filepath.Dir(app.Executable), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(app.Executable, []byte("portable"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := execute(t, app, "install"); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(app.InstallTarget); err != nil || string(data) != "portable" {
		t.Fatalf("installed program = %q, %v", data, err)
	}
	if !strings.Contains(out.String(), "aigw setup") {
		t.Fatalf("install output = %q", out.String())
	}
	out.Reset()
	if err := execute(t, app, "uninstall", "--target", app.InstallTarget); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(app.InstallTarget); !os.IsNotExist(err) {
		t.Fatalf("installed program remains: %v", err)
	}
	if !strings.Contains(out.String(), "Configuration and credential-store secrets were preserved") {
		t.Fatalf("uninstall output = %q", out.String())
	}
}
