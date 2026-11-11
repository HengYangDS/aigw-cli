package secret

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"aigw-cli/internal/platform"
)

// A failure to observe a slot is told from a closed set. A backend's own
// diagnostic stays out of it.
func TestObservationFailureNamesItsCauseAndNoPrivateDetail(t *testing.T) {
	cause, fix := ObservationFailure(fmt.Errorf("inspect: %w", unsafeTokenFileError{name: "orphan", message: "must be owner-only"}), platform.PathContext{})
	if cause != "must be owner-only" || !strings.HasPrefix(fix, "Make orphan in AIGW's Token directory "+tokenFileRequirement) {
		t.Fatalf("an unsafe file = %q, %q", cause, fix)
	}
	cause, fix = ObservationFailure(unsafeTokenDirectoryError{message: "must be a directory"}, platform.PathContext{})
	if cause != "must be a directory" || !strings.HasPrefix(fix, "Make AIGW's Token directory "+tokenDirectoryRequirement) {
		t.Fatalf("an unsafe directory = %q, %q", cause, fix)
	}
	cause, fix = ObservationFailure(errors.New("open /private/home/tokens: permission denied"), platform.PathContext{})
	if cause != "the credential backend is unavailable" || fix != BackendUnavailableFix {
		t.Fatalf("another failure = %q, %q", cause, fix)
	}
}

// A file that stands where AIGW's Token folder belongs, or a link that leads
// nowhere, is named with the one fix of its path's condition, which says how
// the Tokens come back, never folded into a backend that is unavailable.
func TestObservationFailureNamesABlockedTokenFolder(t *testing.T) {
	home := t.TempDir()
	secrets := filepath.Join(home, "aigw", "secrets")
	if err := os.MkdirAll(filepath.Dir(secrets), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secrets, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := newFileStore(filepath.Join(secrets, "tokens")).Exists("team")
	cause, fix := ObservationFailure(err, platform.PathContext{Home: home, Check: true})
	if cause != "AIGW's Token folder ~/aigw/secrets/tokens cannot be used: not a directory" ||
		fix != "Move the file ~/aigw/secrets. If it replaced AIGW's folder, put that folder back from a copy; otherwise store each provider's Token again with `aigw provider rotate <provider>`. Then run `aigw sync`." {
		t.Fatalf("a blocked Token folder = %q, %q", cause, fix)
	}
}

// A Token folder outside the home directory, as on Linux or Windows when
// AIGW's folder lies elsewhere, is not named: the sentence says that AIGW's
// Token folder cannot be used, and the fix points to `aigw config path`.
func TestObservationFailureOfATokenFolderOutsideHome(t *testing.T) {
	secrets := filepath.Join(t.TempDir(), "aigw", "secrets")
	if err := os.MkdirAll(filepath.Dir(secrets), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secrets, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := newFileStore(filepath.Join(secrets, "tokens")).Exists("team")
	cause, fix := ObservationFailure(err, platform.PathContext{Home: t.TempDir(), Check: true})
	if cause != "AIGW's Token folder cannot be used: not a directory" ||
		fix != "Move the file in the way in AIGW's folder, which `aigw config path` prints. If it replaced AIGW's folder, put that folder back from a copy; otherwise store each provider's Token again with `aigw provider rotate <provider>`. Then run `aigw sync`." {
		t.Fatalf("a Token folder outside home = %q, %q", cause, fix)
	}
}

// A permission error that a sandbox or the file's mode gives at AIGW's Token
// folder is the generic denial naming that folder, never "unlock the
// credential store" or no fix: the store is a folder, and its path is known.
func TestObservationFailureNamesADeniedTokenFolder(t *testing.T) {
	home := t.TempDir()
	tokens := filepath.Join(home, "aigw", "secrets", "tokens")
	denied := fmt.Errorf("inspect Token directory: %w", &fs.PathError{Op: "lstat", Path: tokens, Err: syscall.EPERM})
	if !NamesTokenFolder(denied) {
		t.Fatal("a denied Token folder is not named")
	}
	cause, fix := ObservationFailure(denied, platform.PathContext{Home: home, Check: true})
	if cause != "AIGW's Token folder ~/aigw/secrets/tokens cannot be read: operation not permitted" ||
		fix != "A lock, a sandbox or system protection forbids this operation on ~/aigw/secrets/tokens; allow it, then run `aigw sync`." {
		t.Fatalf("a denied Token folder = %q, %q", cause, fix)
	}
	for _, err := range []error{
		nil,
		errors.New("locked"),
		fmt.Errorf("read: %w", &fs.PathError{Op: "open", Path: "orphan", Err: syscall.EPERM}),
		&fs.PathError{Op: "lstat", Path: tokens, Err: syscall.EIO},
	} {
		if NamesTokenFolder(err) {
			t.Errorf("%v names a Token folder", err)
		}
	}
}

// A permission error at a Token file, such as a flag or a sandbox that
// refuses its replacement, names that Token file with its own fix, never the
// Token folder: the folder's path is the store's, and the file's is its slot
// inside it.
func TestObservationFailureNamesADeniedTokenFileNotTheFolder(t *testing.T) {
	home := t.TempDir()
	tokens := filepath.Join(home, "aigw", "secrets", "tokens")
	file := filepath.Join(tokens, "team")
	denied := fmt.Errorf("write Token file: %w", tokenFileFailure(&fs.PathError{Op: "open", Path: file, Err: syscall.EPERM}))
	if !NamesTokenFolder(denied) {
		t.Fatal("a denied Token file gets the generic denial")
	}
	cause, fix := ObservationFailure(denied, platform.PathContext{Home: home, Check: true})
	if cause != "AIGW's Token file ~/aigw/secrets/tokens/team cannot be used: operation not permitted" ||
		fix != "A lock, a sandbox or system protection forbids this operation on ~/aigw/secrets/tokens/team; allow it, then run `aigw sync`." {
		t.Fatalf("a denied Token file = %q, %q", cause, fix)
	}
	// The same path error from the folder's own operation still names the folder.
	folderCause, _ := ObservationFailure(fmt.Errorf("inspect Token directory: %w", &fs.PathError{Op: "lstat", Path: tokens, Err: syscall.EPERM}), platform.PathContext{Home: home, Check: true})
	if !strings.HasPrefix(folderCause, "AIGW's Token folder ~/aigw/secrets/tokens cannot be read") {
		t.Fatalf("a denied Token folder = %q", folderCause)
	}
}
