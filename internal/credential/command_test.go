package credential

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandQuotesOneExecutableAndScopesCredentialLookup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "aigw client")
	command, err := Command(path, "hermes", "projection", "linux")
	if err != nil || command != "'"+path+"' credential hermes projection" {
		t.Fatalf("command = %q, %v", command, err)
	}
	command, err = Command(path, "claude", "projection", "windows")
	if err != nil || !strings.HasSuffix(command, `" credential claude projection`) {
		t.Fatalf("Windows command = %q, %v", command, err)
	}
	for _, path := range []string{"relative", "/bad\npath", "/bad%PATH%"} {
		if _, err := Command(path, "hermes", "projection", "windows"); err == nil {
			t.Fatalf("unsafe Windows path accepted: %q", path)
		}
	}
}
