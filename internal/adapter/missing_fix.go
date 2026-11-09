package adapter

import (
	"errors"
	"os"
	"path/filepath"

	"aigw-cli/internal/platform"
)

// MissingTargetFix is the fix for a client file at path that does not exist. A
// client file and the folder that holds it both come from the operator, since
// AIGW creates neither a client's home nor the file in it, so the fix names
// both: creating only the folder leaves the same command refused, and a folder
// that is there leaves only the file. On Windows a
// path through a regular file reads as a missing file, so the file in the way
// is named to move first, and a symbolic link that leads nowhere is named to
// remove, because a create through it fails with "file exists". A file inside
// home is named as ~/…; one outside it by its own name, so a local check
// exposes no private path.
func MissingTargetFix(path, home string) string {
	path = filepath.Clean(path)
	file := namedPath(path, home)
	if failure, atFile := errors.AsType[*os.PathError](platform.FileInTheWay(path)); atFile {
		return "Move the file " + namedPath(failure.Path, home) + ", then create " + file + " and the folder that holds it, or choose another path, then run the same command again."
	}
	if link, atLink := platform.DanglingLinkInPath(filepath.Dir(path)); atLink {
		return "Remove the link " + namedPath(link, home) + ", then create " + file + " and the folder that holds it, or choose another path, then run the same command again."
	}
	if info, err := os.Stat(filepath.Dir(path)); err == nil && info.IsDir() {
		return "Create " + file + ", or choose another path, then run the same command again."
	}
	return "Create " + file + " and the folder that holds it, or choose another path, then run the same command again."
}

// Named names path as a report names it: relative to home as ~/…, and a path
// in a home as its own name, so that no private folder appears. A caller that
// names no home renders the path as configured.
func Named(path, home string) string { return namedPath(path, home) }

// namedPath names path relative to home as ~/…; a file in a home that the
// caller knows is named by its own name, so a local check exposes no private
// folder, and a caller that names no home renders the path as configured.
func namedPath(path, home string) string {
	if target, _ := platform.HomeTarget(path, home); target != "" {
		return target
	}
	if home == "" {
		return path
	}
	return filepath.Base(path)
}
