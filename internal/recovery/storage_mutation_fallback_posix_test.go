//go:build !darwin && !linux && !windows

package recovery

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateRecoveryMutationParentsEnforcesPOSIXPermissions(t *testing.T) {
	root := privateRecoveryRootForTest(t)
	parent := filepath.Join(root, "air")
	if err := os.Mkdir(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := validateRecoveryMutationParents(root, filepath.Join(parent, "ledger.json"), false); err == nil {
		t.Fatal("accepted a recovery parent with unsafe permissions")
	}
}
