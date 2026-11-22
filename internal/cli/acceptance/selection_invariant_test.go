package cli_test

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/domain"
)

// selectionWorld is one client in one state of the matrix: its files, how to
// read the model that a file selects, whether AIGW's record of a file names
// that model, and what the steps that built the state declare about it.
type selectionWorld struct {
	app     *cli.App
	output  *bytes.Buffer
	client  string
	account string
	files   []string
	// shows returns the selection that the file at path shows, in the words
	// that storedAs uses, or "" for a missing file. It reads the file through
	// the parser of the file's own format, whatever the spelling of the pick.
	shows func(t *testing.T, path string) string
	// follows reports whether AIGW's record of the file names the selection
	// that the file shows. Without a record nothing follows.
	follows func(t *testing.T, path string) bool
	// storedAs renders the stored selection as a file would show it.
	storedAs func(runtime domain.Runtime) string
	// reaches names the endpoint that the file at path reaches through AIGW's
	// entry, or "" where the world does not read it. Two providers that serve
	// one model ID are two selections: a choice recorded for another provider
	// than the one the file reaches is not the file's selection.
	reaches func(t *testing.T, path string) string
	// keepsAIGWsPart reports whether a file's bytes still hold what AIGW
	// writes into the client's file, which a withdrawal removes.
	keepsAIGWsPart func(file []byte) bool
	// restored is the selection that a rollback of the last change restores,
	// in storedAs's words, or "" where the state does not fix it.
	restored string
	// restoredBinding is the client's binding in the configuration that the
	// last change replaced, captured as the state is built, so that a rollback
	// is checked by everything that the change touched and not by the model
	// alone.
	restoredBinding *domain.ClientBinding
	// blocked says what in this state prevents AIGW from recording the
	// client's files, or "" when nothing does. The step that builds the
	// obstacle declares it (losingFile, losingRecord, chose), so it is read
	// from how the state is built and never from what the product does.
	blocked string
	// choice is the selection that a file of the state shows in place of the
	// one that its record names, or "" when no file chose.
	choice string
	// chosenAccount is the provider of the chosen selection when it is
	// another provider than account, the stored selection's.
	chosenAccount string
}

// selectionCommand is one command of the matrix. explicit commands decide the
// client's selection themselves; restores is a rollback, which leaves the
// restored selection stored when it succeeds; withdraws removes AIGW's part
// of its files; preview writes nothing.
type selectionCommand struct {
	name      string
	args      []string
	explicit  bool
	restores  bool
	withdraws bool
	preview   bool
	// stores names the provider whose Token an `aigw use` must be able to
	// use, because it selects a model of that provider: without that Token
	// the command is refused where its twin with the Token succeeds.
	stores string
	// leaves returns what a run that succeeded failed to leave in the saved
	// configuration, or "" when the command's own change is there.
	leaves func(after selectionSnapshot) string
	// idle reports that, in the state that before shows, the command asks for
	// nothing that is not stored already. Such a command has nothing to save:
	// it commits nothing, so it must change nothing, like a preview. A command
	// without this declaration that succeeds and changes nothing fails.
	idle func(before selectionSnapshot) bool
	// where limits the command to the states that it accepts; nil runs it in
	// every state. A command that the plain state refuses needs it.
	where func(state selectionWorld) bool
}

// selectionSnapshot is what the invariant compares: the stored configuration,
// the client's binding, and every file of the client with its record.
type selectionSnapshot struct {
	config  []byte
	binding domain.ClientBinding
	stored  string
	files   [][]byte
	records [][]byte
	shows   []string
	follows []bool
	// provider is the provider of the stored selection, whose endpoints an
	// AIGW-written file of the client reaches.
	provider string
	// tree is every path below the world's configuration folder with the hash
	// of its content, so that a command that writes nothing is seen to write
	// nothing anywhere.
	tree map[string]string
}

// selectionRun is one command run in one world: what it returned, and the
// client before and after it.
type selectionRun struct {
	err           error
	before, after selectionSnapshot
	// done says that the run reached its end; a world that could not be
	// built leaves nothing to compare.
	done bool
}

func (w selectionWorld) recordOf(t *testing.T, path string) []byte {
	t.Helper()
	paths := []string{path + adapter.StateSuffix}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		paths = append(paths, resolved+adapter.StateSuffix)
	}
	for _, record := range paths {
		if data, err := os.ReadFile(record); err == nil {
			return data
		}
	}
	return nil
}

func (w selectionWorld) snapshot(t *testing.T) selectionSnapshot {
	t.Helper()
	cfg, err := w.app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	snap := selectionSnapshot{config: readFile(t, w.app.Config.Path()), binding: cfg.Clients[w.client]}
	snap.tree = w.folder(t, filepath.Dir(w.app.Config.Path()))
	if runtime, err := cfg.ResolveRuntime(w.client, domain.ModelRef{}); err == nil {
		snap.stored, snap.provider = w.storedAs(runtime), runtime.ProviderID
	}
	for _, path := range w.files {
		data, err := os.ReadFile(path)
		if err != nil {
			data = nil
		}
		snap.files = append(snap.files, data)
		snap.records = append(snap.records, w.recordOf(t, path))
		shows, follows := "", false
		if data != nil {
			shows, follows = w.shows(t, path), w.follows(t, path)
		}
		snap.shows, snap.follows = append(snap.shows, shows), append(snap.follows, follows)
	}
	return snap
}

// folder names every path below root with the hash of its content, so that a
// command that writes nothing is seen to write nothing anywhere below it. It
// reads no file's content beyond its hash.
func (w selectionWorld) folder(t *testing.T, root string) map[string]string {
	t.Helper()
	paths := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		name, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		paths[name] = fmt.Sprintf("%x", sha256.Sum256(data))
		return nil
	})
	if err != nil {
		t.Fatalf("read the world's tree: %v", err)
	}
	return paths
}

