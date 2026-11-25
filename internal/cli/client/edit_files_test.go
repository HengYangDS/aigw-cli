package client

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/terminal"
)

// An enabled client's configuration files move while its provider has no
// Token that AIGW can use, or the store cannot observe it: AIGW withdraws its
// settings from the file that the edit drops, which needs no Token, and the
// file that the client moves to waits. The edit's next step is the one that
// status names for the waiting client: storing the Token, setting the
// environment backend's variable, or the local check of a store that cannot
// tell.
func TestEditWithdrawsTheDroppedFileWhileTheTokenIsMissing(t *testing.T) {
	for _, test := range []struct {
		name  string
		store secret.Store
		next  string
	}{
		{"a missing Token", secret.NewMemoryStore(), "run `aigw provider rotate gateway`"},
		{"the environment backend", environmentBackend(func(string) string { return "" }), "set environment variable " + secret.EnvironmentKey("gateway")},
		{"a store that cannot tell", unavailableAdapterSecretStore{}, "aigw check --depth local"},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime, out, target, _ := enabledCodex(t)
			moved := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(moved, []byte("model = \"other\"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			// As in the product, Codex reads its Token through AIGW's
			// credential reader, so the Token decides whether its files are
			// written.
			reader := filepath.Join(t.TempDir(), "aigw-credential")
			runtime.Secrets, runtime.ResolveCredentialPath = test.store, func() (string, error) { return reader, nil }
			document := edited(t, runtime, out, codex.ID, "--target", moved)
			if !document.Changed || !slices.Equal(document.Targets, []string{moved}) || document.NextAction+document.NextDetail != test.next {
				t.Fatalf("the edit = %#v; want Codex on the new file and the next step %q", document, test.next)
			}
			if strings.Contains(readTarget(t, target), "AIGW managed provider") || readTarget(t, moved) != "model = \"other\"\n" {
				t.Fatalf("the edit left AIGW's settings in the dropped file:\n%s\nor wrote the file that waits:\n%s", readTarget(t, target), readTarget(t, moved))
			}
		})
	}
}

// An edit of an enabled client whose files AIGW has not written names the step
// that writes them, as status does: installing the client first, aigw sync
// once it is installed, and storing its provider's Token when that is missing.
func TestEditOfAClientWithoutFilesNamesTheStepThatWritesThem(t *testing.T) {
	cfg := adapterConfig()
	cfg.SetClientActivation(codex.ID, true, "", nil)
	runtime, out, secretStore, _ := adapterRuntime(t, cfg)
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		budget       string
		found        map[string]string
		removeToken  bool
		action, step string
	}{
		{budget: "1", step: "Install Codex if needed, then run `aigw sync`"},
		{budget: "2", found: map[string]string{codex.ID: "/opt/codex"}, action: "aigw sync"},
		{budget: "3", found: map[string]string{codex.ID: "/opt/codex"}, removeToken: true, step: "run `aigw provider rotate gateway`"},
	} {
		runtime.Discovery = adapterDiscovery{result: discovery.Result{Executables: test.found}}
		if test.removeToken {
			if err := secretStore.Delete("gateway"); err != nil {
				t.Fatal(err)
			}
		}
		document := edited(t, runtime, out, codex.ID, "--request-retries", test.budget)
		if !document.Changed || len(document.ClientFiles) != 0 || document.NextAction != test.action || document.NextDetail != test.step {
			t.Fatalf("edit with budget %s = %#v", test.budget, document)
		}
	}
}

