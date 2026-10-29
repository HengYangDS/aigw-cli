//go:build !windows

package secret

import (
	"os"
	"syscall"
	"testing"
	"time"
)

// foreignFile is a file or directory that another user owns.
type foreignFile struct{ mode os.FileMode }

func (file foreignFile) Name() string       { return "foreign" }
func (file foreignFile) Size() int64        { return 0 }
func (file foreignFile) Mode() os.FileMode  { return file.mode }
func (file foreignFile) ModTime() time.Time { return time.Time{} }
func (file foreignFile) IsDir() bool        { return file.mode.IsDir() }
func (file foreignFile) Sys() any {
	return &syscall.Stat_t{Uid: foreignOwner(), Nlink: 1}
}

// foreignOwner is a user ID other than the effective one.
func foreignOwner() uint32 {
	if os.Geteuid() == 1 {
		return 2
	}
	return 1
}

// TestOwnershipChecksNameWhatTheyRefuse pins both refusals of a path that
// another user owns: the Token store's files and the credential reader's.
func TestOwnershipChecksNameWhatTheyRefuse(t *testing.T) {
	t.Parallel()

	file, directory := foreignFile{mode: 0o600}, foreignFile{mode: os.ModeDir | 0o700}
	const tokens, reader = "Token storage is not owned by the current user", "credential entrypoint path is not owned by the current user"
	for name, test := range map[string]struct {
		check func() error
		want  string
	}{
		"Token file":           {func() error { return validateOwnedFile(file) }, tokens},
		"Token directory":      {func() error { return validateSecureRoot(directory) }, tokens},
		"credential directory": {func() error { return validatePrivateDirectory("", directory) }, reader},
		"credential reader":    {func() error { return validatePrivateFile("", foreignFile{mode: 0o700}, 0o700) }, reader},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := test.check(); err == nil || err.Error() != test.want {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}
