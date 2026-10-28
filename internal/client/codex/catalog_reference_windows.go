package codex

import (
	"os"
	"path/filepath"
)

// resolveCatalogReference resolves a non-absolute model_catalog_json reference
// as Codex's Path::join does on Windows: a reference with a volume but no root
// stays as it is, a rooted one takes the volume of the configuration's
// directory, and any other is joined with that directory.
func resolveCatalogReference(dir, reference string) string {
	switch {
	case filepath.VolumeName(reference) != "":
		return reference
	case os.IsPathSeparator(reference[0]):
		return filepath.VolumeName(dir) + filepath.Clean(reference)
	default:
		return filepath.Join(dir, reference)
	}
}
