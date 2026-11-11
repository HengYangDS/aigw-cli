package projection

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"
)

func TestNativePowerShellAcceptancePreservesDeclaredEmptyTag(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("AIGW_NATIVE_ARGUMENT_WITNESS") == "1" {
		index := slices.Index(os.Args, "--")
		if index < 0 {
			t.Fatal("native PowerShell argument boundary is missing")
		}
		selected := os.Args[index+1:]
		want := []string{
			"--baseline-tag=v1.2.3", "--tag=", "--clients=false",
			"--diagnostic-client=" + os.Getenv("AIGW_NATIVE_DIAGNOSTIC_CLIENT"),
			"--performance-attribution=false",
		}
		if !slices.Equal(selected, want) {
			t.Fatalf("native PowerShell changed argument identity: %q", selected)
		}
		return
	}
	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		t.Fatalf("Windows native acceptance requires PowerShell: %v", err)
	}
	projections := canonicalProjections(t)
	var workflow struct {
		Jobs map[string]struct {
			Steps []struct {
				Name string `yaml:"name"`
				Run  string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &workflow); err != nil {
		t.Fatal(err)
	}
	steps := workflow.Jobs["native-windows"].Steps
	index := slices.IndexFunc(steps, func(step struct {
		Name string `yaml:"name"`
		Run  string `yaml:"run"`
	}) bool {
		return step.Name == "Run historical release acceptance"
	})
	if index < 0 {
		t.Fatal("Windows native command is missing")
	}
	_, suffix, found := strings.Cut(steps[index].Run, " --baseline-tag=")
	if !found {
		t.Fatal("Windows native command must declare the published baseline tag")
	}
	arguments := "--baseline-tag=" + strings.NewReplacer(
		"${{ inputs.native_clients && inputs.diagnostic_client == '' }}", "false",
		"${{ inputs.performance_attribution }}", "false",
	).Replace(suffix)
	script := "& '" + strings.ReplaceAll(executable, "'", "''") + "' '-test.run=^TestNativePowerShellAcceptancePreservesDeclaredEmptyTag$' -- " + arguments
	for _, client := range []string{"", "codex"} {
		// The bound stops a hung witness; a loaded runner is slower than an idle one.
		ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
		command := exec.CommandContext(ctx, pwsh, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", script)
		// A witness that outlives PowerShell keeps the output pipe open.
		command.WaitDelay = 5 * time.Second
		command.Env = append(os.Environ(),
			"AIGW_NATIVE_ARGUMENT_WITNESS=1", "AIGW_BASELINE_TAG=v1.2.3",
			"AIGW_CANDIDATE_TAG=", "AIGW_NATIVE_DIAGNOSTIC_CLIENT="+client,
		)
		output, err := command.CombinedOutput()
		cancel()
		if err != nil || !bytes.Contains(output, []byte("PASS")) {
			t.Fatalf("native PowerShell argument witness failed for client %q: %v\n%s", client, err, output)
		}
	}
}
