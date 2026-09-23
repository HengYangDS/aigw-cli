package main

import (
	"aigw-cli/internal/transaction"
	"aigw-cli/internal/upgrade"
	"aigw-cli/tools/release/readiness"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeRollbackConfigurationAdmission(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	version, err := readiness.ReadProductVersion(root)
	if err != nil {
		t.Fatal(err)
	}
	baseline := requireNativeLifecycleBaseline(t, func() string { return buildNativeProgram(t, root, "0.0.0") })
	candidate, archive, checksums := nativeReleaseCandidate(t, root, version)
	journey := newNativeJourney(t, baseline, "https://unused.example.test", false)
	journey.run("setup", "--from", journey.manifest)
	original := readFile(t, journey.config)
	journey.run("update", "--candidate", archive, "--checksums", checksums)
	journey.run("config", "import", journey.manifest)
	compatibility := exec.CommandContext(t.Context(), baseline, "config", "export")
	compatibility.Env = journey.environment
	_, compatibilityErr := compatibility.CombinedOutput()
	injected := compatibilityErr == nil
	if injected {
		unsupported := append([]byte("unsupported_future_field = true\n"), readFile(t, journey.config)...)
		if err := os.WriteFile(journey.config, unsupported, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want := map[string]transaction.FileSnapshot{}
	for _, path := range []string{journey.binary, upgrade.RollbackPath(journey.binary), journey.config, journey.config + ".bak"} {
		snapshot, err := transaction.CaptureFileSnapshot(path)
		if err != nil {
			t.Fatal(err)
		}
		want[path] = snapshot
	}
	command := exec.CommandContext(t.Context(), journey.binary, "update", "--rollback")
	command.Env = journey.environment
	output, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "incompatible with the current configuration") {
		t.Fatalf("unsafe predecessor activation: %v\n%s", err, output)
	}
	for path, expected := range want {
		actual, err := transaction.CaptureFileSnapshot(path)
		if err != nil || !actual.Equal(expected) {
			t.Fatalf("rejected rollback changed %s: %v", path, err)
		}
	}
	journey.requireInstalledProgramFiles()
	if injected {
		if err := os.WriteFile(journey.config, original, 0o600); err != nil {
			t.Fatal(err)
		}
	} else {
		journey.run("rollback", "--last-change")
	}
	journey.run("update", "--rollback")
	journey.run("config", "export")
	journey.requireProgramBytes(baseline)
	journey.run("update", "--candidate", archive, "--checksums", checksums)
	journey.requireProgramBytes(candidate)
	journey.run("config", "export")
	journey.uninstallAndRequireOwnedFilesAbsent()
}
