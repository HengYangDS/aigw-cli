package cli_test

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
)

// This file holds the eleventh review's probes of commands that the matrix
// does not run, in every state that the matrix builds and in a few more. Each
// run is judged by the rule alone, with a stricter eye than the matrix's
// checker: a refusal and a preview may change no path below the world's root,
// and a command that does not select for the client may never change which
// model a file selects, except to bring a file that showed its recorded model
// to the model that another file chose or to the stored selection.

// zz11Command is one command of the sweep. args builds its arguments in the
// world, so that a command can name what the world holds; stdin is what it
// reads from standard input.
type zz11Command struct {
	name  string
	args  func(t *testing.T, w selectionWorld) []string
	stdin string
	// idle says that the command asks for what the world stores already, in
	// every state: it may leave a record behind a file, and then it must have
	// changed no path at all.
	idle bool
	// preview says that the command writes nothing.
	preview bool
	// tokenRefuses says that the command is known to be refused without the
	// Token where it succeeds with it, which a probe of its own reports
	// (TestZZ11TheSameFilesAreRefusedWithoutTheToken); the sweep then compares
	// nothing more of that pair.
	tokenRefuses bool
}

// zz11Args returns a command whose arguments do not depend on the world.
func zz11Args(args ...string) func(*testing.T, selectionWorld) []string {
	return func(*testing.T, selectionWorld) []string { return args }
}

// zz11Run runs the command in the world.
func zz11Run(t *testing.T, w selectionWorld, command zz11Command) ranIn {
	t.Helper()
	if command.stdin != "" {
		w.app.In = strings.NewReader(command.stdin)
	}
	return runIn(t, w, command.args(t, w)...)
}

// zz11Judged is one judged run.
type zz11Judged struct {
	class  string
	stored string
	behind []bool
	err    error
}

// zz11Judge judges a run of a command that neither selects for the client nor
// withdraws it, in a world whose declarations say whether something prevents
// the record.
//
// The classes: "refused"; "recorded" (every record names what its file
// selects, and a model that a file chose is stored); "left" (the client's
// binding, files and records are as before, and a record is behind its file);
// "broken" (anything else).
func zz11Judge(t *testing.T, w selectionWorld, command zz11Command, token string, run ranIn) zz11Judged {
	t.Helper()
	judged := zz11Judged{stored: run.after.stored, err: run.err}
	fail := func(format string, args ...any) {
		t.Helper()
		t.Errorf("%s, %s: %s\n  before: %s\n  after:  %s\n  err: %v\n  changed: %v", command.name, token, fmt.Sprintf(format, args...),
			shownLine(run.before), shownLine(run.after), run.err, run.changed)
	}
	if run.err != nil || command.preview {
		judged.class = "refused"
		if command.preview && run.err == nil {
			judged.class = "previewed"
		}
		if len(run.changed) != 0 {
			fail("a refusal or a preview changed paths")
		}
		return judged
	}
	choice := zz11Choice(run.before)
	behindFlags, behind := zz11Behind(run, choice, fail)
	judged.behind = behindFlags
	zz11Class(w, command, run, choice, behind, &judged, fail)
	return judged
}

// zz11Choice names the model that a file of the before state chose, the last
// one in order, or "" when no file chose.
func zz11Choice(before selectionSnapshot) string {
	choice := ""
	for index := range before.shows {
		if before.shows[index] != "" && !before.follows[index] {
			choice = before.shows[index]
		}
	}
	return choice
}

