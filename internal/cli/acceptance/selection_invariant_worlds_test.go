package cli_test

import (
	"os"
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/domain"
)

// run executes one command of a world's preparation, which must succeed.
func (w selectionWorld) run(t *testing.T, args ...string) {
	t.Helper()
	w.output.Reset()
	if err := cli.Execute(w.app, args); err != nil {
		t.Fatalf("prepare with %v: %v\n%s", args, err, w.output)
	}
}

// codexWorld is Codex on count files that show gateway/gpt-sol with its
// record, after one earlier change that a rollback can undo.
func codexWorld(t *testing.T, count int) selectionWorld {
	t.Helper()
	app, output, targets := recCodexFiles(t, count)
	world := selectionWorld{
		app: app, output: output, client: codex.ID, account: "gateway", files: targets,
		shows: func(t *testing.T, path string) string {
			t.Helper()
			_, value, _ := strings.Cut(recCodexModel(t, path), `"`)
			value, _, _ = strings.Cut(value, `"`)
			return value
		},
		storedAs: func(runtime domain.Runtime) string { return runtime.UpstreamModel },
	}
	world.follows = func(t *testing.T, path string) bool {
		t.Helper()
		return recCodexRecorded(t, path) == world.shows(t, path)
	}
	world.run(t, "provider", "edit", "spare", "--label", "Earlier")
	world.restored = "gpt-sol"
	return world
}

// codexRestoring is codexWorld whose last change selected gpt-sol in place of
// earlier, so that a rollback restores earlier.
func codexRestoring(t *testing.T, count int, earlier string) selectionWorld {
	t.Helper()
	world := codexWorld(t, count)
	world.run(t, "use", "codex", "gateway/"+earlier)
	world.run(t, "use", "codex", "gateway/gpt-sol")
	world.restored = earlier
	return world
}

// codexPending leaves Codex with a stored selection that waits for its Token:
// AIGW wrote gpt-terra, the Token is lost, and a rollback restores gpt-sol,
// which cannot be written. Every file and its record show gpt-terra.
func codexPending(t *testing.T, count int) selectionWorld {
	t.Helper()
	world := codexWorld(t, count)
	world.run(t, "use", "codex", "gateway/gpt-terra")
	world.setToken(t, false)
	world.run(t, "config", "rollback", "--last-change", "--json")
	if got := selection(t, world.app, codex.ID).Model.String(); got != "gateway/gpt-sol" {
		t.Fatalf("the rollback saved %s", got)
	}
	world.restored = ""
	return world
}

// chosenBeforeARollback is a state in which the first file chose gpt-terra
// and a rollback restores earlier; without names the other file to remove.
func chosenBeforeARollback(count int, earlier string, without bool) func(t *testing.T) selectionWorld {
	return func(t *testing.T) selectionWorld {
		world := codexRestoring(t, count, earlier)
		recPick(t, world.files[0], "gpt-terra")
		if without {
			if err := os.Remove(world.files[1]); err != nil {
				t.Fatal(err)
			}
		}
		return world
	}
}

func codexCommands() []selectionCommand {
	return []selectionCommand{
		{name: "another provider", args: []string{"provider", "edit", "spare", "--label", "Other", "--json"}},
		{name: "sync", args: []string{"sync", "--json"}},
		{name: "own setting", args: []string{"client", "edit", "codex", "--request-retries", "2", "--json"}},
		{name: "select a third model", args: []string{"use", "codex", "gateway/gpt-luna", "--json"}, explicit: true},
		{name: "select the stored model", args: []string{"use", "codex", "gateway/gpt-sol", "--json"}, explicit: true},
		{name: "rollback", args: []string{"config", "rollback", "--last-change", "--json"}, explicit: true, restores: true},
		{name: "disable", args: []string{"client", "disable", "codex", "--json"}, withdraws: true},
		{name: "sync preview", args: []string{"sync", "--dry-run", "--json"}, preview: true},
		{name: "own setting preview", args: []string{"client", "edit", "codex", "--request-retries", "2", "--dry-run", "--json"}, preview: true},
		{name: "select preview", args: []string{"use", "codex", "gateway/gpt-luna", "--dry-run", "--json"}, preview: true},
		{name: "rollback preview", args: []string{"config", "rollback", "--last-change", "--dry-run", "--json"}, preview: true},
		{name: "disable preview", args: []string{"client", "disable", "codex", "--dry-run", "--json"}, preview: true},
	}
}