// The review's reproduction: Codex's configuration file is deleted with its
// folder, record and all. Moving Codex to another file succeeds and reports
// the old file as absent, disable then succeeds, and sync, while Codex keeps
// a file that is gone, refuses with the move as its fix.
func TestEditMovesCodexAwayFromADeletedConfigurationFile(t *testing.T) {
	runtime, out, target, _ := enabledCodex(t)
	if err := os.RemoveAll(filepath.Dir(target)); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(moved, []byte("model = \"other\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	document := edited(t, runtime, out, codex.ID, "--target", moved)
	actions := map[string]string{}
	for _, file := range document.ClientFiles {
		actions[filepath.Base(filepath.Dir(file.File))+"/"+filepath.Base(file.File)] = file.Action
	}
	if !document.Changed || len(document.Targets) != 1 || document.Targets[0] != moved || len(document.ClientFiles) != 2 ||
		!strings.Contains(fmt.Sprint(document.ClientFiles), "absent") || !strings.Contains(readTarget(t, moved), "AIGW managed provider") {
		t.Fatalf("the move from a deleted file = %#v, actions %v", document, actions)
	}
	if err := executeAdapter(t, runtime, "disable", codex.ID); err != nil {
		t.Fatalf("disable after the move = %v", err)
	}
}

// The review's reproduction: Codex moves to two configuration files, and the
// second one's folder is read-only. The edit fails at that file, names it with
// its operating-system error, says that the first file was restored, and asks
// for a writable file and folder; the configuration and every file stay as
// they were. A folder's permissions do not bind root.
func TestAFailedWriteNamesTheFileAndWhatWasRestored(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("folder permissions do not refuse this user's writes")
	}
	runtime, _, secretStore, _ := adapterRuntime(t, adapterConfig())
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	home := runtime.Host.Home
	files := map[string]string{}
	for _, name := range []string{"a", "b", "o"} {
		files[name] = filepath.Join(home, name, "config.toml")
		if err := os.MkdirAll(filepath.Dir(files[name]), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(files[name], []byte("model = \"original\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := executeAdapter(t, runtime, "enable", codex.ID, "--executable", "/opt/codex", "--target", files["o"]); err != nil {
		t.Fatal(err)
	}
	before, err := runtime.Config.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	projected := readTarget(t, files["o"])
	if err := os.Chmod(filepath.Dir(files["b"]), 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Dir(files["b"]), 0o700) })
	err = executeAdapter(t, runtime, "edit", codex.ID, "--target", files["a"], "--target", files["b"])
	want := terminal.Problem{
		Title:    "Codex could not write ~/b/config.toml: permission denied",
		Evidence: "~/a/config.toml.aigw-state.json was restored. ~/a/config.toml was restored. ~/b/config.toml is unchanged.",
		Impact:   "The configuration was restored.",
		Fix:      "Make ~/b/config.toml and its folder readable and writable, then run the same command again.",
	}
	if problem := terminal.ProblemFor(err); problem != want {
		t.Fatalf("a failed write = %#v\nwant %#v", problem, want)
	}
	if after, err := runtime.Config.CaptureSnapshot(); err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("the failed edit changed the configuration: %v", err)
	}
	if readTarget(t, files["a"]) != "model = \"original\"\n" || readTarget(t, files["b"]) != "model = \"original\"\n" || readTarget(t, files["o"]) != projected {
		t.Fatal("the failed edit left a client file changed")
	}
	if _, err := os.Stat(files["a"] + ".aigw-state.json"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the failed edit left AIGW's record beside ~/a/config.toml: %v", err)
	}
}

// existsCounter is a credential store that counts how often a command asks
// whether a provider's Token is stored.
type existsCounter struct {
	secret.Store
	asked int
}

func (store *existsCounter) Exists(provider string) (bool, error) {
	store.asked++
	return store.Store.Exists(provider)
}

// A preview and an edit that changes nothing name no step that writes a
// client's files, so neither asks about the provider's Token to find one; an
// applied edit of a client whose files wait does.
func TestOnlyAnAppliedEditObservesTheStepThatWritesTheFiles(t *testing.T) {
	cfg := adapterConfig()
	cfg.SetClientActivation(codex.ID, true, "", nil)
	runtime, out, secretStore, _ := adapterRuntime(t, cfg)
	counter := &existsCounter{Store: secretStore}
	runtime.Secrets = counter
	for _, args := range [][]string{{codex.ID, "--request-retries", "1", "--dry-run"}, {codex.ID, "--direct"}} {
		if edited(t, runtime, out, args...); counter.asked != 0 {
			t.Fatalf("%q asked about the Token %d times", args, counter.asked)
		}
	}
	if document := edited(t, runtime, out, codex.ID, "--request-retries", "1"); counter.asked == 0 || document.NextDetail == "" {
		t.Fatalf("an applied edit = %#v, asked %d times", document, counter.asked)
	}
}

// environmentBackend is the read-only environment backend that getenv
// supplies, as an invocation selects it.
func environmentBackend(getenv func(string) string) secret.Store {
	store, err := secret.Select(secret.Selection{Backend: "env", Getenv: getenv})
	if err != nil {
		panic(err)
	}
	return store
}
