//go:build linux

package native

import (
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ss "github.com/zalando/go-keyring/secret_service"
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

func TestSecretServiceRotationPreservesExistingItem(t *testing.T) {
	if os.Getenv("AIGW_VERIFY_SYSTEM_KEYRING") != "1" {
		t.Skip("native Secret Service verification was not selected")
	}
	credentialService, err := ss.NewSecretService()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := credentialService.Conn.Close(); err != nil {
			t.Errorf("close native credential connection: %v", err)
		}
	})
	collection := credentialService.GetLoginCollection()
	service, account := "AIGW_TOKEN", "native-rotation-"+rand.Text()
	attributes := map[string]string{"service": service, "username": account}
	t.Cleanup(func() {
		items, err := credentialService.SearchItems(collection, attributes)
		if err != nil {
			t.Errorf("observe exact owned native items: %v", err)
			return
		}
		for _, item := range items {
			if err := credentialService.Delete(item); err != nil {
				t.Errorf("delete exact owned native item: %v", err)
			}
		}
		if remaining, err := credentialService.SearchItems(collection, attributes); err != nil || len(remaining) != 0 {
			t.Errorf("native item cleanup is incomplete: %v", err)
		}
	})
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := Write(executable, service, account, "synthetic-initial"); err != nil {
		t.Fatal(err)
	}
	before, err := credentialService.SearchItems(collection, attributes)
	if err != nil || len(before) != 1 {
		t.Fatalf("fixture did not create one exact native item: %v", err)
	}
	const label = "Existing synthetic Account Token"
	original := credentialService.Object("org.freedesktop.secrets", before[0])
	if err := original.SetProperty("org.freedesktop.Secret.Item.Label", label); err != nil {
		t.Fatal(err)
	}
	if err := Write(executable, service, account, "synthetic-replacement"); err != nil {
		t.Fatal(err)
	}
	after, err := credentialService.SearchItems(collection, attributes)
	if err != nil || len(after) != 1 || after[0] != before[0] {
		t.Fatalf("rotation changed native item identity: %v", err)
	}
	item := credentialService.Object("org.freedesktop.secrets", after[0])
	gotLabel, err := item.GetProperty("org.freedesktop.Secret.Item.Label")
	if err != nil || gotLabel.Value() != label {
		t.Fatalf("rotation rewrote existing native item metadata: %v", err)
	}
	if value, err := Read(executable, service, account); err != nil || value != "synthetic-replacement" {
		t.Fatalf("native reader did not return the rotated value: %v", err)
	}
	for range 2 {
		if err := Delete(executable, service, account); err != nil {
			t.Fatalf("native removal was not idempotent: %v", err)
		}
	}
}
