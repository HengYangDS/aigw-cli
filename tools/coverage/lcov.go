package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/tools/cover"
)

// sourcePackage is the part of `go list -json` that names a package's files.
type sourcePackage struct {
	ImportPath     string   `json:"ImportPath"`
	Dir            string   `json:"Dir"`
	GoFiles        []string `json:"GoFiles"`
	CgoFiles       []string `json:"CgoFiles"`
	IgnoredGoFiles []string `json:"IgnoredGoFiles"`
}

// sourceInventory holds the selected production files by repository path:
// the files that this platform's build compiles, keyed by the name that the
// coverage profile gives them, and the files that its build constraints exclude.
type sourceInventory struct {
	compiled map[string]string
	excluded []string
}

// listSources reads the selected packages' files from Go's own package
// inventory under the attempt's build selection.
func listSources(runner commandRunner, root string, selection, packages []string) (sourceInventory, error) {
	var output, diagnostics bytes.Buffer
	args := slices.Concat([]string{"list", "-json"}, selection, packages)
	if err := runner.Run("go", args, &output, &diagnostics); err != nil {
		return sourceInventory{}, fmt.Errorf("go list: %w: %s", err, &diagnostics)
	}
	inventory := sourceInventory{compiled: map[string]string{}}
	decoder := json.NewDecoder(&output)
	for {
		var pkg sourcePackage
		err := decoder.Decode(&pkg)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return sourceInventory{}, fmt.Errorf("parse go list: %w", err)
		}
		directory, err := filepath.Rel(root, pkg.Dir)
		if err != nil || pkg.ImportPath == "" || !filepath.IsLocal(directory) {
			return sourceInventory{}, fmt.Errorf("package %q at %q is outside the repository", pkg.ImportPath, pkg.Dir)
		}
		for _, name := range slices.Concat(pkg.GoFiles, pkg.CgoFiles) {
			inventory.compiled[pkg.ImportPath+"/"+name] = filepath.ToSlash(filepath.Join(directory, name))
		}
		for _, name := range pkg.IgnoredGoFiles {
			if !strings.HasSuffix(name, "_test.go") {
				inventory.excluded = append(inventory.excluded, filepath.ToSlash(filepath.Join(directory, name)))
			}
		}
	}
	if len(inventory.compiled) == 0 {
		return sourceInventory{}, fmt.Errorf("go list returned no production files")
	}
	return inventory, nil
}

// lineCounts derives each production file's executed lines from the attempt's
// statement blocks: a line counts once, with the highest count of the blocks
// that span it. A file in which the build instrumented no statement, or that
// the build excludes, has no lines.
func lineCounts(profilePath string, inventory sourceInventory) (map[string]map[int]int, error) {
	profiles, err := cover.ParseProfiles(profilePath)
	if err != nil {
		return nil, err
	}
	records := map[string]map[int]int{}
	for _, path := range slices.Concat(slices.Collect(maps.Values(inventory.compiled)), inventory.excluded) {
		records[path] = map[int]int{}
	}
	for _, profile := range profiles {
		path, selected := inventory.compiled[profile.FileName]
		if !selected {
			return nil, fmt.Errorf("coverage profile names %s outside the selected inventory", profile.FileName)
		}
		for _, block := range profile.Blocks {
			if block.NumStmt == 0 {
				continue
			}
			for line := block.StartLine; line <= block.EndLine; line++ {
				records[path][line] = max(records[path][line], block.Count)
			}
		}
	}
	return records, nil
}

// writeLCOV writes one line record per file, sorted by path; it states no
// branch or function figure.
func writeLCOV(writer io.Writer, records map[string]map[int]int) error {
	var report strings.Builder
	for _, path := range slices.Sorted(maps.Keys(records)) {
		lines := records[path]
		hit := 0
		fmt.Fprintf(&report, "SF:%s\n", path)
		for _, line := range slices.Sorted(maps.Keys(lines)) {
			fmt.Fprintf(&report, "DA:%d,%d\n", line, lines[line])
			if lines[line] > 0 {
				hit++
			}
		}
		fmt.Fprintf(&report, "LF:%d\nLH:%d\nend_of_record\n", len(lines), hit)
	}
	_, err := io.WriteString(writer, report.String())
	return err
}
