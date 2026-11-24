//go:build selection_matrix

package cli_test

// This file holds the exhaustive tests of the client's own choice: every command in
// every state of the files, with and without each Token. They build only under the
// selection_matrix tag and run in the quality graph's own gate of that name, without
// the race detector and coverage: under the race detector they take about six times
// as long and ran past the coverage run's package limit (full checks of 2026-11-22
// and 2026-11-23). The ordinary tests of the same rules stay in the untagged files.
// Every test here begins with TestMatrix, the gate's whole selection, so that it
// runs none of the untagged tests, which go-behavior runs.

import (
	"slices"
	"strings"
	"testing"
)

// Every committed command leaves the record of each client file naming the
// model that the file selects, for every state of the files and every
// spelling of a pick, with and without each Token that the state depends on;
// only where the state itself prevents the record may it leave the client
// exactly as it was. The reviews of versions one to ten each found one more
// path on which a record stayed behind a file; these tests run the product of
// states and commands instead of the paths that a reviewer happens to pick.
// There is one test for each client's matrix, so that each runs alone.
func TestMatrixEveryCommandLeavesEachRecordNamingWhatItsFileSelectsCodexOneFile(t *testing.T) {
	t.Parallel()
	codexOneFileMatrix().run(t)
}

// The matrix of Codex on two files.
func TestMatrixEveryCommandLeavesEachRecordNamingWhatItsFileSelectsCodexTwoFiles(t *testing.T) {
	t.Parallel()
	codexTwoFilesMatrix().run(t)
}

// The matrix of Hermes.
func TestMatrixEveryCommandLeavesEachRecordNamingWhatItsFileSelectsHermes(t *testing.T) {
	t.Parallel()
	hermesMatrix().run(t)
}

// Every command of zz11CodexCommands in every state of Codex on one file,
// with the Token and without it: the rule, judged by zz11Judge.
func TestMatrixCommandsOutsideTheMatrixCodexOneFile(t *testing.T) {
	t.Parallel()
	zz11Sweep(t, zz11CodexStates(1), zz11CodexCommands())
}

// The same for Codex on two files, in four parts, so that each run stays
// inside the time that a run may take.
func TestMatrixCommandsOutsideTheMatrixCodexTwoFilesFirst(t *testing.T) {
	t.Parallel()
	zz11Sweep(t, zz11Part(zz11CodexStates(2), 4, 0), zz11CodexCommands())
}

func TestMatrixCommandsOutsideTheMatrixCodexTwoFilesSecond(t *testing.T) {
	t.Parallel()
	zz11Sweep(t, zz11Part(zz11CodexStates(2), 4, 1), zz11CodexCommands())
}

func TestMatrixCommandsOutsideTheMatrixCodexTwoFilesThird(t *testing.T) {
	t.Parallel()
	zz11Sweep(t, zz11Part(zz11CodexStates(2), 4, 2), zz11CodexCommands())
}

func TestMatrixCommandsOutsideTheMatrixCodexTwoFilesFourth(t *testing.T) {
	t.Parallel()
	zz11Sweep(t, zz11Part(zz11CodexStates(2), 4, 3), zz11CodexCommands())
}

// Every command of zz11HermesSweepCommands in every state of Hermes, with
// each Token and without it, in three parts.
func TestMatrixCommandsOutsideTheMatrixHermesFirst(t *testing.T) {
	t.Parallel()
	zz11Sweep(t, zz11Part(zz11HermesStates(), 3, 0), zz11HermesSweepCommands())
}

func TestMatrixCommandsOutsideTheMatrixHermesSecond(t *testing.T) {
	t.Parallel()
	zz11Sweep(t, zz11Part(zz11HermesStates(), 3, 1), zz11HermesSweepCommands())
}

func TestMatrixCommandsOutsideTheMatrixHermesThird(t *testing.T) {
	t.Parallel()
	zz11Sweep(t, zz11Part(zz11HermesStates(), 3, 2), zz11HermesSweepCommands())
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
func TestMatrixCommandsInFurtherStatesCodex(t *testing.T) {
	t.Parallel()
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
func TestMatrixCommandsInFurtherStatesHermes(t *testing.T) {
	t.Parallel()
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

// A pick of another model in Hermes's file, written in each further way that
// YAML allows, with the Token and without it.
//
// Before: F = R = aigw-dmxapi-responses/gpt-6.2-sol, S = dmxapi/gpt-6.2-sol on
// the Responses protocol. The person selects claude-opus-5-5 of the entry
// aigw-dmxapi-anthropic in the spelling: F is that by the YAML parser, R and
// S stay.
//
// Then a command that does not select for Hermes runs. The rule is the one of
// the Codex probe above. Because the record of Hermes holds a digest and no
// model, "the record names what the file selects" is then tried by what
// follows from it: the person selects gpt-6.2-sol again, in the same spelling,
// the Token is there, and `aigw sync` must leave that last pick stored and in
// the file, or refuse and change nothing.
func TestMatrixAHermesPickInFurtherSpellingsIsRecordedOrRefused(t *testing.T) {
	t.Parallel()
	const earlier, chosen = "aigw-dmxapi-responses/gpt-6.2-sol", "aigw-dmxapi-anthropic/claude-opus-5-5"
	for _, pick := range zz11HermesPicks() {
		for _, crlf := range []bool{false, true} {
			name := pick.name
			if crlf {
				name += ", the whole file with carriage returns"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				zz11HermesSpellingProbe(t, pick, chosen, earlier, crlf)
			})
		}
	}
}
