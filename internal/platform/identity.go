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
		resolved, err := filepath.EvalSymlinks(absolute)
		if err == nil {
			for _, segment := range slices.Backward(missing) {
				resolved = filepath.Join(resolved, segment)
			}
			return filepath.Clean(resolved), nil
		}
		// A path through a regular file resolves to nothing, and on Windows it
		// reads as a missing path; the file system tells it from a missing one.
		failure := ClassifyPath(absolute, FileExpected, err)
		switch {
		case failure.Blocked():
			blocked := *failure
			blocked.Path = path
			return "", &blocked
		case !failure.Absent():
			return "", fmt.Errorf("resolve symbolic links: %w", err)
		}
		missing = append(missing, filepath.Base(absolute))
		absolute = filepath.Dir(absolute)
	}
}

// SameFile reports whether two configured paths name one file. Paths that
// both exist are one file only when the operating system says so, which
// covers a volume that ignores case, a symbolic link and a hard link, never a
// guess from the platform. Otherwise they are one file only when their
// canonical forms (CanonicalPath) are equal, which makes / and \ alike on
// Windows, resolves the links of the folders that exist and folds no case.
// Two spellings that clean to one path are one file, whatever their
// condition.
func SameFile(left, right string) bool {
	if filepath.Clean(left) == filepath.Clean(right) {
		return true
	}
	leftInfo, leftErr := statTarget(left)
	rightInfo, rightErr := statTarget(right)
	if leftErr == nil && rightErr == nil {
		return os.SameFile(leftInfo, rightInfo)
	}
	leftCanonical, leftErr := CanonicalPath(left)
	rightCanonical, rightErr := CanonicalPath(right)
	return leftErr == nil && rightErr == nil && leftCanonical == rightCanonical
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
