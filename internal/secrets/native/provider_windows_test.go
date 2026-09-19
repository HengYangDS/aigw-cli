//go:build windows

package native

import (
	"strings"
	"testing"
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
