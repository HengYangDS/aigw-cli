// Package claude owns Claude Code route planning and its native integration.
// It never persists provider credentials or modifies user shell profiles.
package claude

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Ready reports whether executable resolves to a runnable native client.
func Ready(executable string) (bool, error) {
	if strings.TrimSpace(executable) == "" {
		return false, nil
	}
	info, err := os.Stat(executable)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect Claude executable: %w", err)
	}
	if info.IsDir() {
		return false, nil
	}
	if executableRequiresModeBit(executable) && info.Mode().Perm()&0o111 == 0 {
		return false, nil
	}
	return true, nil
}

func executableRequiresModeBit(path string) bool {
	extension := strings.ToLower(filepath.Ext(path))
	return extension != ".exe" && extension != ".cmd" && extension != ".bat"
}
