//go:build !windows

package secret

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// A copied reader that macOS refuses a Token while the keychain is unlocked
// fails verification as the refusal of that Token's item, which names its
// fix. A locked keychain's refusal stays the reader's own.
func TestVerifyNativeReaderAccessNamesARefusedItem(t *testing.T) {
	for _, test := range []struct {
		diagnostic string
		refused    bool
	}{
		{`{"keychain_status":-25293,"keychain_unlocked":true}`, true},
		{`{"keychain_status":-25293}`, false},
	} {
		reader := filepath.Join(t.TempDir(), "aigw")
		script := "#!/bin/sh\nprintf '%s\\n' '" + test.diagnostic + "' >&2\nexit 3\n"
		if err := os.WriteFile(reader, []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
		err := VerifyNativeReaderAccess(scopedView{store: mockKeyringStore()}, reader, []string{"team"})
		refusal, typed := errors.AsType[*KeychainItemRefusedError](err)
		if !errors.Is(err, ErrNativeReaderUnverified) || typed != test.refused {
			t.Fatalf("%s: refusal %t, error %v", test.diagnostic, typed, err)
		}
		if typed && *refusal != (KeychainItemRefusedError{Kind: APIToken, Provider: "team", Status: -25293}) {
			t.Fatalf("refusal = %+v", *refusal)
		}
	}
}
