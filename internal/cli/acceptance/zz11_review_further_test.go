package cli_test

import (
	"slices"
	"strings"
	"testing"
)

// This file holds the eleventh review's runs of the matrix's own commands and
// checker in states of Codex and Hermes that the matrix does not build.

// zz11After runs a last change in a Codex world, after which a rollback
// restores gpt-sol with what the change replaced, and then picks model in the
// first file.
func zz11After(count int, model string, change ...string) func(t *testing.T) selectionWorld {
	return func(t *testing.T) selectionWorld {
		world := codexWorld(t, count)
		world.run(t, change...)
		world.restored = "gpt-sol"
		return world.picking(t, 0, basicString, model)
	}
}

// The matrix's own commands, judged by the matrix's own checker with its
// comparison across the Token, in states of Codex that the matrix does not
// build.
//
//   - Three different models: S = gateway/gpt-sol waits, R = gpt-terra, and
//     the person picks gpt-luna (F), in one file and in one of two.
//   - Both files pick the stored model beside earlier records, and one picks
//     the stored model while the other picks a third.
//   - A last change that is not about the selection and still changes what
//     Codex's file holds, so that a rollback restores another file without
//     another binding: the provider's endpoint, the chosen model's label, the
//     stored model's label, a retry budget.
//   - A pick of the model that the last change added, which a rollback takes
//     out of the configuration again.
//   - Two files of which one lost its model line and the other chose.
func TestZZ11TheMatrixsCommandsInFurtherStatesCodex(t *testing.T) {
	states := map[string]func(t *testing.T) selectionWorld{
		plainState: func(t *testing.T) selectionWorld { return codexWorld(t, 1) },
		"a third model picked while the stored selection waits": func(t *testing.T) selectionWorld {
			return codexPending(t, 1).picking(t, 0, basicString, "gpt-luna")
		},
		"two files, a third model picked in one while the stored selection waits": func(t *testing.T) selectionWorld {
			return codexPending(t, 2).picking(t, 0, basicString, "gpt-luna")
		},
		"two files, both show the stored model beside earlier records": func(t *testing.T) selectionWorld {
			return codexPending(t, 2).picking(t, 0, basicString, "gpt-sol").picking(t, 1, literalString, "gpt-sol")
		},
		"two files, one shows the stored model and the other a third": func(t *testing.T) selectionWorld {
			return codexPending(t, 2).picking(t, 0, basicString, "gpt-sol").picking(t, 1, basicString, "gpt-luna")
		},
		"chosen, a rollback restores the provider's endpoint":            zz11After(1, "gpt-terra", "provider", "edit", "gateway", "--endpoint", "responses=https://gateway-two.test/v1"),
		"two files, chosen, a rollback restores the provider's endpoint": zz11After(2, "gpt-terra", "provider", "edit", "gateway", "--endpoint", "responses=https://gateway-two.test/v1"),
		"chosen, a rollback restores the chosen model's label":           zz11After(1, "gpt-terra", "model", "edit", "gateway/gpt-terra", "--label", "Terra Two"),
		"chosen, a rollback restores the stored model's label":           zz11After(1, "gpt-terra", "model", "edit", "gateway/gpt-sol", "--label", "Sol Two"),
		"chosen, a rollback restores no retry budget":                    zz11After(1, "gpt-terra", "client", "edit", "codex", "--request-retries", "4"),
		"chosen the model that the last change added":                    zz11After(1, "gpt-nova", "model", "add", "gateway/gpt-nova", "--unlisted"),
		"two files, chosen the model that the last change added":         zz11After(2, "gpt-nova", "model", "add", "gateway/gpt-nova", "--unlisted"),
		// AIGW reads a file without a root model as one that shows the recorded
		// model, so the other file's choice is followed in every command's
		// configuration, while no plan admits the file. The state is declared
		// blocked by hand: an outside edit prevents the record.
		"two files, one without its model line, the other chose": func(t *testing.T) selectionWorld {
			world := codexWorld(t, 2)
			lines, at := recCodexModelLine(t, world.files[0])
			writeFile(t, world.files[0], []byte(strings.Join(append(slices.Clone(lines[:at]), lines[at+1:]...), "\n")), 0o600)
			world = world.picking(t, 1, basicString, "gpt-terra")
			world.blocked = "a file of the client lost its model line"
			return world
		},
	}
	selectionMatrix{client: "codex, further states", commands: codexCommands(), states: states}.run(t)
}

// zz11HermesAfter runs a last change in the plain Hermes world, after which a
// rollback restores the stored selection with what the change replaced, and
// then picks model of account on protocol.
func zz11HermesAfter(account, protocol, model string, change ...string) func(t *testing.T) selectionWorld {
	return func(t *testing.T) selectionWorld {
		world := hermesPlain(t)
		world.run(t, change...)
		return world.choosing(t, account, protocol, model, plainScalars)
	}
}

// The same for Hermes.
//
//   - Three different selections: the stored one waits, the record names the
//     one that AIGW wrote before, and the person picks a third.
//   - A pick of another model of the same entry, with and without a stored
//     forwarding, which such a pick does not end.
//   - A stored forwarding and a pick of the same provider on another protocol.
//   - A last change that changes what Hermes's file holds without another
//     binding: the chosen entry's endpoint.
//   - A pick of the model that the last change added.
func TestZZ11TheMatrixsCommandsInFurtherStatesHermes(t *testing.T) {
	states := map[string]func(t *testing.T) selectionWorld{
		plainState: hermesPlain,
		"a third model picked while the stored selection waits": func(t *testing.T) selectionWorld {
			return hermesPendingWorld(t).choosing(t, "dmxapi", "responses", "gpt-6.1-terra", plainScalars)
		},
		"chosen of the same entry": func(t *testing.T) selectionWorld {
			return hermesPlain(t).choosing(t, "dmxapi", "responses", "gpt-6.1-terra", plainScalars)
		},
		"forwards, chosen on another protocol": func(t *testing.T) selectionWorld {
			return hermesForwarding(t).choosing(t, "dmxapi", "anthropic", "claude-opus-5-5", plainScalars)
		},
		"forwards, chosen of the same entry": func(t *testing.T) selectionWorld {
			return hermesForwarding(t).choosing(t, "dmxapi", "responses", "gpt-6.1-terra", plainScalars)
		},
		"chosen, a rollback restores the chosen entry's endpoint": zz11HermesAfter("dmxapi", "anthropic", "claude-opus-5-5",
			"provider", "edit", "dmxapi", "--endpoint", "anthropic=https://dmxapi-two.test"),
		"chosen the model that the last change added": zz11HermesAfter("dmxapi", "responses", "gpt-nova",
			"model", "add", "dmxapi/gpt-nova", "--unlisted", "--protocol", "responses"),
	}
	selectionMatrix{client: "hermes, further states", commands: hermesCommands(), states: states}.run(t)
}
