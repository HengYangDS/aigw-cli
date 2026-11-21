package cli_test

import (
	"bytes"
	"testing"

	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/domain"
)

// hermesForwardedTo is the local address that the forwarding state forwards
// to, and that the command "own setting" selects.
const hermesForwardedTo = "http://127.0.0.1:8792"

// hermesWorld is Hermes on one file whose stored selection is a model of
// dmxapi.
func hermesWorld(t *testing.T, app *cli.App, target string) selectionWorld {
	t.Helper()
	world := selectionWorld{
		app: app, client: hermes.ID, account: "dmxapi", files: []string{target},
		shows: func(t *testing.T, path string) string {
			t.Helper()
			provider, model := hermesProjectedSelection(t, path)
			return provider + "/" + model
		},
		storedAs: func(runtime domain.Runtime) string {
			return "aigw-" + runtime.ProviderID + "-" + runtime.Protocol.Name() + "/" + runtime.UpstreamModel
		},
		// Every provider entry that AIGW writes is named aigw-<provider>-<protocol>.
		keepsAIGWsPart: func(file []byte) bool { return bytes.Contains(file, []byte("aigw-")) },
	}
	world.follows = func(t *testing.T, path string) bool {
		t.Helper()
		// Without a record ObserveSelection reports no difference, and
		// nothing follows.
		if world.recordOf(t, path) == nil {
			return false
		}
		_, behind, err := hermes.ObserveSelection(path)
		return err == nil && !behind
	}
	return world
}

// hermesPlain is Hermes on one file that shows dmxapi/gpt-6.2-sol on the
// Responses protocol with its record, after one earlier change.
func hermesPlain(t *testing.T) selectionWorld {
	t.Helper()
	app, output, _, target := shippedHermesAppWith(t, true)
	world := hermesWorld(t, app, target)
	world.output = output
	world.run(t, "provider", "edit", "ucloud", "--label", "Earlier")
	world.restored = "aigw-dmxapi-responses/gpt-6.2-sol"
	return world
}

// hermesPendingWorld is pendingHermes as a world: the stored selection waits
// for its Token, and the file and its record show claude-opus-5-5. The last
// change is the rollback that stored the waiting selection, and the
// configuration that it replaced, which it keeps as the backup, selects
// claude-opus-5-5 on the Anthropic protocol: so the construction fixes what
// the next rollback restores.
func hermesPendingWorld(t *testing.T) selectionWorld {
	t.Helper()
	app, output, target := pendingHermes(t)
	world := hermesWorld(t, app, target)
	world.output = output
	world.restored = "aigw-dmxapi-anthropic/claude-opus-5-5"
	return world
}

// hermesForwarding is hermesPlain whose last change forwards the stored
// selection to a local address, so that a rollback restores the same model
// without the forwarding.
func hermesForwarding(t *testing.T) selectionWorld {
	t.Helper()
	world := hermesPlain(t)
	world.run(t, "client", "edit", "hermes", "--via", hermesForwardedTo)
	if got := selection(t, world.app, hermes.ID).ForwardingEndpoint; got != hermesForwardedTo {
		t.Fatalf("the stored selection forwards to %q", got)
	}
	return world
}

// hermesRestoring leaves a last change that selected the stored model in
// place of another, which a rollback restores, and then a choice in the file.
func hermesRestoring(model, protocol, shown string) func(t *testing.T) selectionWorld {
	return func(t *testing.T) selectionWorld {
		world := hermesPlain(t)
		world.run(t, "use", "hermes", model, "--protocol", protocol)
		world.run(t, "use", "hermes", "dmxapi/gpt-6.2-sol", "--protocol", "responses")
		world.restored = shown
		return world.choosing(t, "dmxapi", "anthropic", "claude-opus-5-5", plainScalars)
	}
}

// withoutForwarding is what `aigw client edit hermes --direct` leaves: the
// stored binding, read from the saved configuration, forwards nowhere.
func withoutForwarding(after selectionSnapshot) string {
	if after.binding.ForwardingEndpoint == "" {
		return ""
	}
	return "the stored selection " + after.binding.Model.String() + " still forwards to " + after.binding.ForwardingEndpoint
}

// nothingToEnd reports that `aigw client edit hermes --direct` asks for what
// is stored already: the stored selection forwards nowhere, and no file of
// the client chose a model, which an edit of the client's own settings would
// save with them.
func nothingToEnd(before selectionSnapshot) bool {
	if before.binding.ForwardingEndpoint != "" {
		return false
	}
	for _, shown := range before.shows {
		if shown != "" && shown != before.stored {
			return false
		}
	}
	return true
}

