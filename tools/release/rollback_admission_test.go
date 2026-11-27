package main

import (
	"aigw-cli/internal/platform"
	"aigw-cli/internal/program"
	"aigw-cli/tools/release/identity"
	"errors"
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
	version, err := identity.ReadProductVersion(root)
	if err != nil {
		t.Fatal(err)
	}
	baseline := requireNativeLifecycleBaseline(t, func() string { return buildNativeProgram(t, root, "0.0.0") })
	candidate, archive, checksums := nativeReleaseCandidate(t, root, version)
	journey := newNativeJourney(t, baseline, "https://unused.example.test", false)
	journey.run("setup", "--from", journey.manifest)
	original := readFile(t, journey.config)
	journey.run("self", "update", "--archive", archive, "--checksums", checksums)
	journey.crossSchema(journey.binary)
	journey.run("setup", "--from", journey.candidateTeamFile())
	// A predecessor of another schema cannot read the candidate's file, which
	// keeps it unable to export without an injected field.
	crossing := !journey.firstProgramReadsConfiguration()
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
	rejectedExport := exec.CommandContext(t.Context(), baseline, "config", "export")
	rejectedExport.Env = journey.environment
	_, exportErr := rejectedExport.CombinedOutput()
	if _, ok := errors.AsType[*exec.ExitError](exportErr); !ok {
		t.Fatalf("unsupported configuration did not fail predecessor export: %v", exportErr)
	}
	want := map[string]platform.FileSnapshot{}
	for _, path := range []string{journey.binary, program.RollbackPath(journey.binary), journey.config, journey.config + ".bak"} {
		snapshot, err := platform.CaptureFileSnapshot(path)
		if err != nil {
			t.Fatal(err)
		}
		want[path] = snapshot
	}
	command := exec.CommandContext(t.Context(), journey.binary, "self", "rollback")
	command.Env = journey.environment
	output, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "Program rollback did not complete") ||
		strings.Contains(string(output), "incompatible with the current configuration") {
		t.Fatalf("failed predecessor export lost its execution verdict: %v\n%s", err, output)
	}
	for path, expected := range want {
		actual, err := platform.CaptureFileSnapshot(path)
		if err != nil || !actual.Equal(expected) {
			t.Fatalf("rejected rollback changed %s: %v", path, err)
		}
	}
	journey.requireInstalledProgramFiles()
	if injected || crossing {
		// The predecessor reads the file that it wrote: the one before the
		// injected field, or the copy kept before the schema edits.
		if err := os.WriteFile(journey.config, original, 0o600); err != nil {
			t.Fatal(err)
		}
	} else {
		journey.run("config", "rollback", "--last-change")
	}
	journey.run("self", "rollback")
	journey.run("config", "export")
	journey.requireProgramBytes(baseline)
	journey.run("self", "update", "--archive", archive, "--checksums", checksums)
	journey.requireProgramBytes(candidate)
	journey.crossSchema(journey.binary)
	journey.run("config", "export")
	if crossing {
		// The predecessor cannot read the edited file that its uninstall
		// would withdraw the clients from.
		journey.source = candidate
	}
	journey.uninstallAndRequireInstallationRemoved()
}
