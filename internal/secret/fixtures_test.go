package secret

import (
	"os"

	"aigw-cli/internal/platform"
)

// The constructors and wrappers below serve this package's tests only; the
// program reads its programs through ReadProgram and selects its backends
// through Select.

// NewEnvironmentStore constructs a read-only environment credential backend.
func NewEnvironmentStore(getenv func(string) string) Store {
	return scopedView{store: environmentStore{getenv: getenv}}
}

// VersionedEntrypointPath binds a private executable path to the exact source
// bytes without creating files or relying on an installer-owned public link.
func VersionedEntrypointPath(dataDir, source, installName string) (path string, err error) {
	program, err := ReadProgram(nil, source)
	if err != nil {
		return "", err
	}
	return program.EntrypointPath(dataDir, installName)
}

// EnsureEntrypoint retains one executable outside a package manager's CLI
// link. It never replaces an existing entrypoint during an ordinary sync.
// The returned compensation removes only bytes created by this operation.
func EnsureEntrypoint(source, target string) (func() error, error) {
	return ensureEntrypoint(source, target, platform.WriteFileAtomicExactModeIfUnchanged)
}

func ensureEntrypoint(
	source, target string,
	write func(string, platform.FileSnapshot, []byte, os.FileMode) (platform.FileSnapshot, error),
) (func() error, error) {
	program, err := ReadProgram(nil, source)
	if err != nil {
		return nil, err
	}
	_, undo, err := program.ensure(target, write)
	return undo, err
}
