package processtest

import (
	"os"
	"testing"
)

// A test that names a path twice names one runnable file, and two paths with
// one base name name two files.
func TestProgramIsOneFileForEachTestAndPath(t *testing.T) {
	first, again, other := Program(t, "/opt/codex"), Program(t, "/opt/codex"), Program(t, "/opt/codex/codex")
	if first != again || first == other {
		t.Fatalf("Program() = %q, %q and %q", first, again, other)
	}
	for _, path := range []string{first, other} {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
			t.Fatalf("%s is not a runnable file: %v", path, err)
		}
	}
}
