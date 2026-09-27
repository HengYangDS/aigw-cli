package credential

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestVersionedEntrypointRejectsChangedSourceBeforeCreatingBytes(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	first := sha256.Sum256([]byte("first-version"))
	target := filepath.Join(root, "data", "credential", hex.EncodeToString(first[:]), "aigw")
	if err := os.WriteFile(source, []byte("second-version"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureEntrypoint(source, target); err == nil {
		t.Fatal("changed executable was installed under the predecessor digest")
	}
	if _, err := os.Lstat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rejected successor left executable bytes: %v", err)
	}
}

func TestVersionedEntrypointRejectsBytesThatMatchOnlyTheirReceipt(t *testing.T) {
	root := t.TempDir()
	first := sha256.Sum256([]byte("first-version"))
	second := sha256.Sum256([]byte("second-version"))
	target := filepath.Join(root, "data", "credential", hex.EncodeToString(first[:]), "aigw")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("second-version"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target+".sha256", []byte(hex.EncodeToString(second[:])+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := EntrypointNeeded(target); err == nil {
		t.Fatal("digest directory accepted different executable bytes")
	}
}

func TestVersionedEntrypointsKeepPredecessorBytesThroughSuccessorRemoval(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	data := filepath.Join(root, "data")
	var predecessor string
	for _, version := range []string{"first-version", "second-version"} {
		if err := os.WriteFile(source, []byte(version), 0o700); err != nil {
			t.Fatal(err)
		}
		path, err := VersionedEntrypointPath(data, source, "aigw")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := EnsureEntrypoint(source, path); err != nil {
			t.Fatal(err)
		}
		if got, err := os.ReadFile(path); err != nil || string(got) != version {
			t.Fatalf("installed version = %q, %v; want %q", got, err, version)
		}
		if version == "first-version" {
			predecessor = path
		} else if err := RemoveEntrypoint(path); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := os.ReadFile(predecessor); err != nil || string(got) != "first-version" {
		t.Fatalf("predecessor changed after successor removal: %q, %v", got, err)
	}
}
