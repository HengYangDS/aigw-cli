package recovery

import (
	"os"
	"path/filepath"
	"strings"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/transaction"
)

func recoveryOwnedPath(root, path string) bool {
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	return err == nil && relative != "." && relative != ".." &&
		!strings.HasPrefix(relative, ".."+string(os.PathSeparator)) && !filepath.IsAbs(relative)
}

func captureStorePath(s Store, path string) (transaction.FileSnapshot, error) {
	if recoveryOwnedPath(s.root, path) {
		return s.captureRecovery(path)
	}
	return s.capture(path)
}
