//go:build !windows

package secret

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// The reader check is the reader's: a copied reader that macOS refuses a
// Token fails verification as an unverified reader, with the keychain unlocked
// or locked, and never as the refusal of that Token's item. The readiness of
// a Token is decided before a reader is installed, where such an item leaves
// its client waiting.
func TestVerifyNativeReaderAccessKeepsARefusalTheReadersOwn(t *testing.T) {
	for _, diagnostic := range []string{
		`{"keychain_status":-25293,"keychain_unlocked":true}`,
		`{"keychain_status":-25293}`,
	} {
		reader := filepath.Join(t.TempDir(), "aigw")
		script := "#!/bin/sh\nprintf '%s\\n' '" + diagnostic + "' >&2\nexit 3\n"
		if err := os.WriteFile(reader, []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
		err := VerifyNativeReaderAccess(scopedView{store: mockKeyringStore()}, reader, []string{"team"})
		if _, typed := errors.AsType[*KeychainItemRefusedError](err); !errors.Is(err, ErrNativeReaderUnverified) || typed {
			t.Fatalf("%s: refusal of the item %t, error %v", diagnostic, typed, err)
		}
	}
}
