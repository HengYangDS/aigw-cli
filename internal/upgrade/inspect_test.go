package upgrade

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestInstallationObservationFollowsReplacementAndRollback(t *testing.T) {
	root := t.TempDir()
	program := filepath.Join(root, "renamed-program")
	previous := RollbackPath(program)
	for path, content := range map[string]string{program: "current", previous: "previous"} {
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	updater := Updater{Executable: program, Runner: &recordingRunner{output: []byte("aigw version 1.2.3\n")}}
	for phase, expected := range []struct{ current, previous string }{{"current", "previous"}, {"previous", "current"}} {
		if phase == 1 {
			if _, err := updater.Rollback(context.Background(), nil); err != nil {
				t.Fatal(err)
			}
		}
		observed, err := InspectInstallation(program, "test")
		if err != nil {
			t.Fatal(err)
		}
		for path, want := range map[string]string{program: expected.current, previous: expected.previous} {
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil {
				t.Fatal(err)
			}
			file := &observed.Payload
			if path == previous {
				file = observed.Rollback
			}
			if file == nil || file.Path != resolved || file.SizeBytes != int64(len(want)) ||
				file.SHA256 != fmt.Sprintf("%x", sha256.Sum256([]byte(want))) {
				t.Fatalf("phase %d file %s = %+v", phase, path, file)
			}
			if data, err := os.ReadFile(path); err != nil || string(data) != want {
				t.Fatalf("observation changed %s: %q, %v", path, data, err)
			}
		}
	}
}

func TestInstallationObservationReportsFileBoundaries(t *testing.T) {
	for _, state := range []string{"empty command", "missing command", "command directory", "rollback directory"} {
		t.Run(state, func(t *testing.T) {
			root := t.TempDir()
			program := filepath.Join(root, "aigw")
			want := "current portable program"
			switch state {
			case "empty command":
				program, want = " ", "command path"
			case "command directory":
				program = root
			case "rollback directory":
				if err := os.WriteFile(program, []byte("program"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(RollbackPath(program), 0o700); err != nil {
					t.Fatal(err)
				}
				want = "retained portable program"
			}
			result, err := InspectInstallation(program, "test")
			if err == nil || !strings.Contains(err.Error(), want) || !reflect.DeepEqual(result, Installation{}) {
				t.Fatalf("partial or successful %s observation: %+v, %v", state, result, err)
			}
		})
	}
}

func TestInstallationObservationResolvesRelativeCommand(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if err := os.WriteFile("aigw", []byte("program"), 0o755); err != nil {
		t.Fatal(err)
	}
	result, err := InspectInstallation("aigw", "test")
	if err != nil {
		t.Fatal(err)
	}
	command, err := filepath.Abs("aigw")
	if err != nil {
		t.Fatal(err)
	}
	if result.CommandPath != command || !filepath.IsAbs(result.Payload.Path) || result.Rollback != nil {
		t.Fatalf("relative command observation = %+v", result)
	}
}