func hermesCommands() []selectionCommand {
	return []selectionCommand{
		{name: "another provider", args: []string{"provider", "edit", "ucloud", "--label", "Other", "--json"}},
		{name: "sync", args: []string{"sync", "--json"}},
		{name: "own setting", args: []string{"client", "edit", "hermes", "--via", hermesForwardedTo, "--json"}},
		{name: "end forwarding", args: []string{"client", "edit", "hermes", "--direct", "--json"}, leaves: withoutForwarding, idle: nothingToEnd},
		{name: "another client", args: []string{"use", "claude", "ucloud/claude-opus-5-5", "--json"}, stores: "ucloud"},
		{name: "select a third model", args: []string{"use", "hermes", "ucloud/claude-opus-5-5", "--protocol", "anthropic", "--json"}, explicit: true, stores: "ucloud"},
		{name: "select the stored model", args: []string{"use", "hermes", "dmxapi/gpt-6.2-sol", "--protocol", "responses", "--json"}, explicit: true, stores: "dmxapi"},
		{name: "rollback", args: []string{"config", "rollback", "--last-change", "--json"}, explicit: true, restores: true},
		{name: "disable", args: []string{"client", "disable", "hermes", "--json"}, withdraws: true},
		// See the same command of Codex.
		{name: "remove the stored model", args: []string{"model", "remove", "dmxapi/gpt-6.2-sol", "--json"}, where: blockedWithAChoice},
		{name: "sync preview", args: []string{"sync", "--dry-run", "--json"}, preview: true},
		{name: "own setting preview", args: []string{"client", "edit", "hermes", "--via", hermesForwardedTo, "--dry-run", "--json"}, preview: true},
		{name: "end forwarding preview", args: []string{"client", "edit", "hermes", "--direct", "--dry-run", "--json"}, preview: true},
		{name: "select preview", args: []string{"use", "hermes", "ucloud/claude-opus-5-5", "--protocol", "anthropic", "--dry-run", "--json"}, preview: true, stores: "ucloud"},
		{name: "rollback preview", args: []string{"config", "rollback", "--last-change", "--dry-run", "--json"}, preview: true},
		{name: "disable preview", args: []string{"client", "disable", "hermes", "--dry-run", "--json"}, preview: true},
	}
}

// hermesMatrix is Hermes on one file. Every spelling of a pick runs as a
// choice on another protocol and as a pick of the stored model beside an
// earlier record.
func hermesMatrix() selectionMatrix {
	states := map[string]func(t *testing.T) selectionWorld{
		plainState:            hermesPlain,
		"waits for its Token": hermesPendingWorld,
		"chosen of another provider": func(t *testing.T) selectionWorld {
			return hermesPlain(t).choosing(t, "ucloud", "anthropic", "claude-opus-5-5", plainScalars)
		},
		"chosen on another protocol, the record missing": func(t *testing.T) selectionWorld {
			return hermesPlain(t).choosing(t, "dmxapi", "anthropic", "claude-opus-5-5", plainScalars).losingRecord(t, 0)
		},
		// The stored selection forwards, and the person chose a model of
		// another upstream, which the forwarding does not reach.
		"forwards, chosen of another provider": func(t *testing.T) selectionWorld {
			return hermesForwarding(t).choosing(t, "ucloud", "anthropic", "claude-opus-5-5", plainScalars)
		},
		"chosen, a rollback restores a third model":  hermesRestoring("ucloud/claude-opus-5-5", "anthropic", "aigw-ucloud-anthropic/claude-opus-5-5"),
		"chosen, a rollback restores the chosen one": hermesRestoring("dmxapi/claude-opus-5-5", "anthropic", "aigw-dmxapi-anthropic/claude-opus-5-5"),
	}
	for _, spelling := range hermesSpellings() {
		states["chosen on another protocol"+spelling.name] = func(t *testing.T) selectionWorld {
			return hermesPlain(t).choosing(t, "dmxapi", "anthropic", "claude-opus-5-5", spelling)
		}
		states["shows the stored model beside an earlier record"+spelling.name] = func(t *testing.T) selectionWorld {
			return hermesPendingWorld(t).choosing(t, "dmxapi", "responses", "gpt-6.2-sol", spelling)
		}
	}
	return selectionMatrix{client: "hermes", commands: hermesCommands(), states: states}
}