// zz11Behind reports, for each file of after, whether its record is behind it,
// and whether any record is; and it faults a file whose selection moved
// although the command does not select for the client.
func zz11Behind(run ranIn, choice string, fail func(string, ...any)) ([]bool, bool) {
	behind := false
	for index := range run.before.shows {
		if run.after.files[index] == nil {
			continue
		}
		trailing := !run.after.follows[index]
		behind = behind || trailing
		moved := run.after.shows[index] != run.before.shows[index]
		if moved && (!run.before.follows[index] || (run.after.shows[index] != choice && run.after.shows[index] != run.before.stored)) {
			fail("file %d selected %q and now selects %q, and the command does not select for the client", index, run.before.shows[index], run.after.shows[index])
		}
	}
	flags := make([]bool, 0, len(run.before.shows))
	for index := range run.before.shows {
		flags = append(flags, run.after.files[index] != nil && !run.after.follows[index])
	}
	return flags, behind
}

// zz11Class names what the run did and faults it when nothing in the state
// allows it.
func zz11Class(w selectionWorld, command zz11Command, run ranIn, choice string, behind bool, judged *zz11Judged, fail func(string, ...any)) {
	untouched := run.after.clientUntouched(run.before)
	switch {
	case !behind && choice != "" && run.after.stored != choice:
		judged.class = "broken"
		fail("a file chose %q and %q is stored", choice, run.after.stored)
	case !behind && choice == "" && run.after.stored != run.before.stored:
		judged.class = "broken"
		fail("no file chose, and the stored selection moved from %q to %q", run.before.stored, run.after.stored)
	case !behind:
		judged.class = "recorded"
	case untouched:
		judged.class = "left"
		switch {
		case command.idle:
			if len(run.changed) != 0 {
				fail("the command asks for what is stored already and changed paths")
			}
		case w.blocked == "":
			fail("the command succeeded and left a record behind its file, and nothing in the state prevents the record")
		}
	default:
		judged.class = "broken"
		fail("the command changed the client and left a record behind its file")
	}
}

// zz11Sweep runs every command in every state: once with every Token, and
// once without each Token that the state depends on, and compares the twins.
// Each run builds its world in a subtest of its own, so that the tree that it
// compares holds that world's files alone; each state runs in parallel with
// the others.
func zz11Sweep(t *testing.T, states map[string]func(t *testing.T) selectionWorld, commands []zz11Command) {
	t.Parallel()
	for _, state := range slices.Sorted(mapKeys(states)) {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			declared := states[state](t)
			for _, command := range commands {
				t.Run(command.name, func(t *testing.T) {
					run := func(absent string) zz11Judged {
						token := "with every Token"
						if absent != "" {
							token = "without the Token of " + absent
						}
						var judged zz11Judged
						t.Run(token, func(t *testing.T) {
							world := states[state](t)
							world.withoutToken(t, absent)
							judged = zz11Judge(t, world, command, token, zz11Run(t, world, command))
							t.Logf("%s", judged.class)
						})
						return judged
					}
					with := run("")
					for _, absent := range declared.absentTokens() {
						without := run(absent)
						// A run whose world could not be built was judged by
						// nobody and has failed on its own.
						if with.class == "" || without.class == "" {
							continue
						}
						if command.tokenRefuses && with.err == nil && without.err != nil {
							continue
						}
						if with.class != without.class || with.stored != without.stored || !slices.Equal(with.behind, without.behind) {
							t.Errorf("the Token of %s decides the outcome: with it %s, %q stored, behind %v (err=%v); without it %s, %q stored, behind %v (err=%v)",
								absent, with.class, with.stored, with.behind, with.err, without.class, without.stored, without.behind, without.err)
						}
					}
				})
			}
		})
	}
}

// zz11SameLabel edits a provider's label to the label that it has.
func zz11SameLabel(provider string) func(*testing.T, selectionWorld) []string {
	return func(t *testing.T, w selectionWorld) []string {
		t.Helper()
		cfg, err := w.app.Config.Load()
		if err != nil {
			t.Fatal(err)
		}
		return []string{"provider", "edit", provider, "--label", cfg.Providers[provider].Label, "--json"}
	}
}

// zz11SameTargets edits a client's files to the files that it has.
func zz11SameTargets(t *testing.T, w selectionWorld) []string {
	t.Helper()
	args := []string{"client", "edit", w.client}
	for _, target := range selection(t, w.app, w.client).Targets {
		args = append(args, "--target", target)
	}
	return append(args, "--json")
}

