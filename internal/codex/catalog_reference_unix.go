//go:build !windows

package codex

import "path/filepath"

// resolveCatalogReference resolves a non-absolute model_catalog_json reference
// the way Codex does: from the directory of the configuration that names it.
func resolveCatalogReference(dir, reference string) string {
	return filepath.Join(dir, reference)
}
