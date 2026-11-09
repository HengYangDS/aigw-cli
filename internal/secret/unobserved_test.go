package secret

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
