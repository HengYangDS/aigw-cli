package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRestorePredecessorCredentialsIgnoresExecutableIndependentBackends(t *testing.T) {
	if err := RestorePredecessorCredentials(NewMemoryStore(), filepath.Join(t.TempDir(), "absent"), []string{"team"}); err != nil {
		t.Fatalf("memory backend restore = %v", err)
	}
}

func TestRestorePredecessorCredentialsConvertsEachKeychainSlot(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("only macOS Keychain items authorize the reading executable")
	}
	if err := RestorePredecessorCredentials(nil, "", nil); err == nil {
		t.Fatal("restore accepted an unavailable credential backend")
	}
	root := t.TempDir()
	record := filepath.Join(root, "requests")
	for _, test := range []struct {
		name string
		code string
		want []string
	}{
		{name: "restored", code: "0", want: []string{"team", "diagnostic@team", "other", "diagnostic@other"}},
		{name: "refused", code: "3", want: []string{"team"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			_ = os.Remove(record)
			program := filepath.Join(root, "aigw-"+test.name)
			script := "#!/bin/sh\nprintf '%s\\n' \"$3\" >> '" + record + "'\nexit " + test.code + "\n"
			if err := os.WriteFile(program, []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			store, err := Select(Selection{Backend: "keyring", GOOS: "darwin", Root: root, Executable: program})
			if err != nil {
				t.Fatal(err)
			}
			err = RestorePredecessorCredentials(store, program, []string{"team", "other"})
			if test.code == "0" && err != nil || test.code != "0" && (err == nil || !strings.Contains(err.Error(), "team")) {
				t.Fatalf("restore = %v", err)
			}
			requests, readErr := os.ReadFile(record)
			if readErr != nil && !errors.Is(readErr, os.ErrNotExist) || strings.Join(strings.Fields(string(requests)), " ") != strings.Join(test.want, " ") {
				t.Fatalf("restore requested %q, want %q (%v)", requests, test.want, readErr)
			}
		})
	}
}
