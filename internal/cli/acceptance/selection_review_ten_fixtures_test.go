package cli_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/cli"

	"go.yaml.in/yaml/v4"
)

// This file holds the helpers of the regression tests for the tenth review's
// findings (selection_review_ten_test.go and
// selection_review_ten_forwarding_test.go). Most come from the reviewer's own
// probes: a command run that also sees every file below the world's root, so
// that "changes no file" means no file at all, and a pick in Hermes that is
// possible only where Hermes's file lists the entry.

// worldRoot is the directory that holds every temporary directory of the test
// that built the world, so that a tree below it holds every file that a
// command could write: the configuration, its backups, the client's files,
// AIGW's records, catalogs and credential readers.
func worldRoot(t *testing.T, w selectionWorld) string {
	t.Helper()
	directory := filepath.Dir(w.app.Config.Path())
	if !regexp.MustCompile(`^\d{3}$`).MatchString(filepath.Base(directory)) {
		t.Fatalf("the configuration is not in a numbered temporary directory: %s", directory)
	}
	return filepath.Dir(directory)
}

// worldTree hashes every regular file and names every link and directory
// below the world's root.
func worldTree(t *testing.T, w selectionWorld) map[string]string {
	t.Helper()
	root := worldRoot(t, w)
	tree := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		switch {
		case entry.IsDir():
			tree[relative] = "dir"
		case info.Mode()&os.ModeSymlink != 0:
			link, _ := os.Readlink(path)
			tree[relative] = "link " + link
		default:
			data, err := os.ReadFile(path)
			if err != nil {
				tree[relative] = "unreadable"
				return nil
			}
			sum := sha256.Sum256(data)
			tree[relative] = info.Mode().Perm().String() + " " + hex.EncodeToString(sum[:8])
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

// treeDiff names the paths that differ between two trees: "+" for a path that
// is new, "~" for one whose content or mode changed, "-" for one that is gone.
func treeDiff(before, after map[string]string) []string {
	var changed []string
	for path, value := range after {
		if earlier, ok := before[path]; !ok {
			changed = append(changed, "+"+path)
		} else if earlier != value {
			changed = append(changed, "~"+path)
		}
	}
	for path := range before {
		if _, ok := after[path]; !ok {
			changed = append(changed, "-"+path)
		}
	}
	slices.Sort(changed)
	return changed
}

// ranIn is one command run in one world: what it returned and wrote, the
// client before and after it, and every path below the world's root that it
// changed.
type ranIn struct {
	err           error
	before, after selectionSnapshot
	changed       []string
	output        string
}

// runIn runs args in the world. The command may fail; the caller decides.
func runIn(t *testing.T, w selectionWorld, args ...string) ranIn {
	t.Helper()
	run := ranIn{before: w.snapshot(t)}
	tree := worldTree(t, w)
	w.output.Reset()
	run.err = cli.Execute(w.app, args)
	run.output = w.output.String()
	run.after = w.snapshot(t)
	run.changed = treeDiff(tree, worldTree(t, w))
	return run
}

// decode reads the run's output as one JSON document into document.
func (run ranIn) decode(t *testing.T, document any) {
	t.Helper()
	if err := json.Unmarshal([]byte(run.output), document); err != nil {
		t.Fatalf("the output is not one JSON document: %v\n%s", err, run.output)
	}
}

// shownLine renders a snapshot on one line for a failure's message: the
// stored selection, and for each file what it selects and whether AIGW's
// record of it names that.
func shownLine(s selectionSnapshot) string {
	parts := []string{"stored " + s.stored}
	for index := range s.shows {
		record := "its record names another"
		switch {
		case s.files[index] == nil:
			record = "the file is missing"
		case s.records[index] == nil:
			record = "no record"
		case s.follows[index]:
			record = "its record names the same"
		}
		parts = append(parts, "file "+string(rune('A'+index))+" selects "+s.shows[index]+" ("+record+")")
	}
	return strings.Join(parts, "; ")
}

// settledOn reports whether the snapshot is the end that a followed choice
// must reach: selection, in the words of the world's shows, is stored, every
// file of the client selects it, and every record names it.
func settledOn(s selectionSnapshot, selection string) bool {
	if s.stored != selection {
		return false
	}
	for index := range s.shows {
		if s.shows[index] != selection || !s.follows[index] {
			return false
		}
	}
	return true
}

// followedOf returns the followed selection that a document's entries name
// for client, and whether they name one.
func followedOf(entries []followed, client string) (followed, bool) {
	index := slices.IndexFunc(entries, func(entry followed) bool { return entry.Client == client })
	if index < 0 {
		return followed{}, false
	}
	return entries[index], true
}

// forwardedThenChose is the state that the forwarding tests start from.
// Hermes's stored selection, dmxapi/gpt-6.2-sol on the Responses protocol,
// forwards through a local forwarding service; Hermes's file selects that
// model, and AIGW's record names it. Then the person picks claude-opus-5-5 of
// account on the Anthropic protocol in Hermes: the file selects it, and the
// record and the stored selection stay. The file still holds the forwarding
// address.
func forwardedThenChose(t *testing.T, account string) selectionWorld {
	t.Helper()
	world := hermesForwarding(t).choosing(t, account, "anthropic", "claude-opus-5-5", plainScalars)
	if !forwardsInFile(t, world) {
		t.Fatal("the fixture's file does not hold the forwarding address")
	}
	return world
}

// forwardsInFile reports whether Hermes's file holds the address that
// hermesForwarding forwards to.
func forwardsInFile(t *testing.T, w selectionWorld) bool {
	t.Helper()
	_, address, _ := strings.Cut(hermesForwardedTo, "://")
	return bytes.Contains(readFile(t, w.files[0]), []byte(address))
}

// pickShown selects shown, in the words of the Hermes world's shows
// (<entry>/<model>), in the world's file, as the person does in Hermes's
// model picker. Hermes offers only the entries that are in its file, and AIGW
// writes only the entries of providers whose Token it can use: where the
// entry is not in the file nothing can be picked, and pickShown reports false.
func pickShown(t *testing.T, w selectionWorld, shown string) bool {
	t.Helper()
	entry, model, _ := strings.Cut(shown, "/")
	var document struct {
		Providers map[string]yaml.Node `yaml:"providers"`
	}
	if err := yaml.Unmarshal(readFile(t, w.files[0]), &document); err != nil {
		t.Fatalf("Hermes's file %s is not YAML: %v", w.files[0], err)
	}
	if _, listed := document.Providers[entry]; !listed {
		return false
	}
	hermesNativeSelection(t, w.files[0], entry, model, nil)
	return true
}
