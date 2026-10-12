//go:build windows

package native

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeCredentialManagerPresencePreservesExactItemLifecycle(t *testing.T) {
	if os.Getenv("AIGW_VERIFY_SYSTEM_KEYRING") != "1" {
		t.Skip("native Credential Manager qualification was not selected")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	const service, token = "AIGW_TOKEN", "synthetic-native-presence-token"
	account := "native-presence-" + filepath.Base(filepath.Dir(t.TempDir())) + "-\u03bb"
	requirePresence := func(want bool) {
		t.Helper()
		value, err := queryCredential(existsCommand, service, account, nil)
		wantValue := "0"
		if want {
			wantValue = "1"
		}
		if err != nil || string(value) != wantValue {
			t.Fatalf("native credential metadata presence = %q, %v; want %t", value, err, want)
		}
		if present, err := Exists(executable, service, account); err != nil || present != want {
			t.Fatalf("bounded native credential presence = %t, %v; want %t", present, err, want)
		}
	}
	requirePresence(false)
	t.Cleanup(func() {
		if err := Delete(executable, service, account); err != nil {
			t.Errorf("remove owned native credential: %v", err)
		}
		requirePresence(false)
	})
	if err := Write(executable, service, account, token); err != nil {
		t.Fatal(err)
	}
	requirePresence(true)
	for _, selectedAccount := range []string{account + "-unselected", account + "*"} {
		if present, err := observeCredential(service, selectedAccount); err != nil || present {
			t.Fatalf("non-exact credential metadata presence = %t, %v", present, err)
		}
	}
	if value, err := Read(executable, service, account); err != nil || value != token {
		t.Fatal("native credential read did not preserve the owned synthetic value")
	}
	if err := Delete(executable, service, account); err != nil {
		t.Fatal(err)
	}
	requirePresence(false)
}

func TestCredentialManagerPresenceRejectsInvalidTarget(t *testing.T) {
	value, err := queryCredential(existsCommand, "AIGW_TOKEN", "invalid\x00target", nil)
	if value != nil || err == nil || !strings.Contains(err.Error(), "encode Windows credential target") {
		t.Fatalf("invalid native credential target returned metadata: %q, %v", value, err)
	}
}

func TestNativeEnvironmentRetainsOnlyCredentialManagerContext(t *testing.T) {
	values := map[string]string{
		"USERPROFILE":  `C:\Users\runner`,
		"APPDATA":      `C:\Users\runner\AppData\Roaming`,
		"LOCALAPPDATA": `C:\Users\runner\AppData\Local`,
		"SystemRoot":   `C:\Windows`,
		"HOMEDRIVE":    `C:`,
		"HOMEPATH":     `\Users\runner`,
		"PATH":         `C:\untrusted`,
	}
	want := strings.Join([]string{
		`USERPROFILE=C:\Users\runner`,
		`APPDATA=C:\Users\runner\AppData\Roaming`,
		`LOCALAPPDATA=C:\Users\runner\AppData\Local`,
		`SystemRoot=C:\Windows`,
		`HOMEDRIVE=C:`,
		`HOMEPATH=\Users\runner`,
	}, "\n")
	if got := strings.Join(nativeEnvironment(func(name string) string { return values[name] }), "\n"); got != want {
		t.Fatalf("native environment = %q, want %q", got, want)
	}
}
