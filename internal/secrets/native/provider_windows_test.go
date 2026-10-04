//go:build windows

package native

import (
	"crypto/rand"
	"errors"
	"strings"
	"testing"

	"github.com/danieljoos/wincred"
	"golang.org/x/sys/windows"
)

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

func TestWindowsMetadataObservationTracksOnlyItsExactSessionSlot(t *testing.T) {
	service, account := "AIGW_TEST_METADATA_"+rand.Text(), "synthetic"
	credential := wincred.NewGenericCredential(service + ":" + account)
	credential.Persist = wincred.PersistSession
	credential.CredentialBlob = []byte("synthetic fixture")
	if err := credential.Write(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := credential.Delete(); err != nil && !errors.Is(err, windows.ERROR_NOT_FOUND) {
			t.Errorf("remove owned session credential: %v", err)
		}
		if present, err := observeCredential(service, account); err != nil || present {
			t.Errorf("owned session credential remains: present=%t error=%v", present, err)
		}
	})
	for _, test := range []struct {
		account string
		want    string
	}{
		{account: account, want: "1"},
		{account: "missing", want: "0"},
		{account: "*", want: "0"},
	} {
		if value, err := queryCredential(existsCommand, service, test.account, nil); err != nil || string(value) != test.want {
			t.Fatalf("exact metadata observation = %q, %v; want %q", value, err, test.want)
		}
	}
	if present, err := observeCredential(service, "invalid\x00target"); err == nil || present {
		t.Fatal("invalid native credential identity was accepted")
	}
}
