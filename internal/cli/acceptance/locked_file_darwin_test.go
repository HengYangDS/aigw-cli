//go:build darwin

package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli"

	"golang.org/x/sys/unix"
)

// A write that macOS's uchg flag refuses names the entry that carries the
// flag, the Codex file or the folder holding it, whose lock refuses the
// rename that replaces the file; clearing that flag lets the same edit run.
// A sandbox, which raises the same EPERM with no flag, is covered by the
// binary's own run under sandbox-exec, which a test process cannot enter.
func TestALockFlagNamesTheEntryThatCarriesIt(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("the superuser is not refused by the uchg flag")
	}
	for _, test := range []struct {
		name   string
		locked func(folder string) string
		want   string
	}{
		{"the file", func(folder string) string { return filepath.Join(folder, "config.toml") }, "~/work/config.toml"},
		{"the folder", func(folder string) string { return folder }, "~/work"},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := projectedClients[codex.ID](t)
			target := codexTargetIn(t, client, filepath.Join(client.app.Host.Home, "work", "config.toml"))
			locked := test.locked(filepath.Dir(target))
			if err := unix.Chflags(locked, unix.UF_IMMUTABLE); err != nil {
				t.Skipf("the uchg flag cannot be set here: %v", err)
			}
			t.Cleanup(func() { _ = unix.Chflags(locked, 0) })
			edit := []string{"client", "edit", codex.ID, "--request-retries", "3", "--json"}
			client.output.Reset()
			if err := cli.Execute(client.app, edit); err == nil {
				t.Fatalf("the edit passed with %s locked", test.name)
			}
			var failure struct {
				Error      string `json:"error"`
				NextDetail string `json:"next_detail"`
			}
			if err := json.Unmarshal(client.output.Bytes(), &failure); err != nil {
				t.Fatalf("%v\n%s", err, client.output)
			}
			want := "Clear the flag that locks " + test.want + " (such as macOS's uchg or Linux's immutable attribute), then run the same command again."
			if failure.NextDetail != want || failure.Error != "Codex could not write ~/work/config.toml: operation not permitted" {
				t.Fatalf("the failure = %+v\nwant fix %q", failure, want)
			}
			if err := unix.Chflags(locked, 0); err != nil {
				t.Fatal(err)
			}
			client.output.Reset()
			if err := cli.Execute(client.app, edit); err != nil {
				t.Fatalf("the edit after its fix: %v\n%s", err, client.output)
			}
		})
	}
}
