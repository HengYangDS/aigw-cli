package artifact

import (
	"testing"
)

func TestArchiveVersionRejectsWrongTarget(t *testing.T) {
	if _, err := (Target{OS: "darwin", Arch: "arm64"}).Version("aigw_1.2.3_linux_amd64.tar.gz"); err == nil {
		t.Fatal("archiveVersion accepted a mismatched target")
	}
	if _, err := (Target{OS: "darwin", Arch: "arm64"}).Version("aigw_bogus_darwin_arm64.tar.gz"); err == nil {
		t.Fatal("archiveVersion accepted an invalid embedded version")
	}
	version, err := (Target{OS: "windows", Arch: "amd64"}).Version("aigw_1.2.3_windows_amd64.zip")
	if err != nil || version != "1.2.3" {
		t.Fatalf("version = %q, err = %v", version, err)
	}
}

func TestExpectedBinaryPathFormatsComponents(t *testing.T) {
	got := (Target{OS: "windows", Arch: "amd64"}).programPath("1.2.3")
	want := "aigw_1.2.3_windows_amd64/aigw.exe"
	if got != want {
		t.Fatalf("expectedBinaryPath = %q, want %q", got, want)
	}
}

func TestPortableArchiveNameUsesZipExtensionOnWindows(t *testing.T) {
	if got := (Target{OS: "windows", Arch: "amd64"}).ArchiveName("1.2.3"); got != "aigw_1.2.3_windows_amd64.zip" {
		t.Fatalf("portableArchiveName = %q", got)
	}
	if got := (Target{OS: "darwin", Arch: "arm64"}).ArchiveName("1.2.3"); got != "aigw_1.2.3_darwin_arm64.tar.gz" {
		t.Fatalf("portableArchiveName = %q", got)
	}
}
