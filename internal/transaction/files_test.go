package transaction_test

import (
	"os"
	"path/filepath"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/transaction"
)

func TestWriteFileAtomicPreservesMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := transaction.WriteFileAtomic(path, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	info, _ := os.Stat(path)
	if string(data) != "new" || info.Mode().Perm() != 0o640 {
		t.Fatalf("content=%q mode=%o", data, info.Mode().Perm())
	}
}
