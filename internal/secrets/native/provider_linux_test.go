//go:build linux

package native

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProviderExistsRejectsSecretServiceConnectionFailure(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+filepath.Join(t.TempDir(), "missing-session-bus.sock"))

	output, err := queryCredential(existsCommand, "aigw-test", "missing", nil)
	if len(output) != 0 || err == nil || !strings.Contains(err.Error(), "connect to Secret Service") {
		t.Fatalf("credential presence output length = %d, error = %v", len(output), err)
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

func TestExistsAcceptsOnlyExactWorkerPresenceBytes(t *testing.T) {
	for _, test := range []struct {
		name    string
		output  string
		present bool
		invalid bool
	}{
		{name: "present", output: "1", present: true},
		{name: "absent", output: "0"},
		{name: "invalid", output: "2", invalid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			worker := filepath.Join(t.TempDir(), "credential-worker")
			if err := os.WriteFile(worker, []byte("#!/bin/sh\nprintf '"+test.output+"'\n"), 0o700); err != nil {
				t.Fatal(err)
			}
			present, err := Exists(worker, "AIGW_TOKEN", "team")
			if test.invalid {
				if present || !errors.Is(err, ErrUnavailable) {
					t.Fatalf("invalid worker presence = %t, error = %v", present, err)
				}
				return
			}
			if err != nil || present != test.present {
				t.Fatalf("worker presence = %t, error = %v", present, err)
			}
		})
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