// zz11EnableAgain enables the world's client, which is enabled, with the
// program and the files that it has.
func zz11EnableAgain(more ...string) func(*testing.T, selectionWorld) []string {
	return func(t *testing.T, w selectionWorld) []string {
		t.Helper()
		binding := selection(t, w.app, w.client)
		args := []string{"client", "enable", w.client, "--executable", binding.Executable}
		for _, target := range binding.Targets {
			args = append(args, "--target", target)
		}
		return append(append(args, more...), "--json")
	}
}

// zz11CodexCommands are the commands that the Codex matrices do not run. None
// selects a model for Codex or withdraws it.
func zz11CodexCommands() []zz11Command {
	return []zz11Command{
		{name: "another provider's label, as it is", args: zz11SameLabel("spare"), idle: true},
		{name: "the selection's provider's label, as it is", args: zz11SameLabel("gateway"), idle: true},
		{name: "the selection's provider's label", args: zz11Args("provider", "edit", "gateway", "--label", "Renamed", "--json")},
		{name: "another provider's endpoint", args: zz11Args("provider", "edit", "spare", "--endpoint", "responses=https://spare-two.test/v1", "--json")},
		{name: "the selection's provider's endpoint", args: zz11Args("provider", "edit", "gateway", "--endpoint", "responses=https://gateway-two.test/v1", "--json")},
		{name: "remove another provider", args: zz11Args("provider", "remove", "spare", "--json")},
		{name: "remove another provider's model", args: zz11Args("model", "remove", "spare/gpt-spare", "--json")},
		{name: "remove gpt-luna", args: zz11Args("model", "remove", "gateway/gpt-luna", "--json")},
		{name: "remove gpt-terra", args: zz11Args("model", "remove", "gateway/gpt-terra", "--json")},
		{name: "remove gpt-sol", args: zz11Args("model", "remove", "gateway/gpt-sol", "--json")},
		{name: "relabel gpt-terra", args: zz11Args("model", "edit", "gateway/gpt-terra", "--label", "Terra Two", "--json")},
		{name: "relabel gpt-sol", args: zz11Args("model", "edit", "gateway/gpt-sol", "--label", "Sol Two", "--json")},
		{name: "relabel gpt-luna", args: zz11Args("model", "edit", "gateway/gpt-luna", "--label", "Luna Two", "--json")},
		{name: "add a model", args: zz11Args("model", "add", "gateway/gpt-nova", "--unlisted", "--json")},
		{name: "add a provider", args: zz11Args("provider", "add", "extra", "--endpoint", "responses=https://extra.test/v1", "--token-stdin", "--json"), stdin: "extra-token\n"},
		// A rotation stores a Token and saves no configuration.
		{name: "rotate another provider's Token", args: zz11Args("provider", "rotate", "spare", "--token-stdin", "--json"), stdin: "spare-token-two\n", idle: true},
		{name: "native retries, as they are", args: zz11Args("client", "edit", "codex", "--native-retries", "--json"), idle: true},
		{name: "stream retries", args: zz11Args("client", "edit", "codex", "--stream-retries", "3", "--json")},
		{name: "the same files", args: zz11SameTargets, idle: true, tokenRefuses: true},
		{name: "enable again", args: zz11EnableAgain()},
		{name: "another provider's endpoint, preview", args: zz11Args("provider", "edit", "spare", "--endpoint", "responses=https://spare-two.test/v1", "--dry-run", "--json"), preview: true},
		{name: "remove gpt-sol, preview", args: zz11Args("model", "remove", "gateway/gpt-sol", "--dry-run", "--json"), preview: true},
		{name: "relabel gpt-terra, preview", args: zz11Args("model", "edit", "gateway/gpt-terra", "--label", "Terra Two", "--dry-run", "--json"), preview: true},
		{name: "enable again, preview", args: zz11EnableAgain("--dry-run"), preview: true},
	}
}

