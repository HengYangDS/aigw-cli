package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli"
)

// nextStep runs command and returns its document's next step, whichever of
// next_action and next_detail holds it.
func nextStep(t *testing.T, client projectedClient, command []string) string {
	t.Helper()
	client.output.Reset()
	_ = cli.Execute(client.app, command)
	var document struct {
		NextAction string `json:"next_action"`
		NextDetail string `json:"next_detail"`
	}
	if err := json.Unmarshal(client.output.Bytes(), &document); err != nil {
		t.Fatalf("%v: %v\n%s", command, err, client.output)
	}
	if strings.Contains(client.output.String(), client.app.Host.Home) {
		t.Fatalf("%v names the private home: %s", command, client.output)
	}
	return document.NextAction + document.NextDetail
}

// codexTargetIn moves the Codex client of client to a file at path, which a
// sync projects, and returns it.
func codexTargetIn(t *testing.T, client projectedClient, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, []byte("approval_policy = \"never\"\n"), 0o600)
	client.output.Reset()
	if err := cli.Execute(client.app, []string{"client", "edit", codex.ID, "--target", path}); err != nil {
		t.Fatalf("the edit with its folder there: %v\n%s", err, client.output)
	}
	return path
}

// Each surface names a Codex target inside home as ~/… and the one fix of its
// path's condition: a check completes it with `aigw sync`, whose drift would
// otherwise be its next failure, and a command runs again. Followed literally,
// once, each fix lets the check pass after its sync and the command pass.
func TestACodexTargetsFixRunsOnEverySurface(t *testing.T) {
	for _, test := range []struct {
		name   string
		break_ func(t *testing.T, folder string)
		check  string
		run    string
		follow func(t *testing.T, folder string)
	}{
		{
			name:   "the folder is gone",
			break_: func(t *testing.T, folder string) { removeAll(t, folder) },
			check:  "Create ~/work/config.toml and the folder that holds it, then run `aigw sync`.",
			run:    "Create ~/work/config.toml and the folder that holds it, then run the same command again.",
			follow: func(t *testing.T, folder string) { createTarget(t, folder) },
		},
		{
			name: "a file stands where the folder belongs",
			break_: func(t *testing.T, folder string) {
				removeAll(t, folder)
				writeFile(t, folder, []byte("x"), 0o600)
			},
			check: "Move the file ~/work, then create ~/work/config.toml and the folder that holds it, then run `aigw sync`.",
			run:   "Move the file ~/work, then create ~/work/config.toml and the folder that holds it, then run the same command again.",
			follow: func(t *testing.T, folder string) {
				if err := os.Rename(folder, folder+".moved"); err != nil {
					t.Fatal(err)
				}
				createTarget(t, folder)
			},
		},
		{
			name: "the folder is a link that leads nowhere",
			break_: func(t *testing.T, folder string) {
				removeAll(t, folder)
				if err := os.Symlink(folder+".nowhere", folder); err != nil {
					t.Skipf("symbolic links are unavailable: %v", err)
				}
			},
			check: "Remove the link ~/work, then create ~/work/config.toml and the folder that holds it, then run `aigw sync`.",
			run:   "Remove the link ~/work, then create ~/work/config.toml and the folder that holds it, then run the same command again.",
			follow: func(t *testing.T, folder string) {
				if err := os.Remove(folder); err != nil {
					t.Fatal(err)
				}
				createTarget(t, folder)
			},
		},
	} {
		for _, surface := range []struct {
			command []string
			check   bool
		}{
			{[]string{"check", codex.ID, "--depth", "local", "--json"}, true},
			{[]string{"status", "--json"}, true},
			{[]string{"sync", "--json"}, false},
			{[]string{"use", codex.ID, "gateway/gpt-sol", "--json"}, false},
		} {
			t.Run(test.name+"/"+surface.command[0], func(t *testing.T) {
				client := projectedClients[codex.ID](t)
				target := codexTargetIn(t, client, filepath.Join(client.app.Host.Home, "work", "config.toml"))
				folder := filepath.Dir(target)
				test.break_(t, folder)
				want := test.run
				if surface.check {
					want = test.check
				}
				if got := nextStep(t, client, surface.command); got != want {
					t.Fatalf("%v names %q\nwant         %q", surface.command, got, want)
				}
				test.follow(t, folder)
				if surface.check {
					client.output.Reset()
					if err := cli.Execute(client.app, []string{"sync"}); err != nil {
						t.Fatalf("the sync that the check named: %v\n%s", err, client.output)
					}
				}
				client.output.Reset()
				if err := cli.Execute(client.app, surface.command); err != nil {
					t.Fatalf("%v after its fix: %v\n%s", surface.command, err, client.output)
				}
			})
		}
	}
}

// removeAll removes path and everything in it.
func removeAll(t *testing.T, path string) {
	t.Helper()
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}
}

// createTarget creates the Codex configuration file in folder, and the folder,
// as the operator does.
func createTarget(t *testing.T, folder string) {
	t.Helper()
	if err := os.MkdirAll(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(folder, "config.toml"), nil, 0o600)
}

// A Codex target outside the home directory is named as the operator gave it
// by a command, and the folder alone does not let the same edit pass, since
// AIGW creates no Codex file.
func TestACodexTargetInAMissingFolderNamesTheFolderAndFile(t *testing.T) {
	client := projectedClients[codex.ID](t)
	gone := filepath.Join(t.TempDir(), "gone", "config.toml")
	want := "Create " + gone + " and the folder that holds it, then run the same command again."
	for _, command := range [][]string{
		{"client", "edit", codex.ID, "--target", gone, "--dry-run", "--json"},
		{"client", "edit", codex.ID, "--target", gone, "--json"},
	} {
		client.output.Reset()
		if err := cli.Execute(client.app, command); err == nil {
			t.Fatalf("%v passed", command)
		}
		var failure struct {
			NextAction string `json:"next_action"`
			NextDetail string `json:"next_detail"`
		}
		if err := json.Unmarshal(client.output.Bytes(), &failure); err != nil {
			t.Fatalf("%v: %v\n%s", command, err, client.output)
		}
		if failure.NextDetail != want || failure.NextAction != "" {
			t.Fatalf("%v names %+v, want %q", command, failure, want)
		}
	}
	if err := os.MkdirAll(filepath.Dir(gone), 0o700); err != nil {
		t.Fatal(err)
	}
	client.output.Reset()
	if err := cli.Execute(client.app, []string{"client", "edit", codex.ID, "--target", gone}); err == nil {
		t.Fatalf("the edit after only the folder exists passed:\n%s", client.output)
	}
	writeFile(t, gone, nil, 0o600)
	client.output.Reset()
	if err := cli.Execute(client.app, []string{"client", "edit", codex.ID, "--target", gone}); err != nil {
		t.Fatalf("the edit after the folder and file exist: %v\n%s", err, client.output)
	}
}
