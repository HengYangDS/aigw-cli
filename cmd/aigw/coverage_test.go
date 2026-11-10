package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// coverageRoot names the directory in which each instrumented process that a
// script runs writes its coverage, in a directory of its own. Processes that
// share one coverage directory race to write its one meta-data file, and on
// Windows the rename of the process that loses fails.
const coverageRoot = "AIGW_SCRIPT_COVERAGE_ROOT"

// isolateCoverage points this process's coverage at a new directory below the
// root that its script names, before the program runs: a test binary that
// stands in for a program writes its coverage when it exits, into GOCOVERDIR
// as it is then. A process outside a script, or without coverage, keeps its
// directory, and so does one whose own directory cannot be made.
func isolateCoverage() {
	root := os.Getenv(coverageRoot)
	if root == "" || os.Getenv("GOCOVERDIR") == "" {
		return
	}
	if directory, err := os.MkdirTemp(root, "process-"); err == nil {
		_ = os.Setenv("GOCOVERDIR", directory)
	}
}

// mergeCoverage copies the coverage that the script processes wrote below
// root into directory, the coverage directory that go test reads when the
// tests end: every counter file, whose name is unique to its process, and the
// meta-data file of a program that directory lacks, which is the same for
// every process of that program. The directory then holds what one shared
// directory would have held without the race. It returns the number of
// processes whose coverage it read.
func mergeCoverage(root, directory string) (int, error) {
	present := map[string]bool{}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return 0, err
	}
	for _, entry := range entries {
		present[entry.Name()] = true
	}
	processes, err := os.ReadDir(root)
	if err != nil {
		return 0, err
	}
	for _, process := range processes {
		files, err := os.ReadDir(filepath.Join(root, process.Name()))
		if err != nil {
			return 0, err
		}
		for _, file := range files {
			name := file.Name()
			meta := strings.HasPrefix(name, "covmeta.")
			switch {
			case !meta && !strings.HasPrefix(name, "covcounters."), meta && present[name]:
				continue
			case present[name]:
				return 0, fmt.Errorf("two processes wrote the coverage counters %s", name)
			}
			data, err := os.ReadFile(filepath.Join(root, process.Name(), name))
			if err != nil {
				return 0, err
			}
			if err := os.WriteFile(filepath.Join(directory, name), data, 0o600); err != nil {
				return 0, err
			}
			present[name] = true
		}
	}
	return len(processes), nil
}

func TestEachScriptProcessWritesItsOwnCoverage(t *testing.T) {
	root, shared := t.TempDir(), t.TempDir()
	t.Setenv(coverageRoot, root)
	t.Setenv("GOCOVERDIR", shared)
	isolateCoverage()
	own := os.Getenv("GOCOVERDIR")
	if parent := filepath.Dir(own); parent != root || own == shared {
		t.Fatalf("GOCOVERDIR = %s, want a new directory in %s", own, root)
	}
	t.Setenv(coverageRoot, "")
	t.Setenv("GOCOVERDIR", shared)
	isolateCoverage()
	if got := os.Getenv("GOCOVERDIR"); got != shared {
		t.Fatalf("a process outside a script moved its coverage to %s", got)
	}
}

func TestMergeCoverageKeepsEveryProcessCounters(t *testing.T) {
	root, directory := t.TempDir(), t.TempDir()
	write := func(path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(directory, "covmeta.a"), "meta")
	for process, files := range map[string][]string{
		"process-1": {"covmeta.a", "covcounters.a.1.1", "tmp.covmeta.a1"},
		"process-2": {"covmeta.a", "covcounters.a.2.2"},
		"process-3": {"covmeta.b", "covcounters.b.3.3"},
	} {
		for _, name := range files {
			write(filepath.Join(root, process, name), process+" "+name)
		}
	}
	merged, err := mergeCoverage(root, directory)
	if err != nil || merged != 3 {
		t.Fatalf("merged %d processes: %v", merged, err)
	}
	for name, want := range map[string]string{
		"covmeta.a": "meta", "covmeta.b": "process-3 covmeta.b",
		"covcounters.a.1.1": "process-1 covcounters.a.1.1", "covcounters.a.2.2": "process-2 covcounters.a.2.2",
		"covcounters.b.3.3": "process-3 covcounters.b.3.3",
	} {
		if data, err := os.ReadFile(filepath.Join(directory, name)); err != nil || string(data) != want {
			t.Errorf("%s = %q, %v; want %q", name, data, err, want)
		}
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 5 {
		t.Fatalf("the merged directory holds %d files, want 5: %v", len(entries), err)
	}
	write(filepath.Join(root, "process-4", "covcounters.a.1.1"), "again")
	if _, err := mergeCoverage(root, directory); err == nil || !strings.Contains(err.Error(), "covcounters.a.1.1") {
		t.Fatalf("a repeated counter file merged: %v", err)
	}
}
