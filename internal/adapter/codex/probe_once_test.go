package codex

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"aigw-cli/internal/platform/processtest"
)

// probeCase is a Codex program for the tests of one probe per program: a
// package whose manifest declares version 1.0.0, its program, which notes each
// run in the file runs and prints table as its model table, and a link to the
// program.
type probeCase struct {
	program, manifest, link, script, runs string
}

func newProbeCase(t *testing.T) probeCase {
	t.Helper()
	root := t.TempDir()
	table := filepath.Join(root, "models.json")
	writeTo(t, table, windowedTable)
	runs := filepath.Join(root, "runs")
	script := "#!/bin/sh\necho \"$*\" >> '" + runs + "'\ncase \"$1\" in\n--version) echo \"codex-cli 1.0.0\" ;;\ndebug) cat '" + table + "' ;;\n*) exit 2 ;;\nesac\n"
	pkg := filepath.Join(root, "pkg")
	manifest := filepath.Join(pkg, "codex-package.json")
	writeTo(t, manifest, `{"version":"1.0.0"}`)
	program := filepath.Join(pkg, "bin", "codex")
	processtest.Write(t, program, script)
	link := filepath.Join(root, "link", "codex")
	pointTo(t, link, program)
	return probeCase{program: program, manifest: manifest, link: link, script: script, runs: runs}
}

// pointTo makes link a symbolic link to target, replacing a link already there.
func pointTo(t *testing.T, link, target string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(link), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(link); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}

