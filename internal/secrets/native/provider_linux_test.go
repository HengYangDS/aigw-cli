//go:build linux

package native

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeSecretServiceCredentialLifecycle(t *testing.T) {
	if os.Getenv("AIGW_VERIFY_SYSTEM_KEYRING") != "1" {
		t.Skip("native Secret Service qualification was not selected")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	const service, token = "AIGW_TOKEN", "synthetic-native-presence-token"
	account := filepath.Base(filepath.Dir(t.TempDir()))
	requirePresence := func(want bool) {
		t.Helper()
		value, err := queryCredential(existsCommand, service, account, nil)
		wantValue := "0"
		if want {
			wantValue = "1"
		}
		if err != nil || string(value) != wantValue {
			t.Fatalf("native credential metadata presence = %q, %v; want %t", value, err, want)
		}
		if present, err := Exists(executable, service, account); err != nil || present != want {
			t.Fatalf("bounded native credential presence = %t, %v; want %t", present, err, want)
		}
	}
	requirePresence(false)
	t.Cleanup(func() {
		if err := Delete(executable, service, account); err != nil {
			t.Errorf("remove owned native credential: %v", err)
		}
		requirePresence(false)
	})
	if err := Write(executable, service, account, token); err != nil {
		t.Fatal(err)
	}
	requirePresence(true)
	if present, err := observeCredential(service+"-unselected", account); err != nil || present {
		t.Fatalf("unselected service metadata presence = %t, %v", present, err)
	}
	if value, err := Read(executable, service, account); err != nil || value != token {
		t.Fatal("native credential read did not preserve the owned synthetic value")
	}
	if err := Delete(executable, service, account); err != nil {
		t.Fatal(err)
	}
	requirePresence(false)
}

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
