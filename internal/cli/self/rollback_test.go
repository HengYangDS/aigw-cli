package self

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestRetainedVersionReadsTheBuildInformation builds a program that sets the
// version variable as an official build does, and one that does not, and
// reads each one's version from its build information without running it. A
// file that is no program has no build information to read.
func TestRetainedVersionReadsTheBuildInformation(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "main.go")
	if err := os.WriteFile(source, []byte("package main\n\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	build := func(name string, flags ...string) string {
		target := filepath.Join(root, name)
		command := exec.Command("go", append(append([]string{"build", "-o", target}, flags...), source)...)
		command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOPROXY=off", "GOWORK=off", "GOFLAGS=")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v: %s", name, err, output)
		}
		return target
	}
	if version, unread := retainedVersion(build("versioned", "-ldflags", "-s -w -X "+versionVariable+"9.8.7")); version != "9.8.7" || unread != "" {
		t.Fatalf("a program with a version = %q, %q", version, unread)
	}
	if version, unread := retainedVersion(build("plain")); version != "" || !strings.Contains(unread, "names no version") {
		t.Fatalf("a program without a version = %q, %q", version, unread)
	}
	notes := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(notes, []byte("not a program"), 0o600); err != nil {
		t.Fatal(err)
	}
	if version, unread := retainedVersion(notes); version != "" || !strings.Contains(unread, "cannot be read") {
		t.Fatalf("a file that is no program = %q, %q", version, unread)
	}
}
