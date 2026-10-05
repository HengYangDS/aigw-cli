//go:build linux

package native

import (
	"context"
	"crypto/rand"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
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

func TestLockedSecretServiceRefusesInteractiveOperations(t *testing.T) {
	if os.Getenv("AIGW_VERIFY_LOCKED_SECRET_SERVICE") != "1" {
		t.Skip("disposable locked Secret Service verification was not selected")
	}
	if os.Getenv("AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE") != "ephemeral-host" {
		t.Fatal("locked Secret Service verification requires an explicitly disposable private D-Bus session")
	}
	t.Setenv("HOME", t.TempDir())
	control, err := os.MkdirTemp(os.TempDir(), "gkr-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(control); err != nil {
			t.Error(err)
		}
	})
	t.Setenv("XDG_RUNTIME_DIR", control)
	t.Setenv("GNOME_KEYRING_CONTROL", control)
	unlockDisposableSecretService(t, false)
	credentialService, err := ss.NewSecretService()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := credentialService.Conn.Close(); err != nil {
			t.Error(err)
		}
	})
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	service, account := "AIGW_TOKEN", "native-locked-"+rand.Text()
	if _, err := queryCredential(writeCommand, service, account, []byte("synthetic-initial")); err != nil {
		t.Fatal(err)
	}
	if err := Write(executable, service, account, "synthetic-before-update"); err != nil {
		t.Fatal(err)
	}
	unlockDisposableSecretService(t, true)
	collection := credentialService.GetLoginCollection()
	attributes := map[string]string{"service": service, "username": account}
	before, err := credentialService.SearchItems(collection, attributes)
	if err != nil || len(before) != 1 {
		t.Fatalf("locked fixture requires one native item: %v", err)
	}
	item := credentialService.Object(secretServiceName, before[0])
	if err := item.SetProperty(secretItemInterface+".Label", "Existing synthetic Account Token"); err != nil {
		t.Fatal(err)
	}
	label, err := item.GetProperty(secretItemInterface + ".Label")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := queryCredential(writeCommand, service, account, []byte("synthetic-retained")); err != nil {
		t.Fatal(err)
	}
	itemAttributes, err := item.GetProperty(secretItemInterface + ".Attributes")
	if err != nil {
		t.Fatal(err)
	}
	lockSecretServiceObject(t, credentialService, before[0])
	itemLocked, err := item.GetProperty(secretItemInterface + ".Locked")
	if err != nil || itemLocked.Value() != true {
		t.Fatalf("native item did not become locked: %v", err)
	}
	lockSecretServiceObject(t, credentialService, collection.Path())
	monitor, messages := monitorSecretService(t)
	requireLockedSecretServiceRefusal(t, executable, service, account)
	var pendingCollection, prompt dbus.ObjectPath
	properties := map[string]dbus.Variant{secretCollectionInterface + ".Label": dbus.MakeVariant("disposable-pending-collection")}
	if err := credentialService.Object(secretServiceName, "/org/freedesktop/secrets").Call("org.freedesktop.Secret.Service.CreateCollection", 0, properties, "").Store(&pendingCollection, &prompt); err != nil || pendingCollection != "/" || prompt == "/" {
		t.Fatalf("native fixture did not return a pending prompt without a collection: %v", err)
	}
	if err := refuseSecretServicePrompt(credentialService.Object(secretServiceName, prompt)); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("pending native prompt was not refused: %v", err)
	}
	after, err := credentialService.SearchItems(collection, attributes)
	if err != nil || len(after) != 1 || after[0] != before[0] {
		t.Fatalf("locked operation changed owned native item identity: %v", err)
	}
	state, err := collection.GetProperty(secretCollectionInterface + ".Locked")
	if err != nil || state.Value() != true {
		t.Fatalf("native collection is not still locked: %v", err)
	}
	requireNoSecretServiceInteraction(t, messages, credentialService.Names()[0])
	if err := monitor.Close(); err != nil {
		t.Fatal(err)
	}
	unlockDisposableSecretService(t, true)
	requireRetainedSecretServiceItem(t, credentialService, executable, service, account, label, itemAttributes)
}

func requireLockedSecretServiceRefusal(t *testing.T, executable, service, account string) {
	t.Helper()
	if present, err := Exists(executable, service, account); err != nil || !present {
		t.Fatalf("locked native item metadata was not observable: %v", err)
	}
	if present, err := queryCredential(existsCommand, service, account, nil); err != nil || string(present) != "1" {
		t.Fatalf("locked native provider metadata was not observable: %v", err)
	}
	if absent, err := queryCredential(existsCommand, service, account+"-absent", nil); err != nil || string(absent) != "0" {
		t.Fatalf("absent native provider item was not observable: %v", err)
	}
	if _, err := Read(executable, service, account+"-absent"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("absent native item read must remain not found while locked: %v", err)
	}
	if _, err := queryCredential(readCommand, service, account+"-absent", nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("absent native provider read must remain not found while locked: %v", err)
	}
	if err := Delete(executable, service, account+"-absent"); err != nil {
		t.Fatalf("absent native item removal must remain a no-op while locked: %v", err)
	}
	for _, operation := range []struct {
		name    string
		command string
		run     func() error
	}{
		{"read", readCommand, func() error { _, err := Read(executable, service, account); return err }},
		{"write", writeCommand, func() error { return Write(executable, service, account, "synthetic-replacement") }},
		{"delete", deleteCommand, func() error { return Delete(executable, service, account) }},
	} {
		t.Run(operation.name, func(t *testing.T) {
			if err := operation.run(); !errors.Is(err, ErrUnavailable) {
				t.Errorf("locked operation must refuse without waiting for interaction: %v", err)
			}
			if _, err := queryCredential(operation.command, service, account, []byte("synthetic-replacement")); !errors.Is(err, ErrUnavailable) {
				t.Errorf("locked provider operation must refuse without interaction: %v", err)
			}
		})
	}
}

