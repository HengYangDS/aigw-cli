package secrets

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/secrets/native"

	keyring "github.com/zalando/go-keyring"
)

func TestDecodeKeyringBase64RequiresCanonicalSingleEnvelope(t *testing.T) {
	const token = "public-fixture-token"
	value := "go-keyring-base64:" + base64.StdEncoding.EncodeToString([]byte(token))
	if decoded, err := DecodeKeyringBase64(value); err != nil || decoded != token {
		t.Fatalf("decoded envelope = %q, %v", decoded, err)
	}
	for _, value := range []string{"", token, "go-keyring-encoded:746f6b656e", "go-keyring-base64:", "go-keyring-base64:Zh==", "go-keyring-base64:dG9r\nZW4=", "go-keyring-base64:" + base64.StdEncoding.EncodeToString([]byte("go-keyring-base64:dG9rZW4="))} {
		if decoded, err := DecodeKeyringBase64(value); err == nil || decoded != "" {
			t.Fatal("noncanonical or nested Keyring envelope was accepted")
		}
	}
}

func TestKeyringCredentialKindsShareOneServiceWithoutSharingSlots(t *testing.T) {
	keyring.MockInit()
	store := scopedView{store: mockKeyringStore()}
	if err := store.Set("dmx", "api-token"); err != nil {
		t.Fatal(err)
	}
	diagnostics, err := ForKind(store, ProviderDiagnostic)
	if err != nil {
		t.Fatal(err)
	}
	if err := diagnostics.Set("dmx", diagnosticValue); err != nil {
		t.Fatal(err)
	}
	if got, err := keyring.Get(Service, "dmx"); err != nil || got != "api-token" {
		t.Fatalf("API token slot = %q, %v", got, err)
	}
	if got, err := keyring.Get(Service, "diagnostic@dmx"); err != nil || got != diagnosticValue {
		t.Fatalf("diagnostic slot = %q, %v", got, err)
	}
}

func TestKeyringStoreLifecycleAndValidation(t *testing.T) {
	keyring.MockInit()
	store := scopedView{store: mockKeyringStore()}
	if mustExist(t, store, "dmx") {
		t.Fatal("new mocked keyring unexpectedly has a token")
	}
	if err := store.Set("dmx", "api-token"); err != nil || !mustExist(t, store, "dmx") {
		t.Fatalf("set token = %v, exists=%v", err, mustExist(t, store, "dmx"))
	}
	if got, err := store.Get("dmx"); err != nil || got != "api-token" {
		t.Fatalf("token = %q, %v", got, err)
	}
	if err := store.Delete("dmx"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("dmx"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing token error = %v", err)
	}
	if err := store.Delete("dmx"); err != nil {
		t.Fatalf("delete absent token = %v", err)
	}
	for _, name := range []string{"bad name", "", "../escape"} {
		if _, err := store.Get(name); err == nil {
			t.Errorf("Get(%q) succeeded", name)
		}
		if err := store.Set(name, "value"); err == nil {
			t.Errorf("Set(%q) succeeded", name)
		}
		if err := store.Delete(name); err == nil {
			t.Errorf("Delete(%q) succeeded", name)
		}
	}
	if err := store.Set("dmx", ""); err == nil {
		t.Fatal("empty token accepted")
	}
}

func TestKeyringStoreMapsEmptyValuesAndProviderErrors(t *testing.T) {
	keyring.MockInit()
	if err := keyring.Set(Service, "dmx", ""); err != nil {
		t.Fatal(err)
	}
	store := scopedView{store: mockKeyringStore()}
	if present, err := store.Exists("dmx"); err != nil || !present {
		t.Fatalf("empty provider value slot presence = %t, %v", present, err)
	}
	if _, err := store.Get("dmx"); err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("present empty provider value was reported absent: %v", err)
	}
	want := errors.New("keyring unavailable")
	keyring.MockInitWithError(want)
	t.Cleanup(keyring.MockInit)
	store = scopedView{store: mockKeyringStore()}
	if _, err := store.Get("dmx"); !errors.Is(err, want) {
		t.Fatalf("Get error = %v", err)
	}
	if err := store.Set("dmx", "value"); !errors.Is(err, want) {
		t.Fatalf("Set error = %v", err)
	}
	if err := store.Delete("dmx"); !errors.Is(err, want) {
		t.Fatalf("Delete error = %v", err)
	}
}

