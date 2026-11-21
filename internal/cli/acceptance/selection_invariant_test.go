package cli_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/domain"
)

// selectionWorld is one client in one state of the matrix: its files, how to
// read the model that a file selects, and whether AIGW's record of a file
// names that model.
type selectionWorld struct {
	app     *cli.App
	output  *bytes.Buffer
	client  string
	account string
	files   []string
	// shows returns the selection that the file at path shows, in the words
	// that storedAs uses, or "" for a missing file.
	shows func(t *testing.T, path string) string
	// follows reports whether AIGW's record of the file names the selection
	// that the file shows.
	follows func(t *testing.T, path string) bool
	// storedAs renders the stored selection as a file would show it.
	storedAs func(runtime domain.Runtime) string
	// restored is the selection that a rollback of the last change restores,
	// in storedAs's words, or "" where the state does not fix it.
	restored string
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
	if runtime, err := cfg.ResolveRuntime(w.client, domain.ModelRef{}); err == nil {
		snap.stored = w.storedAs(runtime)
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

// clientUntouched reports whether the client's binding, files and records are
// as they were.
func (after selectionSnapshot) clientUntouched(before selectionSnapshot) bool {
	return reflect.DeepEqual(after.binding, before.binding) &&
		slices.EqualFunc(after.files, before.files, bytes.Equal) &&
		slices.EqualFunc(after.records, before.records, bytes.Equal)
}

// holds checks the one sentence that every committed command must leave true
// for a client that keeps a record: the record of each of its files names the
// model that the file selects, or the client's selection, files and records
// are exactly as before. A preview and a refused command change nothing at
// all. A choice that a file showed is the stored selection afterwards, unless
// the command selected for the client itself or left the client untouched.
func (w selectionWorld) holds(t *testing.T, before selectionSnapshot, command selectionCommand, err error) {
	t.Helper()
	after := w.snapshot(t)
	if command.preview || err != nil {
		if !bytes.Equal(after.config, before.config) || !after.clientUntouched(before) {
			t.Errorf("%s (err=%v) changed the configuration, a file or a record: stored %q -> %q, files show %q -> %q",
				command.name, err, before.stored, after.stored, before.shows, after.shows)
		}
		return
	}
	if command.withdraws {
		return
	}
	if command.restores && w.restored != "" && after.stored != w.restored {
		t.Errorf("%s succeeded and left %q stored, not the restored %q", command.name, after.stored, w.restored)
	}
	untouched := after.clientUntouched(before)
	for index := range w.files {
		if after.files[index] == nil || untouched {
			continue
		}
		if !after.follows[index] {
			t.Errorf("%s left the record of file %d behind: the file shows %q, stored %q -> %q, and the client was changed",
				command.name, index, after.shows[index], before.stored, after.stored)
		}
		if chose := before.shows[index] != "" && !before.follows[index]; chose && !command.explicit && after.stored != before.shows[index] {
			t.Errorf("%s neither followed nor left the model chosen in file %d: it showed %q, stored %q -> %q",
				command.name, index, before.shows[index], before.stored, after.stored)
		}
	}
}

// selectionMatrix is one client's part of the matrix: the states that its
// files can be in and the commands that run against each.
type selectionMatrix struct {
	client   string
	states   map[string]func(t *testing.T) selectionWorld
	commands []selectionCommand
}

// Every committed command leaves the record of each client file naming the
// model that the file selects, or leaves that client exactly as it was, for
// every state of the files, with and without the provider's Token. The
// reviews of versions one to nine each found one more path on which a record
// stayed behind a file; this test runs the product of states and commands
// instead of the paths that a reviewer happens to pick.
func TestEveryCommandLeavesEachRecordNamingWhatItsFileSelects(t *testing.T) {
	for _, matrix := range []selectionMatrix{codexOneFileMatrix(), codexTwoFilesMatrix(), hermesMatrix()} {
		for _, state := range slices.Sorted(mapKeys(matrix.states)) {
			for _, token := range []bool{true, false} {
				for _, command := range matrix.commands {
					name := fmt.Sprintf("%s/%s/token=%t/%s", matrix.client, state, token, command.name)
					t.Run(strings.ReplaceAll(name, " ", "_"), func(t *testing.T) {
						world := matrix.states[state](t)
						world.setToken(t, token)
						before := world.snapshot(t)
						world.output.Reset()
						err := cli.Execute(world.app, command.args)
						if err != nil {
							t.Logf("refused: %v", err)
						}
						world.holds(t, before, command, err)
						if state == plainState && token && err != nil {
							t.Errorf("%s fails in the plain state, so the matrix proves nothing with it: %v\n%s", command.name, err, world.output)
						}
					})
				}
			}
		}
	}
}

// plainState names the state in which every file shows its recorded model,
// where every command of the matrix must run.
const plainState = "recorded"

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
