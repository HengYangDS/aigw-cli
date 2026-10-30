package program

import (
	"aigw-cli/internal/platform/process"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// custodyRecorder records credential custody transitions after the predecessor
// commands that precede them.
type custodyRecorder struct {
	events     *[]string
	releaseErr error
	onRelease  func()
}

func (recorder custodyRecorder) Release(version string) error {
	*recorder.events = append(*recorder.events, "release "+version)
	if recorder.onRelease != nil {
		recorder.onRelease()
	}
	return recorder.releaseErr
}

func (recorder custodyRecorder) Reclaim() error {
	*recorder.events = append(*recorder.events, "reclaim")
	return nil
}

func TestRollbackMovesCredentialCustodyOnlyAfterAdmission(t *testing.T) {
	admitted := []string{"verify --version", "verify config export"}
	for _, test := range []struct {
		name       string
		backup     bool
		export     string
		releaseErr error
		cancel     bool
		activated  bool
		want       []string
	}{
		{name: "missing predecessor"},
		{name: "incompatible configuration", backup: true, export: "not a configuration", want: admitted},
		{name: "activated predecessor", backup: true, activated: true, want: append(slices.Clone(admitted), "release 0.3.5")},
		{name: "failed release", backup: true, releaseErr: errors.New("keychain locked"), want: append(slices.Clone(admitted), "release 0.3.5", "reclaim")},
		{name: "failed activation", backup: true, cancel: true, want: append(slices.Clone(admitted), "release 0.3.5", "reclaim")},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			current := filepath.Join(root, "aigw")
			programs := map[string]string{current: "current"}
			if test.backup {
				programs[RollbackPath(current)] = "previous"
			}
			for path, value := range programs {
				if err := os.WriteFile(path, []byte(value), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			export := test.export
			if export == "" {
				export = "version = 5\n"
			}
			var events []string
			runner := &recordingRunner{inspect: func(plan process.Plan) ([]byte, error) {
				events = append(events, "verify "+strings.Join(plan.Args, " "))
				if slices.Equal(plan.Args, []string{"--version"}) {
					return []byte("aigw version 0.3.5\n"), nil
				}
				return []byte(export), nil
			}}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			custody := custodyRecorder{events: &events, releaseErr: test.releaseErr}
			if test.cancel {
				custody.onRelease = cancel
			}
			_, err := (Updater{Executable: current, Runner: runner}).Rollback(ctx, []byte("version = 3\n"), custody)
			if test.activated != (err == nil) {
				t.Fatalf("rollback result = %v", err)
			}
			if test.releaseErr != nil && !errors.Is(err, test.releaseErr) {
				t.Fatalf("release failure was not reported: %v", err)
			}
			if !slices.Equal(events, test.want) {
				t.Fatalf("custody events = %q, want %q", events, test.want)
			}
			want := programs
			if test.activated {
				want = map[string]string{current: "previous", RollbackPath(current): "current"}
			}
			for path, value := range want {
				if data, err := os.ReadFile(path); err != nil || string(data) != value {
					t.Fatalf("program %s = %q, %v; want %q", path, data, err, value)
				}
			}
		})
	}
}

func TestRollbackWithoutCredentialCustodyActivatesPredecessor(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "aigw")
	for path, value := range map[string]string{current: "current", RollbackPath(current): "previous"} {
		if err := os.WriteFile(path, []byte(value), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	runner := &recordingRunner{output: []byte("aigw version 0.3.5\n")}
	if _, err := (Updater{Executable: current, Runner: runner}).Rollback(t.Context(), nil, nil); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(current); err != nil || string(data) != "previous" {
		t.Fatalf("program = %q, %v", data, err)
	}
}