func codexOneFileMatrix() selectionMatrix {
	return selectionMatrix{client: "codex one file", commands: codexCommands(), states: map[string]func(t *testing.T) selectionWorld{
		plainState: func(t *testing.T) selectionWorld { return codexWorld(t, 1) },
		"chosen": func(t *testing.T) selectionWorld {
			world := codexWorld(t, 1)
			recPick(t, world.files[0], "gpt-terra")
			return world
		},
		"shows the stored model beside an earlier record": func(t *testing.T) selectionWorld {
			world := codexPending(t, 1)
			recPick(t, world.files[0], "gpt-sol")
			return world
		},
		"waits for its Token":                        func(t *testing.T) selectionWorld { return codexPending(t, 1) },
		"chosen, a rollback restores a third model":  chosenBeforeARollback(1, "gpt-luna", false),
		"chosen, a rollback restores the chosen one": chosenBeforeARollback(1, "gpt-terra", false),
	}}
}

func codexTwoFilesMatrix() selectionMatrix {
	chosenInFirst := func(t *testing.T) selectionWorld {
		world := codexWorld(t, 2)
		recPick(t, world.files[0], "gpt-terra")
		return world
	}
	removed := func(t *testing.T, path string) {
		t.Helper()
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	return selectionMatrix{client: "codex two files", commands: codexCommands(), states: map[string]func(t *testing.T) selectionWorld{
		plainState:      func(t *testing.T) selectionWorld { return codexWorld(t, 2) },
		"chosen in one": chosenInFirst,
		"chosen in both": func(t *testing.T) selectionWorld {
			w := chosenInFirst(t)
			recPick(t, w.files[1], "gpt-terra")
			return w
		},
		"different picks": func(t *testing.T) selectionWorld { w := chosenInFirst(t); recPick(t, w.files[1], "gpt-luna"); return w },
		"chosen, the other file missing": func(t *testing.T) selectionWorld {
			world := chosenInFirst(t)
			removed(t, world.files[1])
			return world
		},
		"chosen, the other record missing": func(t *testing.T) selectionWorld {
			world := chosenInFirst(t)
			if world.recordOf(t, world.files[1]) == nil {
				t.Fatal("the second file has no record to remove")
			}
			for _, path := range []string{world.files[1] + adapter.StateSuffix} {
				_ = os.Remove(path)
			}
			return world
		},
		"recorded, the other file missing": func(t *testing.T) selectionWorld {
			world := codexWorld(t, 2)
			removed(t, world.files[1])
			return world
		},
		"one shows the stored model beside an earlier record": func(t *testing.T) selectionWorld {
			world := codexPending(t, 2)
			recPick(t, world.files[0], "gpt-sol")
			return world
		},
		"chosen, a rollback restores a third model":                          chosenBeforeARollback(2, "gpt-luna", false),
		"chosen, a rollback restores the chosen one":                         chosenBeforeARollback(2, "gpt-terra", false),
		"chosen, the other file missing, a rollback restores a third model":  chosenBeforeARollback(2, "gpt-luna", true),
		"chosen, the other file missing, a rollback restores the chosen one": chosenBeforeARollback(2, "gpt-terra", true),
	}}
}

// hermesWorld is Hermes on one file that shows dmxapi/gpt-6.2-sol on the
// Responses protocol with its record, after one earlier change.
func hermesWorld(t *testing.T, app *cli.App, target string) selectionWorld {
	t.Helper()
	return selectionWorld{
		app: app, client: hermes.ID, account: "dmxapi", files: []string{target},
		shows: func(t *testing.T, path string) string {
			t.Helper()
			provider, model := hermesProjectedSelection(t, path)
			return provider + "/" + model
		},
		follows: func(t *testing.T, path string) bool {
			t.Helper()
			_, behind, err := hermes.ObserveSelection(path)
			return err == nil && !behind
		},
		storedAs: func(runtime domain.Runtime) string {
			return "aigw-" + runtime.ProviderID + "-" + runtime.Protocol.Name() + "/" + runtime.UpstreamModel
		},
	}
}

func hermesMatrix() selectionMatrix {
	plain := func(t *testing.T) selectionWorld {
		app, output, _, target := shippedHermesAppWith(t, true)
		world := hermesWorld(t, app, target)
		world.output = output
		world.run(t, "provider", "edit", "ucloud", "--label", "Earlier")
		world.restored = "aigw-dmxapi-responses/gpt-6.2-sol"
		return world
	}
	// restoring leaves a last change that selected the stored model in place
	// of another, which a rollback restores, and then the choice in the file.
	restoring := func(model, protocol, shown string) func(t *testing.T) selectionWorld {
		return func(t *testing.T) selectionWorld {
			world := plain(t)
			world.run(t, "use", "hermes", model, "--protocol", protocol)
			world.run(t, "use", "hermes", "dmxapi/gpt-6.2-sol", "--protocol", "responses")
			world.restored = shown
			hermesNativeSelection(t, world.files[0], "aigw-dmxapi-anthropic", "claude-opus-5-5", nil)
			return world
		}
	}
	pending := func(t *testing.T) selectionWorld {
		app, output, target := pendingHermes(t)
		world := hermesWorld(t, app, target)
		world.output = output
		return world
	}
	return selectionMatrix{
		client: "hermes",
		states: map[string]func(t *testing.T) selectionWorld{
			plainState: plain,
			"chosen on another protocol": func(t *testing.T) selectionWorld {
				world := plain(t)
				hermesNativeSelection(t, world.files[0], "aigw-dmxapi-anthropic", "claude-opus-5-5", nil)
				return world
			},
			"chosen of another provider": func(t *testing.T) selectionWorld {
				world := plain(t)
				hermesNativeSelection(t, world.files[0], "aigw-ucloud-anthropic", "claude-opus-5-5", nil)
				return world
			},
			"shows the stored model beside an earlier record": func(t *testing.T) selectionWorld {
				world := pending(t)
				hermesNativeSelection(t, world.files[0], "aigw-dmxapi-responses", "gpt-6.2-sol", nil)
				return world
			},
			"waits for its Token":                        pending,
			"chosen, a rollback restores a third model":  restoring("ucloud/claude-opus-5-5", "anthropic", "aigw-ucloud-anthropic/claude-opus-5-5"),
			"chosen, a rollback restores the chosen one": restoring("dmxapi/claude-opus-5-5", "anthropic", "aigw-dmxapi-anthropic/claude-opus-5-5"),
		},
		commands: []selectionCommand{
			{name: "another provider", args: []string{"provider", "edit", "ucloud", "--label", "Other", "--json"}},
			{name: "sync", args: []string{"sync", "--json"}},
			{name: "own setting", args: []string{"client", "edit", "hermes", "--via", "http://127.0.0.1:8792", "--json"}},
			{name: "another client", args: []string{"use", "claude", "ucloud/claude-opus-5-5", "--json"}},
			{name: "select a third model", args: []string{"use", "hermes", "ucloud/claude-opus-5-5", "--protocol", "anthropic", "--json"}, explicit: true},
			{name: "select the stored model", args: []string{"use", "hermes", "dmxapi/gpt-6.2-sol", "--protocol", "responses", "--json"}, explicit: true},
			{name: "rollback", args: []string{"config", "rollback", "--last-change", "--json"}, explicit: true, restores: true},
			{name: "disable", args: []string{"client", "disable", "hermes", "--json"}, withdraws: true},
			{name: "sync preview", args: []string{"sync", "--dry-run", "--json"}, preview: true},
			{name: "own setting preview", args: []string{"client", "edit", "hermes", "--via", "http://127.0.0.1:8792", "--dry-run", "--json"}, preview: true},
			{name: "select preview", args: []string{"use", "hermes", "ucloud/claude-opus-5-5", "--protocol", "anthropic", "--dry-run", "--json"}, preview: true},
			{name: "rollback preview", args: []string{"config", "rollback", "--last-change", "--dry-run", "--json"}, preview: true},
			{name: "disable preview", args: []string{"client", "disable", "hermes", "--dry-run", "--json"}, preview: true},
		},
	}
}
