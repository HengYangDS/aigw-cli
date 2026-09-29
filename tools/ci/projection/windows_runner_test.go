package projection

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestGitLabWindowsVerifiesRunnerOwnedMiseBeforeRepositoryTools(t *testing.T) {
	projections, err := renderProjections(filepath.Clean(filepath.Join("..", "..", "..")))
	if err != nil {
		t.Fatal(err)
	}
	var gitlab struct {
		Windows struct {
			Script      []string `yaml:"script"`
			AfterScript []string `yaml:"after_script"`
		} `yaml:"native-windows"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &gitlab); err != nil {
		t.Fatal(err)
	}
	commands := gitlab.Windows.Script
	if len(commands) < 2 || !strings.Contains(commands[0], `Join-Path $env:ProgramFiles 'mise\bin\mise.exe'`) ||
		!strings.Contains(commands[0], "2026.9.16") ||
		!strings.Contains(commands[0], "a3e8a5e9850cb48dc0ec493820bcd6ab38ad5d431a304ee10b9e9c998977bcd8") ||
		!strings.Contains(commands[0], "Get-FileHash -LiteralPath $mise -Algorithm SHA256") ||
		!strings.Contains(commands[1], "mise install --locked") {
		t.Fatalf("Windows runner Mise identity is not pinned before the locked toolchain: %v", commands)
	}
	if len(gitlab.Windows.AfterScript) != 1 || !strings.Contains(gitlab.Windows.AfterScript[0], "ci-mise-$env:CI_JOB_ID") {
		t.Fatalf("Windows Mise job state has no exact cleanup: %v", gitlab.Windows.AfterScript)
	}
	for _, required := range []string{
		"whoami.exe /user",
		"$shim = Join-Path (Split-Path -Parent $mise) 'mise-shim.exe'",
		"Get-FileHash -LiteralPath $shim -Algorithm SHA256",
		"ab81436773ad4c377c62a026b5869e9838bc85b1c3eea46725ba5440aec637ad",
		"$shimsDirectory = Join-Path $env:MISE_DATA_DIR 'shims'",
		"Copy-Item -LiteralPath $shim -Destination $probeTarget -ErrorAction Stop",
		"Remove-Item -LiteralPath $probeDirectory -Recurse -Force -ErrorAction Stop",
		"icacls.exe $shim",
	} {
		if !strings.Contains(commands[0], required) {
			t.Errorf("Windows Mise preflight omits %q", required)
		}
	}
	hashReads := 0
	for line := range strings.SplitSeq(commands[0], "\n") {
		if strings.Contains(line, "Get-FileHash -LiteralPath") {
			hashReads++
			if !strings.Contains(line, "-ErrorAction Stop") {
				t.Errorf("Windows hash read can hide its actual failure: %s", line)
			}
		}
	}
	if hashReads != 3 {
		t.Errorf("Windows preflight has %d hash reads, want three exact executable and copy checks", hashReads)
	}
	for _, required := range []string{
		"robocopy.exe $emptyDirectory $jobDirectory /MIR /R:1 /W:1",
		"$mirrorExit -ge 8",
		"[IO.Directory]::Delete($jobDirectory)",
		"$cause.GetType().FullName",
		"$cause.HResult",
	} {
		if !strings.Contains(gitlab.Windows.AfterScript[0], required) {
			t.Errorf("Windows cleanup lacks exact owned-tree mirror or structured failure %q", required)
		}
	}
}

func TestGitLabWindowsLockedToolsUseJobScopedMirror(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	type windowsJob struct {
		Script      []string `yaml:"script"`
		AfterScript []string `yaml:"after_script"`
	}
	var gitlab struct {
		NativeWindows       windowsJob `yaml:"native-windows"`
		NativeWindowsReview windowsJob `yaml:"native-windows-review"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &gitlab); err != nil {
		t.Fatal(err)
	}
	windows := gitlab.NativeWindows
	if len(windows.Script) < 2 {
		t.Fatalf("Windows job lacks the Mise preflight and locked install: %+v", windows)
	}
	if !strings.Contains(windows.Script[1], "mise install --locked") {
		t.Fatalf("Windows job does not install locked tools: %+v", windows)
	}
	const jobDirectory = `Join-Path (Split-Path -Parent $env:CI_PROJECT_DIR) "aigw-ci-mise-$env:CI_JOB_ID"`
	for name, job := range map[string]windowsJob{"protected": windows, "review": gitlab.NativeWindowsReview} {
		requireWindowsMiseJobStorage(t, name, job.Script, job.AfterScript, jobDirectory)
		if !strings.Contains(job.Script[0], `= "${mirrorBase}" + 'release-$1-$2-$3.json'`) {
			t.Errorf("%s Windows job expands Mise regex captures before Mise receives them", name)
		}
	}
	if len(gitlab.NativeWindowsReview.Script) == 0 || windows.Script[0] != gitlab.NativeWindowsReview.Script[0] ||
		!reflect.DeepEqual(windows.AfterScript, gitlab.NativeWindowsReview.AfterScript) {
		t.Fatal("protected and review Windows jobs use different Mise admission or cleanup")
	}
	if _, err := os.Stat(filepath.Join(root, "tools", "ci", "bootstrap", "mise-windows.ps1")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("obsolete Windows bootstrap remains: %v", err)
	}
	for _, required := range []string{"MISE_URL_REPLACEMENTS", "MISE_NETRC_FILE", "CI_SERVER_HOST", "CI_JOB_TOKEN", "packages/generic/mise-github/v1/", "release-$1-$2-$3.json", "https://github.com/", "https://api.github.com/", "icacls"} {
		if !strings.Contains(windows.Script[0], required) {
			t.Errorf("Windows job omits scoped mirror control %q", required)
		}
	}
	probe := strings.Index(windows.Script[0], "$reported = & $mise --version")
	if probe < 0 {
		t.Fatal("Windows job never probes the runner-owned Mise executable")
	}
	for _, name := range []string{"MISE_CONFIG_DIR", "MISE_CACHE_DIR", "MISE_STATE_DIR", "MISE_DATA_DIR"} {
		assignment := strings.Index(windows.Script[0], "'"+name+"'")
		if assignment < 0 || assignment > probe {
			t.Fatalf("%s is not owned by the job before Mise loads configuration", name)
		}
	}
	if assignment := strings.Index(windows.Script[0], "[Environment]::SetEnvironmentVariable($name, $path)"); assignment < 0 || assignment > probe ||
		!strings.Contains(windows.Script[0], "$path = Join-Path $jobDirectory $name") {
		t.Fatal("Windows job does not confine Mise state before its first invocation")
	}
	if trust := strings.Index(windows.Script[0], "$env:MISE_TRUSTED_CONFIG_PATHS = $env:CI_PROJECT_DIR"); trust < 0 || trust > probe {
		t.Fatal("Windows job does not trust its exact checkout before Mise walks config ancestors")
	}
	if !strings.Contains(windows.Script[0], "throw 'Runner-owned Mise failed to start under the job identity.'") {
		t.Fatal("Windows job conflates a Mise startup failure with a version mismatch")
	}
}

func requireWindowsMiseJobStorage(t *testing.T, name string, script, cleanup []string, directory string) {
	t.Helper()
	if len(script) == 0 || len(cleanup) != 1 ||
		!strings.Contains(script[0], "$jobDirectory = "+directory) ||
		!strings.Contains(cleanup[0], directory) ||
		!strings.Contains(cleanup[0], "Test-Path -LiteralPath $jobDirectory") {
		t.Fatalf("%s Mise lifecycle enters the Go module or lacks exact teardown", name)
	}
}
