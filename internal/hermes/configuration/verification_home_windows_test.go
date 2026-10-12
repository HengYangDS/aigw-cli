package configuration

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestVerificationHomeRefusesWindowsLauncherEffects(t *testing.T) {
	home, cleanup, err := CreateVerificationHome(filepath.Join(t.TempDir(), "config.yaml"))
	if home != "" || cleanup != nil || err == nil || !strings.Contains(err.Error(), "no client process was started") {
		t.Fatalf("unisolated Windows verification was permitted: %q %v", home, err)
	}
}
