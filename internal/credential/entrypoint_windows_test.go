//go:build windows

package credential

import (
	"os"
	"path/filepath"
	"strings"
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

func TestWindowsCredentialDescriptorOwnsItsNativeAuthorizationDecision(t *testing.T) {
	user, err := windows.StringToSid("S-1-5-21-1-2-3-1001")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		sddl string
		want string
	}{
		{name: "owner write", sddl: "O:" + user.String() + "D:(A;;FA;;;" + user.String() + ")"},
		{name: "foreign read", sddl: "O:" + user.String() + "D:(A;;FR;;;WD)"},
		{name: "deny and inherit only", sddl: "O:" + user.String() + "D:(D;;FA;;;WD)(A;IO;FA;;;WD)"},
		{name: "foreign write", sddl: "O:" + user.String() + "D:(A;;FW;;;WD)", want: "writable by another"},
		{name: "no restrictive ACL", sddl: "O:" + user.String(), want: "no restrictive ACL"},
		{name: "null ACL", sddl: "O:" + user.String() + "D:NO_ACCESS_CONTROL", want: "no restrictive ACL"},
		{name: "object-specific grant", sddl: "O:" + user.String() + "D:(OA;;FW;00000000-0000-0000-0000-000000000001;;WD)", want: "unsupported grant"},
		{name: "no owner", sddl: "D:(A;;FR;;;WD)", want: "no owner"},
		{name: "foreign owner", sddl: "O:WDD:(A;;FR;;;WD)", want: "untrusted Windows owner"},
	} {
		t.Run(test.name, func(t *testing.T) {
			descriptor, err := windows.SecurityDescriptorFromString(test.sddl)
			if err != nil {
				t.Fatal(err)
			}
			err = validateWindowsCredentialDescriptor(descriptor, user, nil)
			if test.want == "" && err != nil || test.want != "" && (err == nil || !strings.Contains(err.Error(), test.want)) {
				t.Fatalf("native descriptor decision = %v, want %q", err, test.want)
			}
		})
	}
	if err := validateWindowsCredentialDescriptor(nil, user, nil); err == nil {
		t.Fatal("missing native descriptor was accepted")
	}
	for _, path := range []string{"invalid\x00path", filepath.Join(t.TempDir(), "missing")} {
		if err := validateOwnedWindowsACL(path); err == nil {
			t.Fatalf("unobservable native credential path was accepted: %q", path)
		}
	}
	directory := t.TempDir()
	info, err := os.Stat(directory)
	if err != nil {
		t.Fatal(err)
	}
	if err := validatePrivateFile(directory, info, 0o700); err == nil || !strings.Contains(err.Error(), "not regular") {
		t.Fatalf("a directory acquired credential executable authorization: %v", err)
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
