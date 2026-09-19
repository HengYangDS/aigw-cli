package upgrade

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHomebrewOwnershipPrecedesPortableMutation(t *testing.T) {
	for _, layout := range []string{"Cellar", "Caskroom"} {
		t.Run(layout, func(t *testing.T) {
			root := t.TempDir()
			packageRoot := filepath.Join(root, layout, "aigw")
			versionRoot := filepath.Join(packageRoot, "0.1.0")
			program := filepath.Join(versionRoot, "bin", "aigw")
			receipt := filepath.Join(versionRoot, "INSTALL_RECEIPT.json")
			if layout == "Caskroom" {
				receipt = filepath.Join(packageRoot, ".metadata", "INSTALL_RECEIPT.json")
			}
			for name, data := range map[string]string{program: "retained", receipt: `{"homebrew_version":"7.0.2","source":{"tap":"owner/tap"}}`} {
				if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(name, []byte(data), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			runner := &recordingRunner{}
			updater := Updater{Executable: program, Runner: runner}
			operations := map[string]func() error{
				"update":      func() error { _, err := updater.Update(t.Context(), "0.1.0"); return err },
				"candidate":   func() error { _, err := updater.UpdateCandidate(t.Context(), "0.1.0", CandidateArchive{}); return err },
				"rollback":    func() error { _, err := updater.Rollback(t.Context(), nil); return err },
				"replacement": func() error { return updater.replacePortableBinary(t.Context(), []byte("replacement")) },
			}
			for name, operation := range operations {
				t.Run(name, func(t *testing.T) {
					err := operation()
					if err == nil || !strings.Contains(err.Error(), "Homebrew") {
						t.Fatalf("ownership error = %v", err)
					}
					if len(runner.plans) != 0 {
						t.Fatal("managed program started portable execution")
					}
					data, err := os.ReadFile(program)
					if err != nil || string(data) != "retained" {
						t.Fatalf("program changed: %q, %v", data, err)
					}
				})
			}
		})
	}
}

func TestPortableOwnershipUsesReceiptEvidence(t *testing.T) {
	for _, layout := range []string{"Cellar", "Caskroom"} {
		for _, content := range []string{"", "invalid", `{}`, `{"homebrew_version":"7.0.2","source":{"tap":"owner/tap"}}`} {
			t.Run(layout+"/"+content, func(t *testing.T) {
				root := t.TempDir()
				packageRoot := filepath.Join(root, layout, "renamed-package")
				versionRoot := filepath.Join(packageRoot, "1.2.3")
				target := filepath.Join(versionRoot, "new", "bin", "renamed-program")
				receipt := filepath.Join(versionRoot, "INSTALL_RECEIPT.json")
				if layout == "Caskroom" {
					receipt = filepath.Join(packageRoot, ".metadata", "INSTALL_RECEIPT.json")
				}
				if err := os.MkdirAll(filepath.Dir(receipt), 0o700); err != nil {
					t.Fatal(err)
				}
				if content != "" {
					if err := os.WriteFile(receipt, []byte(content), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				err := RequirePortableOwnership(target)
				if (err != nil) != (content != "") {
					t.Fatalf("ownership = %v", err)
				}
				if _, err := os.Stat(filepath.Dir(target)); !os.IsNotExist(err) {
					t.Fatalf("inspection created directories: %v", err)
				}
			})
		}
	}
	if err := RequirePortableOwnership(""); err != nil {
		t.Fatalf("empty path: %v", err)
	}
	if err := RequirePortableOwnership("invalid\x00path"); err == nil {
		t.Fatal("invalid path accepted")
	}
}
