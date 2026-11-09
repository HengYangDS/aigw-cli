//go:build darwin

package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
		locked func(folder string) []string
		want   string
	}{
		{"the file", func(folder string) []string { return []string{filepath.Join(folder, "config.toml")} }, "~/work/config.toml"},
		{"the folder", func(folder string) []string { return []string{folder} }, "~/work"},
		{"the file and the folder", func(folder string) []string { return []string{filepath.Join(folder, "config.toml"), folder} }, "~/work/config.toml and ~/work"},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := projectedClients[codex.ID](t)
			target := codexTargetIn(t, client, filepath.Join(client.app.Host.Home, "work", "config.toml"))
			locked := test.locked(filepath.Dir(target))
			for _, entry := range locked {
				if err := unix.Chflags(entry, unix.UF_IMMUTABLE); err != nil {
					t.Skipf("the uchg flag cannot be set here: %v", err)
				}
			}
			t.Cleanup(func() {
				for _, entry := range locked {
					_ = unix.Chflags(entry, 0)
				}
			})
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
			want := "Clear the lock flag, such as macOS's uchg or Linux's immutable attribute, on " + test.want + ", then run the same command again."
			if failure.NextDetail != want || failure.Error != "Codex could not write ~/work/config.toml: operation not permitted" {
				t.Fatalf("the failure = %+v\nwant fix %q", failure, want)
			}
			// One round of following the fix clears every flag that it names.
			for _, entry := range locked {
				if err := unix.Chflags(entry, 0); err != nil {
					t.Fatal(err)
				}
			}
			client.output.Reset()
			if err := cli.Execute(client.app, edit); err != nil {
				t.Fatalf("the edit after its fix: %v\n%s", err, client.output)
			}
		})
	}
}

// The Codex target may be a link to a file elsewhere, as dotfiles keep it:
// the flag is read on the file that the link leads to, named inside home, and
// clearing it lets the same edit run.
func TestALockFlagOnTheFileThatALinkLeadsTo(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("the superuser is not refused by the uchg flag")
	}
	client := projectedClients[codex.ID](t)
	target := codexTargetIn(t, client, filepath.Join(client.app.Host.Home, "work", "config.toml"))
	dotfile := filepath.Join(client.app.Host.Home, "dotfiles", "config.toml")
	if err := os.MkdirAll(filepath.Dir(dotfile), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(target, dotfile); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(dotfile, target); err != nil {
		t.Skipf("symbolic links are unavailable: %v", err)
	}
	if err := unix.Chflags(dotfile, unix.UF_IMMUTABLE); err != nil {
		t.Skipf("the uchg flag cannot be set here: %v", err)
	}
	t.Cleanup(func() { _ = unix.Chflags(dotfile, 0) })
	edit := []string{"client", "edit", codex.ID, "--request-retries", "3", "--json"}
	client.output.Reset()
	if err := cli.Execute(client.app, edit); err == nil {
		t.Fatal("the edit passed with the linked file locked")
	}
	var failure struct {
		NextDetail string `json:"next_detail"`
	}
	if err := json.Unmarshal(client.output.Bytes(), &failure); err != nil {
		t.Fatalf("%v\n%s", err, client.output)
	}
	if want := "Clear the lock flag, such as macOS's uchg or Linux's immutable attribute, on ~/dotfiles/config.toml (which ~/work/config.toml leads to), then run the same command again."; failure.NextDetail != want {
		t.Fatalf("the fix = %q\nwant      %q", failure.NextDetail, want)
	}
	if err := unix.Chflags(dotfile, 0); err != nil {
		t.Fatal(err)
	}
	client.output.Reset()
	if err := cli.Execute(client.app, edit); err != nil {
		t.Fatalf("the edit after its fix: %v\n%s", err, client.output)
	}
}

// A write that goes through a temporary file and a rename names its intended
// target and the folder that holds it, never the temporary file: AIGW's Token
// folder locked, rotate names that folder, and clearing its flag lets the
// rotation run.
func TestALockedTokenFolderNamesTheFolderNotAStagingFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("the superuser is not refused by the uchg flag")
	}
	client := newBlockedStoreClient(t)
	tokens := filepath.Join(client.secrets, "tokens")
	if err := unix.Chflags(tokens, unix.UF_IMMUTABLE); err != nil {
		t.Skipf("the uchg flag cannot be set here: %v", err)
	}
	t.Cleanup(func() { _ = unix.Chflags(tokens, 0) })
	client.app.In = strings.NewReader("fixture-token-2\n")
	rotate := []string{"provider", "rotate", "gateway", "--token-stdin", "--json"}
	if got, want := client.next(t, rotate), "Clear the lock flag, such as macOS's uchg or Linux's immutable attribute, on ~/aigw/secrets/tokens, then run the same command again."; got != want {
		t.Fatalf("rotate names %q\nwant         %q", got, want)
	}
	if strings.Contains(client.output.String(), ".token-") {
		t.Fatalf("rotate names a staging file: %s", client.output)
	}
	if err := unix.Chflags(tokens, 0); err != nil {
		t.Fatal(err)
	}
	client.app.In = strings.NewReader("fixture-token-2\n")
	client.output.Reset()
	if err := cli.Execute(client.app, rotate); err != nil {
		t.Fatalf("rotate after its fix: %v\n%s", err, client.output)
	}
}

// An install into a locked folder names that folder, never the temporary file
// that the write goes through, whose name changes with every run, and
// clearing the folder's flag lets the same install run.
func TestAnInstallIntoALockedFolderNamesTheFolder(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("the superuser is not refused by the uchg flag")
	}
	app, output, _, _, _ := testApp(t, "")
	app.Executable = filepath.Join(t.TempDir(), "download", "aigw")
	writeFile(t, app.Executable, []byte("portable"), 0o755)
	folder := filepath.Join(app.Host.Home, "bin")
	if err := os.MkdirAll(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := unix.Chflags(folder, unix.UF_IMMUTABLE); err != nil {
		t.Skipf("the uchg flag cannot be set here: %v", err)
	}
	t.Cleanup(func() { _ = unix.Chflags(folder, 0) })
	install := []string{"install", "--path", filepath.Join(folder, "aigw"), "--json"}
	if err := cli.Execute(app, install); err == nil {
		t.Fatal("the install passed into a locked folder")
	}
	var failure struct {
		Error      string `json:"error"`
		NextDetail string `json:"next_detail"`
	}
	if err := json.Unmarshal(output.Bytes(), &failure); err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
	if want := "Clear the lock flag, such as macOS's uchg or Linux's immutable attribute, on ~/bin, then run the same command again."; failure.NextDetail != want ||
		failure.Error != "AIGW cannot use ~/bin/aigw: operation not permitted" || strings.Contains(output.String(), ".aigw-write-") {
		t.Fatalf("the failure = %+v", failure)
	}
	if err := unix.Chflags(folder, 0); err != nil {
		t.Fatal(err)
	}
	output.Reset()
	if err := cli.Execute(app, install); err != nil {
		t.Fatalf("the install after its fix: %v\n%s", err, output)
	}
}