func requireRetainedSecretServiceItem(t *testing.T, credentialService *ss.SecretService, executable, service, account string, label, itemAttributes dbus.Variant) {
	t.Helper()
	restored, err := credentialService.SearchItems(credentialService.GetLoginCollection(), map[string]string{"service": service, "username": account})
	if err != nil || len(restored) != 1 {
		t.Fatalf("retained native fixture requires one restored item: %v", err)
	}
	item := credentialService.Object(secretServiceName, restored[0])
	if got, err := item.GetProperty(secretItemInterface + ".Label"); err != nil || !reflect.DeepEqual(got.Value(), label.Value()) {
		t.Fatalf("locked operations changed retained item label: %v", err)
	}
	if got, err := item.GetProperty(secretItemInterface + ".Attributes"); err != nil || !reflect.DeepEqual(got.Value(), itemAttributes.Value()) {
		t.Fatalf("locked operations changed retained item attributes: %v", err)
	}
	if value, err := Read(executable, service, account); err != nil || value != "synthetic-retained" {
		t.Fatalf("locked operations changed retained credential bytes: %v", err)
	}
	if value, err := queryCredential(readCommand, service, account, nil); err != nil || string(value) != "synthetic-retained" {
		t.Fatalf("native provider changed retained credential bytes: %v", err)
	}
	for range 2 {
		if _, err := queryCredential(deleteCommand, service, account, nil); err != nil {
			t.Fatalf("native provider removal was not idempotent: %v", err)
		}
	}
	if err := Delete(executable, service, account); err != nil {
		t.Fatal(err)
	}
	if remaining, err := credentialService.SearchItems(credentialService.GetLoginCollection(), map[string]string{"service": service, "username": account}); err != nil || len(remaining) != 0 {
		t.Fatalf("owned native item survived cleanup: %v", err)
	}
}

func lockSecretServiceObject(t *testing.T, credentialService *ss.SecretService, object dbus.ObjectPath) {
	t.Helper()
	var locked []dbus.ObjectPath
	var prompt dbus.ObjectPath
	if err := credentialService.Object(secretServiceName, "/org/freedesktop/secrets").Call("org.freedesktop.Secret.Service.Lock", 0, []dbus.ObjectPath{object}).Store(&locked, &prompt); err != nil || len(locked) != 1 || prompt != "/" {
		t.Fatalf("disposable native object lock failed: %v", err)
	}
}

func unlockDisposableSecretService(t *testing.T, replace bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	activation := exec.CommandContext(ctx, "dbus-update-activation-environment", "HOME", "XDG_RUNTIME_DIR", "GNOME_KEYRING_CONTROL")
	activation.Stdout, activation.Stderr = os.Stdout, os.Stderr
	if err := activation.Run(); err != nil {
		t.Fatalf("private Secret Service activation environment was not updated: %v", err)
	}
	args := []string{"--unlock", "--components=secrets", "--control-directory=" + os.Getenv("GNOME_KEYRING_CONTROL")}
	if replace {
		args = append(args, "--replace")
	}
	command := exec.CommandContext(ctx, "gnome-keyring-daemon", args...)
	command.Stdin = strings.NewReader("aigw-disposable-fixture\n")
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		t.Fatalf("controlled disposable Secret Service unlock failed: %v", err)
	}
}

func monitorSecretService(t *testing.T) (*dbus.Conn, <-chan *dbus.Message) {
	t.Helper()
	monitor, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := monitor.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := monitor.BusObject().Call("org.freedesktop.DBus.Monitoring.BecomeMonitor", 0, []string{"type='method_call',destination='org.freedesktop.secrets'"}, uint32(0)).Err; err != nil {
		t.Fatal(err)
	}
	messages := make(chan *dbus.Message, 256)
	monitor.Eavesdrop(messages)
	return monitor, messages
}

func requireNoSecretServiceInteraction(t *testing.T, messages <-chan *dbus.Message, witnessSender string) {
	t.Helper()
	dismissed := false
	witness := time.NewTimer(time.Second)
	defer witness.Stop()
	for {
		select {
		case message := <-messages:
			member := message.Headers[dbus.FieldMember].Value()
			if member == "Prompt" || member == "Unlock" || member == "SetSecret" || member == "Delete" || member == "GetSecret" {
				t.Errorf("locked credential operation invoked forbidden native method %v", member)
			}
			if member == "Dismiss" {
				dismissed = true
			}
			if dismissed && member == "Get" && message.Headers[dbus.FieldSender].Value() == witnessSender {
				t.Log("native monitor witnessed the final locked-state observation")
				return
			}
		case <-witness.C:
			t.Fatal("native method monitor did not observe its positive witness")
		}
	}
}
