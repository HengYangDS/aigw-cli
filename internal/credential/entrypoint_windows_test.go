//go:build windows

package credential

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestEntrypointRejectsWritableWindowsACL(t *testing.T) {
	for _, subject := range []string{"data directory", "directory", "executable"} {
		t.Run(subject, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "source.exe")
			target := filepath.Join(root, "data", "credential", "aigw.exe")
			if err := os.WriteFile(source, []byte("fixture"), 0o700); err != nil {
				t.Fatal(err)
			}
			if subject != "executable" {
				if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
					t.Fatal(err)
				}
				if subject == "data directory" {
					allowEveryoneFullControl(t, filepath.Dir(filepath.Dir(target)))
				} else {
					allowEveryoneFullControl(t, filepath.Dir(target))
				}
				if _, err := EnsureEntrypoint(source, target); err == nil {
					t.Fatal("world-writable credential directory was accepted")
				}
				if _, err := os.Lstat(target); !os.IsNotExist(err) {
					t.Fatalf("rejected directory received executable: %v", err)
				}
				return
			}
			if _, err := EnsureEntrypoint(source, target); err != nil {
				t.Fatal(err)
			}
			allowEveryoneFullControl(t, target)
			if _, err := EntrypointNeeded(target); err == nil {
				t.Fatal("world-writable credential executable was accepted")
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