func TestNativeKeyringStoreRequiresProductExecutable(t *testing.T) {
	store := newKeyringStore("")
	if present, err := store.exists(APIToken, "team"); present || !errors.Is(err, native.ErrUnavailable) {
		t.Fatalf("credential presence without executable = %t, %v", present, err)
	}
	if value, err := store.get(APIToken, "team"); value != "" || !errors.Is(err, native.ErrUnavailable) {
		t.Fatalf("credential read without executable = %q, %v", value, err)
	}
	if err := store.set(APIToken, "team", "synthetic-token"); !errors.Is(err, native.ErrUnavailable) {
		t.Fatalf("credential write without executable = %v", err)
	}
	if err := store.delete(APIToken, "team"); !errors.Is(err, native.ErrUnavailable) {
		t.Fatalf("credential delete without executable = %v", err)
	}
}

func mockKeyringStore() keyringStore {
	return keyringStore{
		observe: mockKeyringObserver,
		read:    keyring.Get,
		write:   keyring.Set,
		remove:  keyring.Delete,
	}
}

func mockKeyringObserver(service, slot string) (bool, error) {
	_, err := keyring.Get(service, slot)
	if errors.Is(err, keyring.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

func TestVerifyNativeReaderAccessUsesOnlyTheCopiedReader(t *testing.T) {
	store := mockKeyringStore()
	store.observe = func(_, _ string) (bool, error) {
		t.Fatal("copied-reader authorization launched a redundant source metadata query")
		return false, nil
	}
	path := filepath.Join(t.TempDir(), "unavailable-copied-reader")
	if err := VerifyNativeReaderAccess(scopedView{store: store}, path, nil); err != nil {
		t.Fatalf("unused reader required authorization: %v", err)
	}
	if err := VerifyNativeReaderAccess(scopedView{store: store}, path, []string{"selected-account"}); !errors.Is(err, ErrNativeReaderUnverified) {
		t.Fatalf("unavailable copied reader authorization error = %v", err)
	}
	for _, account := range []string{"", "invalid/account"} {
		if err := VerifyNativeReaderAccess(scopedView{store: store}, path, []string{account}); !errors.Is(err, ErrNativeReaderUnverified) || !strings.Contains(err.Error(), "invalid Account identifier") {
			t.Fatalf("invalid Account reached the copied reader: %v", err)
		}
	}
}

func TestVerifyNativeReaderAccessRechecksAutomaticBackendIdentity(t *testing.T) {
	for _, test := range []struct {
		backend string
		problem string
	}{
		{backend: "file"},
		{},
		{backend: "keyring", problem: "selection changed"},
		{backend: "invalid", problem: "invalid persisted"},
	} {
		t.Run("current="+test.backend, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "secrets")
			choice := newBackendChoice(root)
			if _, _, err := choice.Persist("file"); err != nil {
				t.Fatal(err)
			}
			store, err := Select(Selection{GOOS: "linux", Root: root, KeyringProbe: func(Store) error {
				t.Fatal("identity validation probed or changed the selected backend")
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Inspect(store); err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(root, backendChoiceName)
			if test.backend == "" {
				err = os.Remove(marker)
			} else {
				err = replaceBackendChoice(choice, test.backend)
			}
			if err != nil {
				t.Fatal(err)
			}
			err = VerifyNativeReaderAccess(store, filepath.Join(root, "unavailable-copied-reader"), []string{"selected-account"})
			if test.problem == "" && err != nil || test.problem != "" && (err == nil || !strings.Contains(err.Error(), test.problem)) {
				t.Fatalf("automatic reader identity error = %v, want %q", err, test.problem)
			}
			if _, err := os.Stat(filepath.Join(root, "tokens")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("identity validation created another credential store: %v", err)
			}
		})
	}
}
