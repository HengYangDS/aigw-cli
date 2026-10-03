package desktop

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRollbackRejectsAChangedPostimage(t *testing.T) {
	root := t.TempDir()
	paths := PathsForLibrary(filepath.Join(root, "Claude-3p", "configLibrary"))
	desired := Desired{
		BaseURL:              "https://gateway.example.test/v1",
		CredentialExecutable: filepath.Join(root, "aigw"),
		CredentialArguments:  []string{"credential", "claude-desktop", "fingerprint"},
		Models:               []Model{{Name: "claude-fable-5-1"}},
	}
	plan, err := Prepare(paths, &desired)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := plan.Apply()
	if err != nil {
		t.Fatal(err)
	}
	foreign := []byte("{\n  \"deploymentMode\": \"3p\",\n  \"foreign\": true\n}\n")
	if err := os.WriteFile(paths.StandardConfig, foreign, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := receipt.Rollback(); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("Rollback() error = %v", err)
	}
	if got, err := os.ReadFile(paths.StandardConfig); err != nil || !bytes.Equal(got, foreign) {
		t.Fatalf("foreign postimage = %q, %v", got, err)
	}
}

func TestProjectionCompensatesFailureAtEveryOwnedWrite(t *testing.T) {
	root := t.TempDir()
	paths := PathsForLibrary(filepath.Join(root, "Claude-3p", "configLibrary"))
	desired := Desired{
		BaseURL:              "https://gateway.example.test/v1",
		CredentialExecutable: filepath.Join(root, "aigw"),
		Models:               []Model{{Name: "claude-fable-5-1"}},
	}
	plan, err := Prepare(paths, &desired)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.changes) < 3 {
		t.Fatalf("expected a multi-file projection, got %d writes", len(plan.changes))
	}
	for index, failed := range plan.changes {
		t.Run(filepath.Base(failed.path), func(t *testing.T) {
			if failed.before.Exists {
				t.Fatal("fixture unexpectedly owns a pre-existing file")
			}
			if err := os.MkdirAll(filepath.Dir(failed.path), 0o700); err != nil {
				t.Fatal(err)
			}
			foreign := []byte("{\"foreign\":true}\n")
			if err := os.WriteFile(failed.path, foreign, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := plan.Apply(); err == nil || !strings.Contains(err.Error(), "preimage changed for "+failed.path) {
				t.Fatalf("write %d did not expose its exact conflict: %v", index, err)
			}
			for position, change := range plan.changes {
				current, readErr := os.ReadFile(change.path)
				if position == index {
					if readErr != nil || !bytes.Equal(current, foreign) {
						t.Fatalf("foreign target changed: %q, %v", current, readErr)
					}
				} else if !os.IsNotExist(readErr) {
					t.Fatalf("owned file %s survived compensation: %q, %v", change.path, current, readErr)
				}
			}
			if err := os.Remove(failed.path); err != nil {
				t.Fatal(err)
			}
		})
	}
}
