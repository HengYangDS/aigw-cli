package doctor

import (
	"path/filepath"
	"strings"
)

// publicTarget names a file under home relative to "~", as a shell prompt does.
// A file elsewhere is not named, so diagnostics never expose a private path;
// outside reports whether the file is known to lie outside home.
func publicTarget(path, home string) (target string, outside bool) {
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
	return filepath.Join("~", relative), false
}

// homeDirectory reads the user's home directory from the command environment.
func homeDirectory(env []string) string {
	for _, name := range []string{"HOME", "USERPROFILE"} {
		for _, entry := range env {
			if value, ok := strings.CutPrefix(entry, name+"="); ok && value != "" {
				return value
			}
		}
	}
	return ""
}
