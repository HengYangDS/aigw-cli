package codex

import (
	"slices"
	"strings"
	"testing"
)

// A check run from inside a Codex session starts Codex without the variables
// through which that session configures its own client, and keeps the user's
// own, such as the home, proxies and a CA bundle.
func TestCodexEnvironmentDropsAParentSessionsVariables(t *testing.T) {
	retained := map[string]string{"HOME": "/home/user", "HTTPS_PROXY": "http://proxy.test", "LANG": "C", "CODEX_CA_CERTIFICATE": "/etc/ca.pem"}
	for key, value := range retained {
		t.Setenv(key, value)
	}
	for _, key := range append(slices.Clone(parentSessionVariables), "CODEX_HOME") {
		t.Setenv(key, "parent")
	}
	got := codexEnvironment("/own/home", "")
	for _, key := range parentSessionVariables {
		if slices.ContainsFunc(got, func(entry string) bool { return strings.HasPrefix(entry, key+"=") }) {
			t.Errorf("%s stays", key)
		}
	}
	for key, value := range retained {
		if !slices.Contains(got, key+"="+value) {
			t.Errorf("%s was removed", key)
		}
	}
	if !slices.Contains(got, "CODEX_HOME=/own/home") || slices.Contains(got, "CODEX_HOME=parent") {
		t.Errorf("CODEX_HOME = %q", got)
	}
}