// clientUntouched reports whether the client's binding, files and records are
// as they were.
func (after selectionSnapshot) clientUntouched(before selectionSnapshot) bool {
	return reflect.DeepEqual(after.binding, before.binding) &&
		slices.EqualFunc(after.files, before.files, bytes.Equal) &&
		slices.EqualFunc(after.records, before.records, bytes.Equal)
}

// selectionMatrix is one client's part of the matrix: the states that its
// files can be in and the commands that run against each.
type selectionMatrix struct {
	client   string
	states   map[string]func(t *testing.T) selectionWorld
	commands []selectionCommand
}

// selectionCell is one state and one command of a matrix.
type selectionCell struct {
	build   func(t *testing.T) selectionWorld
	command selectionCommand
	plain   bool
}

// Every committed command leaves the record of each client file naming the
// model that the file selects, for every state of the files and every
// spelling of a pick, with and without each Token that the state depends on;
// only where the state itself prevents the record may it leave the client
// exactly as it was. The reviews of versions one to ten each found one more
// path on which a record stayed behind a file; these tests run the product of
// states and commands instead of the paths that a reviewer happens to pick.
// There is one test for each client's matrix, so that each runs alone.
func TestEveryCommandLeavesEachRecordNamingWhatItsFileSelectsCodexOneFile(t *testing.T) {
	codexOneFileMatrix().run(t)
}

// The matrix of Codex on two files.
func TestEveryCommandLeavesEachRecordNamingWhatItsFileSelectsCodexTwoFiles(t *testing.T) {
	codexTwoFilesMatrix().run(t)
}

// The matrix of Hermes.
func TestEveryCommandLeavesEachRecordNamingWhatItsFileSelectsHermes(t *testing.T) {
	hermesMatrix().run(t)
}

// run runs every command of the matrix in every state that the command
// accepts: once with every Token, and once without each Token that the state
// depends on, each time in a world of its own. A run without a Token is
// compared with its twin with every Token (twinned).
func (matrix selectionMatrix) run(t *testing.T) {
	t.Logf("the matrix of %s: %d states, %d commands", matrix.client, len(matrix.states), len(matrix.commands))
	for _, state := range slices.Sorted(mapKeys(matrix.states)) {
		t.Run(state, func(t *testing.T) {
			// The state is built once more to read what its steps declare.
			declared := matrix.states[state](t)
			t.Logf("declared: blocked=%q, choice=%q, the choice's other provider=%q, a rollback restores %q",
				declared.blocked, declared.choice, declared.chosenAccount, declared.restored)
			for _, command := range matrix.commands {
				if command.where != nil && !command.where(declared) {
					continue
				}
				cell := selectionCell{build: matrix.states[state], command: command, plain: state == plainState}
				t.Run(command.name, func(t *testing.T) {
					var with selectionRun
					t.Run("token=true", func(t *testing.T) { with = cell.run(t, "") })
					for _, absent := range declared.absentTokens() {
						t.Run(declared.tokenCase(absent), func(t *testing.T) {
							if without := cell.run(t, absent); with.done {
								without.twinned(t, with, command, absent)
							}
						})
					}
				})
			}
		})
	}
}

// run executes the cell's command in a fresh world, without the Token of the
// provider absent or with every Token when absent is empty, and checks the
// sentence.
func (cell selectionCell) run(t *testing.T, absent string) selectionRun {
	t.Helper()
	world := cell.build(t)
	world.withoutToken(t, absent)
	run := selectionRun{before: world.snapshot(t)}
	world.output.Reset()
	run.err = cli.Execute(world.app, cell.command.args)
	if run.err != nil {
		t.Logf("refused: %v", run.err)
	}
	run.after = world.holds(t, run.before, cell.command, run.err)
	if cell.plain && absent == "" && run.err != nil {
		t.Errorf("%s fails in the plain state, so the matrix proves nothing with it: %v", cell.command.name, run.err)
	}
	if t.Failed() {
		said := world.output.String()
		if len(said) > 1500 {
			said = said[:1500] + "\n...[cut]"
		}
		t.Logf("%s said:\n%s", cell.command.name, said)
	}
	run.done = true
	return run
}

// plainState names the state in which every file shows its recorded model,
// where every command of the matrix that runs in every state must succeed.
const plainState = "recorded"

// absentTokens names the providers whose Token a run of the state goes
// without, one run each: the stored selection's provider, and the provider of
// the chosen model where that is another.
func (w selectionWorld) absentTokens() []string {
	if w.chosenAccount == "" {
		return []string{w.account}
	}
	return []string{w.account, w.chosenAccount}
}

// tokenCase names the run without the Token of absent.
func (w selectionWorld) tokenCase(absent string) string {
	if absent == w.account {
		return "token=false"
	}
	return "token=false for the chosen provider"
}

// withoutToken stores the Token of the stored selection's provider, which a
// state may have removed to build itself, and removes the Token of absent.
func (w selectionWorld) withoutToken(t *testing.T, absent string) {
	t.Helper()
	w.setToken(t, true)
	if absent == "" {
		return
	}
	if err := w.app.Secrets.Delete(absent); err != nil {
		t.Fatal(err)
	}
}

func (w selectionWorld) setToken(t *testing.T, present bool) {
	t.Helper()
	var err error
	if present {
		err = w.app.Secrets.Set(w.account, "fixture-token")
	} else {
		err = w.app.Secrets.Delete(w.account)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func mapKeys[V any](values map[string]V) func(yield func(string) bool) {
	return func(yield func(string) bool) {
		for key := range values {
			if !yield(key) {
				return
			}
		}
	}
}
