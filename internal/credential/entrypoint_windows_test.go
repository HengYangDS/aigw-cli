//go:build windows

package credential

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestEntrypointRejectsWritableWindowsDirectoryACL(t *testing.T) {
	tests := []struct {
		name      string
		directory func(string) string
	}{
		{"data directory", func(target string) string { return filepath.Dir(filepath.Dir(target)) }},
		{"credential directory", filepath.Dir},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "source.exe")
			target := filepath.Join(root, "data", "credential", "aigw.exe")
			if err := os.WriteFile(source, []byte("fixture"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				t.Fatal(err)
			}
			allowEveryoneFullControl(t, test.directory(target))
			if _, err := EnsureEntrypoint(source, target); err == nil {
				t.Fatal("world-writable credential directory was accepted")
			}
			if _, err := os.Lstat(target); !os.IsNotExist(err) {
				t.Fatalf("rejected directory received executable: %v", err)
			}
		})
	}
}

func TestEntrypointRejectsWritableWindowsExecutableACL(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.exe")
	target := filepath.Join(root, "data", "credential", "aigw.exe")
	if err := os.WriteFile(source, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureEntrypoint(source, target); err != nil {
		t.Fatal(err)
	}
	allowEveryoneFullControl(t, target)
	if _, err := EntrypointNeeded(target); err == nil {
		t.Fatal("world-writable credential executable was accepted")
	}
}

func TestWindowsCredentialOwnerMatchesTokenOwnerPrincipals(t *testing.T) {
	user, err := windows.StringToSid("S-1-5-21-1-2-3-1001")
	if err != nil {
		t.Fatal(err)
	}
	group, err := windows.StringToSid("S-1-5-32-544")
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := windows.StringToSid("S-1-5-21-1-2-3-1002")
	if err != nil {
		t.Fatal(err)
	}
	untrustedGroup, err := windows.CreateWellKnownSid(windows.WinBuiltinUsersSid)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		owner  *windows.SID
		user   *windows.SID
		groups []windows.SIDAndAttributes
		want   bool
	}{
		{name: "user", owner: user, user: user, want: true},
		{name: "owner-capable group", owner: group, user: user, groups: []windows.SIDAndAttributes{{Sid: group, Attributes: windows.SE_GROUP_OWNER}}, want: true},
		{name: "deny-only group", owner: group, user: user, groups: []windows.SIDAndAttributes{{Sid: group, Attributes: windows.SE_GROUP_OWNER | windows.SE_GROUP_USE_FOR_DENY_ONLY}}},
		{name: "group without owner capability", owner: group, user: user, groups: []windows.SIDAndAttributes{{Sid: group, Attributes: windows.SE_GROUP_ENABLED}}},
		{name: "untrusted owner-capable group", owner: untrustedGroup, user: user, groups: []windows.SIDAndAttributes{{Sid: untrustedGroup, Attributes: windows.SE_GROUP_OWNER}}},
		{name: "foreign owner", owner: foreign, user: user, groups: []windows.SIDAndAttributes{{Sid: group, Attributes: windows.SE_GROUP_OWNER}}},
		{name: "missing owner", user: user},
		{name: "missing token user", owner: group},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := ownerMatchesWindowsToken(test.owner, test.user, test.groups); got != test.want {
				t.Fatalf("owner membership = %t, want %t", got, test.want)
			}
		})
	}
}

func allowEveryoneFullControl(t *testing.T, path string) {
	t.Helper()
	descriptor, err := windows.SecurityDescriptorFromString("D:(A;;FA;;;WD)")
	if err != nil {
		t.Fatal(err)
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		dacl,
		nil,
	); err != nil {
		t.Fatal(err)
	}
}
