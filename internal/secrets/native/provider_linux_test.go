//go:build linux

package native

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestObserveCredentialReportsSecretServiceConnectionFailure(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+filepath.Join(t.TempDir(), "missing-session-bus.sock"))

	_, err := observeCredential("aigw-test", "missing")
	if err == nil || !strings.Contains(err.Error(), "connect to Secret Service") {
		t.Fatalf("observeCredential() error = %v", err)
	}
}

func TestExistsFailsClosedWithoutSecretServiceSession(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+filepath.Join(t.TempDir(), "missing-session-bus.sock"))

	present, err := Exists(executable, "AIGW_TOKEN", "missing")
	if present || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("credential presence without Secret Service = %v, %v", present, err)
	}
}

func TestNativeEnvironmentRetainsOnlySecretServiceContext(t *testing.T) {
	values := map[string]string{
		"HOME":                     "/home/runner",
		"DBUS_SESSION_BUS_ADDRESS": "unix:path=/run/user/1000/bus",
		"XDG_RUNTIME_DIR":          "/run/user/1000",
		"PATH":                     "/untrusted",
	}
	want := "HOME=/home/runner\nDBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/1000/bus\nXDG_RUNTIME_DIR=/run/user/1000"
	if got := strings.Join(nativeEnvironment(func(name string) string { return values[name] }), "\n"); got != want {
		t.Fatalf("native environment = %q, want %q", got, want)
	}
}