// started returns what the case's program was asked, one run a line.
func (c probeCase) started(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(c.runs)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

var oneProbe = []string{"--version", "debug models --bundled"}

// asNewProcess forgets the probes of earlier commands, since the process of a
// command starts with none. A fixture that runs several commands in one test
// process calls it before each, so that a change that a probe's key does not
// cover, such as a launcher's target that goes, is seen as a new command sees
// it. No test of this package runs in parallel.
func asNewProcess() { bundledProbes = newBundledProbeSet() }

// Three reads of an unchanged program start it once for its version and once
// for its table, and each read receives the identity and the table that the
// one probe measured.
func TestAnUnchangedProgramIsProbedOncePerProcess(t *testing.T) {
	c := newProbeCase(t)
	first, table, err := bundledCatalogOnce(c.program)
	if err != nil || !first.known() || !bytes.Equal(table, []byte(windowedTable)) {
		t.Fatalf("first read = %+v, %d bytes, %v; want the program's identity and table", first, len(table), err)
	}
	for read := range 2 {
		client, again, err := bundledCatalogOnce(c.program)
		if err != nil || client != first || !bytes.Equal(again, table) {
			t.Fatalf("read %d = %+v, %d bytes, %v; want the first read's answer", read+2, client, len(again), err)
		}
	}
	if runs := c.started(t); !slices.Equal(runs, oneProbe) {
		t.Fatalf("the program ran %q, want one probe %q", runs, oneProbe)
	}
}

// A change of the program's bytes, of the version that its package declares,
// of the file that its configured path resolves to or of the configured path
// itself starts the program again at the next read.
func TestAChangedProgramIsProbedAgain(t *testing.T) {
	for name, test := range map[string]struct {
		// via returns the path of the first read; change changes the program
		// and returns the path of the second.
		via, change func(t *testing.T, c probeCase) string
	}{
		"its bytes": {
			via: func(_ *testing.T, c probeCase) string { return c.program },
			change: func(t *testing.T, c probeCase) string {
				appendTo(t, c.program, "# another build\n")
				return c.program
			},
		},
		"the version that its package declares": {
			via: func(_ *testing.T, c probeCase) string { return c.program },
			change: func(t *testing.T, c probeCase) string {
				writeTo(t, c.manifest, `{"version":"1.1.0"}`)
				return c.program
			},
		},
		"the file that its path resolves to": {
			// The same bytes in another folder, with no manifest beside
			// either, so that only the resolved file differs.
			via: func(t *testing.T, c probeCase) string {
				if err := os.Remove(c.manifest); err != nil {
					t.Fatal(err)
				}
				return c.link
			},
			change: func(t *testing.T, c probeCase) string {
				moved := filepath.Join(t.TempDir(), "codex")
				processtest.Write(t, moved, c.script)
				pointTo(t, c.link, moved)
				return c.link
			},
		},
		"its configured path": {
			via: func(_ *testing.T, c probeCase) string { return c.link },
			change: func(t *testing.T, c probeCase) string {
				other := filepath.Join(t.TempDir(), "codex")
				pointTo(t, other, c.program)
				return other
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			c := newProbeCase(t)
			if _, _, err := bundledCatalogOnce(test.via(t, c)); err != nil {
				t.Fatal(err)
			}
			if _, _, err := bundledCatalogOnce(test.change(t, c)); err != nil {
				t.Fatal(err)
			}
			if runs, want := c.started(t), slices.Concat(oneProbe, oneProbe); !slices.Equal(runs, want) {
				t.Fatalf("the program ran %q, want two probes", runs)
			}
		})
	}
}

// A program that reports its version and cannot print its table fails each
// read in the process the same way, with the identity that it measured, after
// one probe.
func TestAFailedProbeIsKept(t *testing.T) {
	c := newProbeCase(t)
	if err := os.Remove(filepath.Join(filepath.Dir(c.runs), "models.json")); err != nil {
		t.Fatal(err)
	}
	first, _, firstErr := bundledCatalogOnce(c.program)
	if firstErr == nil || !first.known() {
		t.Fatalf("first read = %+v, %v; want the measured identity and a failure", first, firstErr)
	}
	client, data, err := bundledCatalogOnce(c.program)
	if client != first || data != nil || err == nil || err.Error() != firstErr.Error() {
		t.Fatalf("second read = %+v, %d bytes, %v; want the first read's failure %v", client, len(data), err, firstErr)
	}
	if runs := c.started(t); !slices.Equal(runs, oneProbe) {
		t.Fatalf("the program ran %q, want one probe %q", runs, oneProbe)
	}
}

// A configured path that names no file fails the read and keeps nothing, so a
// program written there later is probed; an empty path is not configured.
func TestAnUnreadableProgramIsNotKept(t *testing.T) {
	if _, _, err := bundledCatalogOnce(" "); err == nil || err.Error() != "Codex executable is not configured" {
		t.Fatalf("read of an empty path = %v, want it not configured", err)
	}
	c := newProbeCase(t)
	absent := filepath.Join(t.TempDir(), "codex")
	if _, _, err := bundledCatalogOnce(absent); err == nil || !strings.HasPrefix(err.Error(), "read Codex executable: ") {
		t.Fatalf("read of a missing program = %v, want that it cannot be read", err)
	}
	if runs := c.started(t); runs != nil {
		t.Fatalf("a read of a missing program ran %q", runs)
	}
	processtest.Write(t, absent, c.script)
	if _, _, err := bundledCatalogOnce(absent); err != nil {
		t.Fatal(err)
	}
	if runs := c.started(t); !slices.Equal(runs, oneProbe) {
		t.Fatalf("the program written later ran %q, want one probe %q", runs, oneProbe)
	}
}

// Reads that overlap the probe of one program wait for it: while the probe is
// held open and every other reader waits, one probe has started; once it
// ends, every reader receives its answer.
func TestConcurrentReadsOfOneProgramShareOneProbe(t *testing.T) {
	c := newProbeCase(t)
	original, originalProbes := readBundledCatalog, bundledProbes
	t.Cleanup(func() { readBundledCatalog, bundledProbes = original, originalProbes })
	bundledProbes = newBundledProbeSet()
	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})
		var probes atomic.Int32
		readBundledCatalog = func(string) (ExecutableIdentity, []byte, error) {
			probes.Add(1)
			<-release
			return ExecutableIdentity{Version: "codex-cli 1.0.0", SHA256: "digest"}, []byte("table"), errors.New("kept")
		}
		type answer struct {
			client ExecutableIdentity
			data   []byte
			err    error
		}
		const readers = 4
		answers := make(chan answer, readers)
		for range readers {
			go func() {
				client, data, err := bundledCatalogOnce(c.program)
				answers <- answer{client, data, err}
			}()
		}
		synctest.Wait()
		if started := probes.Load(); started != 1 {
			t.Fatalf("%d probes started while one was held open, want 1", started)
		}
		close(release)
		for range readers {
			got := <-answers
			if got.client.Version != "codex-cli 1.0.0" || string(got.data) != "table" || got.err == nil || got.err.Error() != "kept" {
				t.Fatalf("a reader received %+v, want the one probe's answer", got)
			}
		}
	})
}

// A caller that changes the table it received does not change what a later
// read receives.
func TestTheReturnedTableIsACopy(t *testing.T) {
	c := newProbeCase(t)
	_, table, err := bundledCatalogOnce(c.program)
	if err != nil || len(table) == 0 {
		t.Fatal(err)
	}
	table[0] = 'X'
	if _, again, _ := bundledCatalogOnce(c.program); !bytes.Equal(again, []byte(windowedTable)) {
		t.Fatalf("a later read received %q..., want the program's table", again[:min(len(again), 16)])
	}
}

// A sync at the adapter layer, which plans and then applies Codex's files,
// starts an unchanged Codex once for its version and once for its table.
func TestASyncStartsAnUnchangedCodexOnce(t *testing.T) {
	fixture := newCodexProgram(t, caskLayout)
	if _, err := fixture.sync(t); err != nil {
		t.Fatal(err)
	}
	if runs := fixture.runs(t); !slices.Equal(runs, oneProbe) {
		t.Fatalf("the sync ran %q, want one probe %q", runs, oneProbe)
	}
}
