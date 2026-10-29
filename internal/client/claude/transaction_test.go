package claude

import (
	"aigw-cli/internal/domain"
	"aigw-cli/internal/store"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSettingsTransactionFailuresRollbackOrReportExactCause(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	runtime := domain.Runtime{RouteID: "team", AccountID: "gateway", Endpoint: "https://gateway.test"}

	t.Run("settings snapshot", func(t *testing.T) {
		withSettingsTransaction(t,
			func(string) (store.FileSnapshot, error) { return store.FileSnapshot{}, os.ErrPermission },
			writeGuarded, removeGuarded, restoreGuarded,
		)
		if _, err := ReconcileSettings(path, false, runtime, testExecutable(), runtime.Model); err == nil || !strings.Contains(err.Error(), "read Claude settings") {
			t.Fatalf("error=%v", err)
		}
	})

	t.Run("state snapshot", func(t *testing.T) {
		calls := 0
		withSettingsTransaction(t,
			func(path string) (store.FileSnapshot, error) {
				calls++
				if calls == 2 {
					return store.FileSnapshot{}, os.ErrPermission
				}
				return store.CaptureFileSnapshot(path)
			},
			writeGuarded, removeGuarded, restoreGuarded,
		)
		if _, err := ReconcileSettings(path, false, runtime, testExecutable(), runtime.Model); err == nil || !strings.Contains(err.Error(), "read Claude settings state") {
			t.Fatalf("error=%v", err)
		}
	})

	t.Run("settings write", func(t *testing.T) {
		withSettingsTransaction(t, captureSnapshot,
			func(string, store.FileSnapshot, []byte, os.FileMode) (store.FileSnapshot, error) {
				return store.FileSnapshot{}, os.ErrPermission
			},
			removeGuarded, restoreGuarded,
		)
		if _, err := ReconcileSettings(path, false, runtime, testExecutable(), runtime.Model); err == nil || !strings.Contains(err.Error(), "write Claude settings") {
			t.Fatalf("error=%v", err)
		}
	})

	t.Run("state write rollback", func(t *testing.T) {
		calls := 0
		rolledBack := false
		withSettingsTransaction(t, captureSnapshot,
			func(path string, before store.FileSnapshot, data []byte, mode os.FileMode) (store.FileSnapshot, error) {
				calls++
				if calls == 2 {
					return store.FileSnapshot{}, os.ErrPermission
				}
				return store.WriteFileAtomicIfUnchanged(path, before, data, mode)
			},
			removeGuarded,
			func(path string, before, after store.FileSnapshot) error {
				rolledBack = true
				return store.RestoreFileAtomicIfPostimage(path, before, after)
			},
		)
		if _, err := ReconcileSettings(path, false, runtime, testExecutable(), runtime.Model); err == nil || !strings.Contains(err.Error(), "write Claude settings state") || !rolledBack {
			t.Fatalf("error=%v rolledBack=%t", err, rolledBack)
		}
	})

	t.Run("state write rollback failure", func(t *testing.T) {
		calls := 0
		withSettingsTransaction(t, captureSnapshot,
			func(path string, before store.FileSnapshot, data []byte, mode os.FileMode) (store.FileSnapshot, error) {
				calls++
				if calls == 2 {
					return store.FileSnapshot{}, os.ErrPermission
				}
				return store.WriteFileAtomicIfUnchanged(path, before, data, mode)
			},
			removeGuarded,
			func(string, store.FileSnapshot, store.FileSnapshot) error {
				return errors.New("rollback failed")
			},
		)
		if _, err := ReconcileSettings(path, false, runtime, testExecutable(), runtime.Model); err == nil || !strings.Contains(err.Error(), "rollback failed") {
			t.Fatalf("error=%v", err)
		}
	})
}

func TestSettingsDisableFailuresPreserveManagedProjection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"theme":"dark"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime := domain.Runtime{RouteID: "team", AccountID: "gateway", Endpoint: "https://gateway.test"}
	if _, err := ReconcileSettings(path, false, runtime, testExecutable(), runtime.Model); err != nil {
		t.Fatal(err)
	}

	t.Run("managed drift", func(t *testing.T) {
		data, _ := os.ReadFile(path)
		if err := os.WriteFile(path, bytes.Replace(data, []byte("gateway.test"), []byte("foreign.test"), 1), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReconcileSettings(path, true, domain.Runtime{}, "", ""); err == nil || !strings.Contains(err.Error(), "refusing to remove") {
			t.Fatalf("error=%v", err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("restore write", func(t *testing.T) {
		withSettingsTransaction(t, captureSnapshot,
			func(string, store.FileSnapshot, []byte, os.FileMode) (store.FileSnapshot, error) {
				return store.FileSnapshot{}, os.ErrPermission
			},
			removeGuarded, restoreGuarded,
		)
		if _, err := ReconcileSettings(path, true, domain.Runtime{}, "", ""); err == nil || !strings.Contains(err.Error(), "restore Claude settings") {
			t.Fatalf("error=%v", err)
		}
	})

	t.Run("state removal and rollback", func(t *testing.T) {
		withSettingsTransaction(t, captureSnapshot, writeGuarded,
			func(string, store.FileSnapshot) (store.FileSnapshot, error) {
				return store.FileSnapshot{}, os.ErrPermission
			},
			restoreGuarded,
		)
		if _, err := ReconcileSettings(path, true, domain.Runtime{}, "", ""); err == nil || !strings.Contains(err.Error(), "remove Claude settings state") {
			t.Fatalf("error=%v", err)
		}
	})

	t.Run("state removal rollback failure", func(t *testing.T) {
		withSettingsTransaction(t, captureSnapshot, writeGuarded,
			func(string, store.FileSnapshot) (store.FileSnapshot, error) {
				return store.FileSnapshot{}, os.ErrPermission
			},
			func(string, store.FileSnapshot, store.FileSnapshot) error {
				return errors.New("rollback failed")
			},
		)
		if _, err := ReconcileSettings(path, true, domain.Runtime{}, "", ""); err == nil || !strings.Contains(err.Error(), "rollback failed") {
			t.Fatalf("error=%v", err)
		}
	})
}

func TestSettingsDisableAbsentFileReportsRemovalFailures(t *testing.T) {
	for _, failAt := range []int{1, 2} {
		t.Run(fmt.Sprintf("remove-%d", failAt), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			runtime := domain.Runtime{RouteID: "team", AccountID: "gateway", Endpoint: "https://gateway.test"}
			if _, err := ReconcileSettings(path, false, runtime, testExecutable(), runtime.Model); err != nil {
				t.Fatal(err)
			}
			calls := 0
			withSettingsTransaction(t, captureSnapshot, writeGuarded,
				func(path string, before store.FileSnapshot) (store.FileSnapshot, error) {
					calls++
					if calls == failAt {
						return store.FileSnapshot{}, os.ErrPermission
					}
					return store.RemoveFileIfUnchanged(path, before)
				},
				restoreGuarded,
			)
			if _, err := ReconcileSettings(path, true, domain.Runtime{}, "", ""); err == nil {
				t.Fatal("removal failure was accepted")
			}
		})
	}
}

func withSettingsTransaction(
	t *testing.T,
	capture func(string) (store.FileSnapshot, error),
	write func(string, store.FileSnapshot, []byte, os.FileMode) (store.FileSnapshot, error),
	remove func(string, store.FileSnapshot) (store.FileSnapshot, error),
	restore func(string, store.FileSnapshot, store.FileSnapshot) error,
) {
	t.Helper()
	oldCapture, oldWrite, oldRemove, oldRestore := captureSnapshot, writeGuarded, removeGuarded, restoreGuarded
	captureSnapshot, writeGuarded, removeGuarded, restoreGuarded = capture, write, remove, restore
	t.Cleanup(func() {
		captureSnapshot, writeGuarded, removeGuarded, restoreGuarded = oldCapture, oldWrite, oldRemove, oldRestore
	})
}