// zz11CodexStates are the states of a Codex matrix and the states that the
// matrix does not build: three different models, and every file with a pick
// of the stored model beside an earlier record.
func zz11CodexStates(count int) map[string]func(t *testing.T) selectionWorld {
	matrix := codexOneFileMatrix()
	if count == 2 {
		matrix = codexTwoFilesMatrix()
	}
	states := maps.Clone(matrix.states)
	states["zz11 a third model picked while the stored selection waits"] = func(t *testing.T) selectionWorld {
		return codexPending(t, count).picking(t, 0, basicString, "gpt-luna")
	}
	if count == 2 {
		states["zz11 both show the stored model beside earlier records"] = func(t *testing.T) selectionWorld {
			return codexPending(t, 2).picking(t, 0, basicString, "gpt-sol").picking(t, 1, literalString, "gpt-sol")
		}
		states["zz11 one shows the stored model, the other a third, while the stored selection waits"] = func(t *testing.T) selectionWorld {
			return codexPending(t, 2).picking(t, 0, basicString, "gpt-sol").picking(t, 1, basicString, "gpt-luna")
		}
	}
	return states
}

// Every command of zz11CodexCommands in every state of Codex on one file,
// with the Token and without it: the rule, judged by zz11Judge.
func TestZZ11CommandsOutsideTheMatrixCodexOneFile(t *testing.T) {
	zz11Sweep(t, zz11CodexStates(1), zz11CodexCommands())
}

// The same for Codex on two files, in four parts, so that each run stays
// inside the time that a run may take.
func TestZZ11CommandsOutsideTheMatrixCodexTwoFilesFirst(t *testing.T) {
	zz11Sweep(t, zz11Part(zz11CodexStates(2), 4, 0), zz11CodexCommands())
}

func TestZZ11CommandsOutsideTheMatrixCodexTwoFilesSecond(t *testing.T) {
	zz11Sweep(t, zz11Part(zz11CodexStates(2), 4, 1), zz11CodexCommands())
}

func TestZZ11CommandsOutsideTheMatrixCodexTwoFilesThird(t *testing.T) {
	zz11Sweep(t, zz11Part(zz11CodexStates(2), 4, 2), zz11CodexCommands())
}

func TestZZ11CommandsOutsideTheMatrixCodexTwoFilesFourth(t *testing.T) {
	zz11Sweep(t, zz11Part(zz11CodexStates(2), 4, 3), zz11CodexCommands())
}

// zz11HermesSweepCommands are the commands that the Hermes matrix does not
// run. None selects a model for Hermes or withdraws it.
func zz11HermesSweepCommands() []zz11Command {
	return []zz11Command{
		{name: "another provider's label, as it is", args: zz11SameLabel("ucloud"), idle: true},
		{name: "the selection's provider's label, as it is", args: zz11SameLabel("dmxapi"), idle: true},
		{name: "the selection's provider's label", args: zz11Args("provider", "edit", "dmxapi", "--label", "Renamed", "--json")},
		{name: "another provider's endpoint", args: zz11Args("provider", "edit", "ucloud", "--endpoint", "responses=https://ucloud-two.test/v1", "--json")},
		{name: "the selection's provider's endpoint", args: zz11Args("provider", "edit", "dmxapi", "--endpoint", "responses=https://dmxapi-two.test/v1", "--json")},
		{name: "remove another provider", args: zz11Args("provider", "remove", "ucloud", "--json")},
		{name: "remove an unused model", args: zz11Args("model", "remove", "dmxapi/gpt-6-luna", "--json")},
		{name: "remove the model that the probes choose", args: zz11Args("model", "remove", "dmxapi/claude-opus-5-5", "--json")},
		{name: "remove the stored model", args: zz11Args("model", "remove", "dmxapi/gpt-6.2-sol", "--json")},
		{name: "relabel the model that the probes choose", args: zz11Args("model", "edit", "dmxapi/claude-opus-5-5", "--label", "Opus Two", "--json")},
		{name: "add a model", args: zz11Args("model", "add", "dmxapi/gpt-nova", "--unlisted", "--protocol", "responses", "--json")},
		{name: "rotate another provider's Token", args: zz11Args("provider", "rotate", "ucloud", "--token-stdin", "--json"), stdin: "ucloud-token-two\n", idle: true},
		{name: "remove the model of the same entry", args: zz11Args("model", "remove", "dmxapi/gpt-6.1-terra", "--json")},
		{name: "the same files", args: zz11SameTargets, idle: true, tokenRefuses: true},
		{name: "enable again", args: zz11EnableAgain()},
		{name: "remove the stored model, preview", args: zz11Args("model", "remove", "dmxapi/gpt-6.2-sol", "--dry-run", "--json"), preview: true},
		{name: "the selection's provider's endpoint, preview", args: zz11Args("provider", "edit", "dmxapi", "--endpoint", "responses=https://dmxapi-two.test/v1", "--dry-run", "--json"), preview: true},
	}
}

