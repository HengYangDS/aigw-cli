package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

// statTarget reads the identity that SameFile compares. A test replaces it to
// stand for a volume that keeps or ignores case.
var statTarget = os.Stat

// resolveLinks resolves a path's symbolic links for CanonicalPath. A test
// replaces it to stand for a volume whose root does not answer.
var resolveLinks = filepath.EvalSymlinks

// CanonicalPath is the one canonical form of a configured path: absolute,
// with the symbolic links of the part that exists resolved, keeping the names
// of the parts that do not exist yet, so a file has the same form before and
// after it is created. A file in the way or a link that leads nowhere refuses
// it, classified at path as given: nothing walks past or creates a folder
// through either one.
func CanonicalPath(path string) (string, error) {
	absolute, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("resolve the path: %w", err)
	}
	missing := []string{}
	for {
		resolved, err := resolveLinks(absolute)
		if err == nil {
			for _, segment := range slices.Backward(missing) {
				resolved = filepath.Join(resolved, segment)
			}
			return filepath.Clean(resolved), nil
		}
		// A path through a regular file resolves to nothing, and on Windows it
		// reads as a missing path; the file system tells it from a missing one.
		failure := ClassifyPath(absolute, FileExpected, err)
		parent := filepath.Dir(absolute)
		switch {
		case failure.Blocked(), failure.Absent() && parent == absolute:
			// A root that does not resolve, such as a share that does not
			// answer, ends the climb as missing, at the path as given.
			classified := *failure
			classified.Path = path
			return "", &classified
		case !failure.Absent():
			return "", fmt.Errorf("resolve symbolic links: %w", err)
		}
		missing = append(missing, filepath.Base(absolute))
		absolute = parent
	}
}

// SameFile reports whether two configured paths name one file, by what the
// operating system reports, never by a guess from the platform. Two paths are
// one file when the deepest part of each that exists is one entry by the
// operating system, which covers a symbolic link, a hard link and a volume
// that ignores case, and the names below those parts, of files and folders
// that do not exist yet, are equal; for two paths that both exist, that is
// the file itself. The part that exists must be a folder when names follow
// it. Two spellings that clean to one path are one file, whatever their
// condition, which makes / and \ alike on Windows.
func SameFile(left, right string) bool {
	if filepath.Clean(left) == filepath.Clean(right) {
		return true
	}
	leftEntry, leftNames, leftFound := deepestEntry(left)
	rightEntry, rightNames, rightFound := deepestEntry(right)
	return leftFound && rightFound && slices.Equal(leftNames, rightNames) && os.SameFile(leftEntry, rightEntry)
}

// deepestEntry returns what the operating system reports, through links, for
// the deepest part of path that exists, and the names below it, outermost
// first. found is false when no part exists, or when the part that exists is
// not a folder and names follow it.
func deepestEntry(path string) (entry os.FileInfo, names []string, found bool) {
	current, err := filepath.Abs(path)
	if err != nil {
		return nil, nil, false
	}
	for {
		info, err := statTarget(current)
		if err == nil {
			return info, names, len(names) == 0 || info.IsDir()
		}
		parent := filepath.Dir(current)
		if parent == current {
			return nil, nil, false
		}
		names = slices.Insert(names, 0, filepath.Base(current))
		current = parent
	}
}

// DroppedFiles returns the paths of before that name no file of after
// (SameFile), each file once by its first path, in before's order: the files
// that a change of a client's configuration from before to after drops,
// however either list spells them.
func DroppedFiles(before, after []string) []string {
	var dropped []string
	for _, path := range before {
		names := func(other string) bool { return SameFile(other, path) }
		if !slices.ContainsFunc(after, names) && !slices.ContainsFunc(dropped, names) {
			dropped = append(dropped, path)
		}
	}
	return dropped
}
