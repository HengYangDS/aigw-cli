package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPredecessorCustodyIgnoresExecutableIndependentBackends(t *testing.T) {
	absent := filepath.Join(t.TempDir(), "absent")
	if err := RestorePredecessorCredentials(NewMemoryStore(), absent, []string{"team"}, "0.3.5"); err != nil {
		t.Fatalf("memory backend restore = %v", err)
	}
	if err := ReclaimPredecessorCredentials(NewMemoryStore(), absent, []string{"team"}); err != nil {
		t.Fatalf("memory backend reclaim = %v", err)
	}
}

func TestPredecessorCustodyConvertsEachKeychainSlot(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("only macOS Keychain items authorize the reading executable")
	}
	if err := RestorePredecessorCredentials(nil, "", nil, ""); err == nil {
		t.Fatal("restore accepted an unavailable credential backend")
	}
	if err := ReclaimPredecessorCredentials(nil, "", nil); err == nil {
		t.Fatal("reclaim accepted an unavailable credential backend")
	}
	root := t.TempDir()
	record := filepath.Join(root, "requests")
	every := []string{"team", "diagnostic@team", "other", "diagnostic@other"}
	for _, test := range []struct {
		name    string
		version string
		reclaim bool
		code    string
		want    []string
	}{
		{name: "predecessor release", version: "0.3.5", code: "0", want: every},
		{name: "unknown release", code: "0", want: every},
		{name: "owner release", version: "0.4.0", code: "0"},
		{name: "later release", version: "v0.4.1", code: "0"},
		{name: "refused", version: "0.3.5", code: "3", want: []string{"team"}},
		{name: "reclaimed", reclaim: true, code: "0", want: every},
		{name: "reclaim refused", reclaim: true, code: "3", want: []string{"team"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			_ = os.Remove(record)
			program := filepath.Join(root, "aigw-"+strings.ReplaceAll(test.name, " ", "-"))
			script := "#!/bin/sh\nprintf '%s %s\\n' \"$1\" \"$3\" >> '" + record + "'\nexit " + test.code + "\n"
			if err := os.WriteFile(program, []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			store, err := Select(Selection{Backend: "keyring", GOOS: "darwin", Root: root, Executable: program})
			if err != nil {
				t.Fatal(err)
			}
			operation := "__aigw-native-credential-restore"
			if test.reclaim {
				operation = "__aigw-native-credential-reclaim"
				err = ReclaimPredecessorCredentials(store, program, []string{"team", "other"})
			} else {
				err = RestorePredecessorCredentials(store, program, []string{"team", "other"}, test.version)
			}
			if test.code == "0" && err != nil || test.code != "0" && (err == nil || !strings.Contains(err.Error(), "team")) {
				t.Fatalf("custody conversion = %v", err)
			}
			want := make([]string, 0, len(test.want))
			for _, slot := range test.want {
				want = append(want, operation+" "+slot)
			}
			requests, readErr := os.ReadFile(record)
			if readErr != nil && !errors.Is(readErr, os.ErrNotExist) || strings.TrimSpace(string(requests)) != strings.Join(want, "\n") {
				t.Fatalf("custody requested %q, want %q (%v)", requests, want, readErr)
			}
		})
	}
}
