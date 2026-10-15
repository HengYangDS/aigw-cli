package credential

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProgramInstallsTheBytesItVerified(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "aigw")
	if err := os.WriteFile(source, []byte("verified program"), 0o700); err != nil {
		t.Fatal(err)
	}
	program, err := ReadProgram(nil, source)
	if err != nil {
		t.Fatal(err)
	}
	target, err := program.EntrypointPath(filepath.Join(root, "data"), "aigw")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("replaced program"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := program.EnsureEntrypoint(target); err != nil {
		t.Fatalf("install after a same-size program rewrite: %v", err)
	}
	if got, err := os.ReadFile(target); err != nil || string(got) != "verified program" {
		t.Fatalf("installed reader = %q, %v; want the verified program bytes", got, err)
	}
}

func TestEntrypointConfirmationRefusesAReaderChangedAfterVerification(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(reader string) error
	}{
		{"removed", func(reader string) error {
			return errors.Join(os.Remove(reader), os.Remove(reader+".sha256"))
		}},
		{"replaced", func(reader string) error {
			replacement := reader + ".replacement"
			if err := os.WriteFile(replacement, []byte("changed program!"), 0o700); err != nil {
				return err
			}
			return os.Rename(replacement, reader)
		}},
		{"receipt", func(reader string) error {
			return os.WriteFile(reader+".sha256", fmt.Appendf(nil, "%x\n", sha256.Sum256([]byte("another program"))), 0o600)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "aigw")
			if err := os.WriteFile(source, []byte("verified program"), 0o700); err != nil {
				t.Fatal(err)
			}
			program, err := ReadProgram(nil, source)
			if err != nil {
				t.Fatal(err)
			}
			target, err := program.EntrypointPath(filepath.Join(root, "data"), "aigw")
			if err != nil {
				t.Fatal(err)
			}
			entrypoint, _, err := program.EnsureEntrypoint(target)
			if err != nil {
				t.Fatal(err)
			}
			if err := entrypoint.Confirm(); err != nil {
				t.Fatalf("unchanged reader confirmation = %v", err)
			}
			if err := tc.change(target); err != nil {
				t.Fatal(err)
			}
			err = entrypoint.Confirm()
			if err == nil || strings.Contains(err.Error(), "disappeared") != (tc.name == "removed") {
				t.Fatalf("%s reader confirmation = %v", tc.name, err)
			}
		})
	}
}
