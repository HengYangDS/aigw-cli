package cli

import (
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/platform"
)

func TestDefaultShimDirectoryIsAIGWOwnedNotExecutableOrSharedUserBin(t *testing.T) {
	env := map[string]string{"HOME": "/Users/alex", "APPDATA": `C:\Users\alex\AppData\Roaming`, "LOCALAPPDATA": `C:\Users\alex\AppData\Local`}
	got, err := defaultShimDirFor("darwin", env, "/usr/local/bin/aigw")
	if err != nil {
		t.Fatal(err)
	}
	want, err := platform.ShimDirFor("darwin", env)
	if err != nil {
		t.Fatal(err)
	}
	if got != want || got == "/usr/local/bin" || got == "/Users/alex/.local/bin" {
		t.Fatalf("shim dir = %q, want %q and not executable or shared user bin", got, want)
	}
}
