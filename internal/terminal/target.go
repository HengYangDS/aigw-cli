package terminal

import (
	"path/filepath"
	"strings"
)

// HomeTarget names a file under home relative to "~", as a shell prompt does,
// with "/" between its parts on every platform. A file elsewhere is not
// named, so no document or diagnostic exposes a private path; outside reports
// whether the file is known to lie outside home. A file or home reached
// through a symbolic link, such as macOS's /var, still lies under home when
// both resolve to it.
func HomeTarget(path, home string) (target string, outside bool) {
	target, outside = homeTarget(path, home)
	// A file on another volume, such as a network share, stays outside
	// without a lookup there.
	if !outside || !strings.EqualFold(filepath.VolumeName(path), filepath.VolumeName(home)) {
		return target, outside
	}
	resolvedHome := home
	if resolved, err := filepath.EvalSymlinks(home); err == nil {
		resolvedHome = resolved
	}
	if under, beyond := homeTarget(location(path), resolvedHome); !beyond {
		return under, false
	}
	return target, outside
}

// location resolves the symbolic links of the longest existing directory
// that holds path, which may not exist yet, and keeps path's own name: a
// link at the file itself still names a file at this location.
func location(path string) string {
	directory, rest := filepath.Dir(path), filepath.Base(path)
	for {
		if resolved, err := filepath.EvalSymlinks(directory); err == nil {
			return filepath.Join(resolved, rest)
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return path
		}
		rest = filepath.Join(filepath.Base(directory), rest)
		directory = parent
	}
}

func homeTarget(path, home string) (target string, outside bool) {
	if path == "" || home == "" {
		return "", false
	}
	relative, err := filepath.Rel(home, path)
	switch {
	case err != nil:
		// Rel fails for absolute paths on different volumes, such as two
		// Windows drives or a drive and a UNC share, and compares volume names
		// without case; such a file is known to lie outside home. A relative
		// or invalid home leaves the location unknown.
		return "", filepath.IsAbs(home) && filepath.IsAbs(path) &&
			!strings.EqualFold(filepath.VolumeName(home), filepath.VolumeName(path))
	case relative == ".":
		return "", false
	case relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)):
		return "", true
	}
	return "~/" + filepath.ToSlash(relative), false
}

// HomeDirectory reads the user's home directory from a command environment.
func HomeDirectory(env []string) string {
	for _, name := range []string{"HOME", "USERPROFILE"} {
		for _, entry := range env {
			if value, ok := strings.CutPrefix(entry, name+"="); ok && value != "" {
				return value
			}
		}
	}
	return ""
}

// SnakeValue spells a state or kind in snake_case, as every JSON document
// does: the hyphens of an adapter's or a planner's own spelling become
// underscores.
func SnakeValue(value string) string {
	return strings.ReplaceAll(value, "-", "_")
}