// zz11HermesStates are the states of the Hermes matrix and two more: a third
// model picked while the stored selection waits, and a pick of another model
// of the same entry.
func zz11HermesStates() map[string]func(t *testing.T) selectionWorld {
	states := maps.Clone(hermesMatrix().states)
	states["zz11 a third model picked while the stored selection waits"] = func(t *testing.T) selectionWorld {
		return hermesPendingWorld(t).choosing(t, "dmxapi", "responses", "gpt-6.1-terra", plainScalars)
	}
	states["zz11 chosen of the same entry"] = func(t *testing.T) selectionWorld {
		return hermesPlain(t).choosing(t, "dmxapi", "responses", "gpt-6.1-terra", plainScalars)
	}
	return states
}

// Every command of zz11HermesSweepCommands in every state of Hermes, with
// each Token and without it, in three parts.
func TestZZ11CommandsOutsideTheMatrixHermesFirst(t *testing.T) {
	zz11Sweep(t, zz11Part(zz11HermesStates(), 3, 0), zz11HermesSweepCommands())
}

func TestZZ11CommandsOutsideTheMatrixHermesSecond(t *testing.T) {
	zz11Sweep(t, zz11Part(zz11HermesStates(), 3, 1), zz11HermesSweepCommands())
}

func TestZZ11CommandsOutsideTheMatrixHermesThird(t *testing.T) {
	zz11Sweep(t, zz11Part(zz11HermesStates(), 3, 2), zz11HermesSweepCommands())
}

// zz11Part returns the states whose place in the sorted names leaves part
// when it is divided by parts.
func zz11Part(states map[string]func(t *testing.T) selectionWorld, parts, part int) map[string]func(t *testing.T) selectionWorld {
	chosen := map[string]func(t *testing.T) selectionWorld{}
	for index, name := range slices.Sorted(mapKeys(states)) {
		if index%parts == part {
			chosen[name] = states[name]
		}
	}
	return chosen
}

// The commands that the sweep runs must do something in the plain state, or
// their columns prove nothing: each succeeds there, and each that is not
// declared idle or a preview changes a path.
func TestZZ11TheSweepsCommandsRunInThePlainState(t *testing.T) {
	for client, sweep := range map[string]struct {
		build    func(t *testing.T) selectionWorld
		commands []zz11Command
	}{
		"codex":  {func(t *testing.T) selectionWorld { return codexWorld(t, 1) }, zz11CodexCommands()},
		"hermes": {hermesPlain, zz11HermesSweepCommands()},
	} {
		for _, command := range sweep.commands {
			world := sweep.build(t)
			run := zz11Run(t, world, command)
			t.Logf("%s, %s: err=%v, changed %d paths", client, command.name, run.err, len(run.changed))
		}
	}
}
