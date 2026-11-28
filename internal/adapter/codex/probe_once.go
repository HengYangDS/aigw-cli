package codex

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"sync"
)

// bundledProbeKey names a Codex program as a probe of its bundled model table
// sees it (design D1 of codex-probe-once): the path as configured, which a
// launcher receives as its own name; the file that the path resolves to,
// beside which a launcher finds its package; the SHA-256 of that file, the
// identity that AIGW records for a table's producer; and the version that the
// package manifest declares, which stands for the package's other files.
type bundledProbeKey struct {
	program, resolved, digest, pkg string
}

// bundledProbe is the one probe of a key. done closes when the probe has
// ended; what it measured is read only after that.
type bundledProbe struct {
	done   chan struct{}
	client ExecutableIdentity
	data   []byte
	err    error
}

// bundledProbeSet holds a process's probes by key.
type bundledProbeSet struct {
	mu    sync.Mutex
	byKey map[bundledProbeKey]*bundledProbe
}

func newBundledProbeSet() *bundledProbeSet {
	return &bundledProbeSet{byKey: map[bundledProbeKey]*bundledProbe{}}
}

// entry returns the probe of key, and whether the caller created it and so
// runs it.
func (set *bundledProbeSet) entry(key bundledProbeKey) (probe *bundledProbe, created bool) {
	set.mu.Lock()
	defer set.mu.Unlock()
	if probe, found := set.byKey[key]; found {
		return probe, false
	}
	probe = &bundledProbe{done: make(chan struct{})}
	set.byKey[key] = probe
	return probe, true
}

// bundledProbes holds this process's probes. AIGW runs one process per
// command, so nothing here outlives the command that read it.
var bundledProbes = newBundledProbeSet()

// readBundledCatalog is the probe that bundledCatalogOnce keeps. A test
// replaces it to hold a probe open.
var readBundledCatalog = ReadBundledCatalog

// bundledCatalogOnce reads the bundled model table of the Codex program at
// program, starting an unchanged program once in the process: every later
// read of its key, and every read that overlaps its probe, receives the
// identity, a copy of the table and the failure that the one probe measured.
// A program whose key cannot be read, because its path is empty or names no
// readable file, is probed as before and nothing is kept; that probe starts
// no program, since the program's file is read before it starts.
func bundledCatalogOnce(program string) (ExecutableIdentity, []byte, error) {
	key, err := bundledProbeKeyOf(program)
	if err != nil {
		return readBundledCatalog(program)
	}
	probe, created := bundledProbes.entry(key)
	if created {
		func() {
			defer close(probe.done)
			probe.client, probe.data, probe.err = readBundledCatalog(program)
		}()
	} else {
		<-probe.done
	}
	return probe.client, bytes.Clone(probe.data), probe.err
}

// bundledProbeKeyOf reads the key of the program at program without starting
// it.
func bundledProbeKeyOf(program string) (bundledProbeKey, error) {
	if strings.TrimSpace(program) == "" {
		return bundledProbeKey{}, errors.New("Codex executable is not configured")
	}
	resolved, err := filepath.EvalSymlinks(program)
	if err != nil {
		return bundledProbeKey{}, err
	}
	digest, err := fileSHA256(resolved)
	if err != nil {
		return bundledProbeKey{}, err
	}
	return bundledProbeKey{program: program, resolved: resolved, digest: digest, pkg: packageVersion(program)}, nil
}
