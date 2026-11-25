package artifact

import (
	"strings"
	"testing"
)

func TestArchiveVersionRejectsWrongTarget(t *testing.T) {
	if _, err := (Target{OS: "darwin", Arch: "arm64"}).Version("aigw_1.2.3_linux_amd64.tar.gz"); err == nil {
		t.Fatal("archiveVersion accepted a mismatched target")
	}
	if _, err := (Target{OS: "darwin", Arch: "arm64"}).Version("aigw_bogus_darwin_arm64.tar.gz"); err == nil {
		t.Fatal("archiveVersion accepted an invalid embedded version")
	}
	version, err := (Target{OS: "linux", Arch: "amd64"}).Version("aigw_1.2.3_linux_amd64.tar.gz")
	if err != nil || version != "1.2.3" {
		t.Fatalf("version = %q, err = %v", version, err)
	}
}

func TestExpectedBinaryPathFormatsComponents(t *testing.T) {
	got := (Target{OS: "linux", Arch: "amd64"}).programPath("1.2.3")
	want := "aigw_1.2.3_linux_amd64/aigw"
	if got != want {
		t.Fatalf("expectedBinaryPath = %q, want %q", got, want)
	}
}

// Every release archive is a gzip-compressed tar named by its target.
func TestEveryTargetArchivesAsTarGz(t *testing.T) {
	for _, target := range Targets() {
		name := target.ArchiveName("1.2.3")
		if name != "aigw_1.2.3_"+target.OS+"_"+target.Arch+".tar.gz" || !strings.HasSuffix(target.programPath("1.2.3"), "/aigw") {
			t.Errorf("%s/%s archives as %q with program %q", target.OS, target.Arch, name, target.programPath("1.2.3"))
		}
	}
}
