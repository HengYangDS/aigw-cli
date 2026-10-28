package codex

import (
	"path/filepath"
	"testing"
)

// Codex resolves model_catalog_json with Rust's Path::join. On Windows that
// keeps only the configuration directory's volume for a rooted reference and
// returns a reference with a volume but no root unchanged.
func TestCatalogReferenceFollowsCodexOnWindows(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "codex")
	volume := filepath.VolumeName(dir)
	for _, test := range []struct{ reference, want string }{
		{reference: `\catalogs\models.json`, want: volume + `\catalogs\models.json`},
		{reference: `/catalogs/models.json`, want: volume + `\catalogs\models.json`},
		{reference: `D:models.json`, want: `D:models.json`},
		{reference: `catalogs\models.json`, want: filepath.Join(dir, "catalogs", "models.json")},
	} {
		if got := resolveCatalogReference(dir, test.reference); got != test.want {
			t.Errorf("resolveCatalogReference(%q) = %q, want %q", test.reference, got, test.want)
		}
	}
}
