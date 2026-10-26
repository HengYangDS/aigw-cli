package projection

import (
	"bytes"
	"context"
	"fmt"
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
			"--input-release=" + os.Getenv("AIGW_NATIVE_INPUT_RELEASE"),
			"--input-sha256=" + os.Getenv("AIGW_NATIVE_INPUT_SHA256"),
			"--candidate-source=" + os.Getenv("AIGW_CANDIDATE_SOURCE"),
			"--candidate=" + fmt.Sprint(os.Getenv("AIGW_NATIVE_INPUT_RELEASE") != ""),
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
	for _, release := range []string{"", "native-inputs-" + strings.Repeat("a", 40)} {
		digest, source := "", ""
		if release != "" {
			digest, source = strings.Repeat("b", 64), strings.Repeat("a", 40)
		}
		arguments := "--baseline-tag=" + strings.NewReplacer(
			"${{ inputs.native_clients && inputs.diagnostic_client == '' }}", "false",
			"${{ inputs.performance_attribution }}", "false",
			"${{ inputs.input_release != '' }}", fmt.Sprint(release != ""),
		).Replace(suffix)
		script := "& '" + strings.ReplaceAll(executable, "'", "''") + "' '-test.run=^TestNativePowerShellAcceptancePreservesDeclaredEmptyTag$' -- " + arguments
		for _, client := range []string{"", "codex"} {
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			command := exec.CommandContext(ctx, pwsh, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", script)
			command.Env = append(os.Environ(),
				"AIGW_NATIVE_ARGUMENT_WITNESS=1", "AIGW_BASELINE_TAG=v1.2.3",
				"AIGW_CANDIDATE_TAG=", "AIGW_NATIVE_DIAGNOSTIC_CLIENT="+client,
				"AIGW_NATIVE_INPUT_RELEASE="+release, "AIGW_NATIVE_INPUT_SHA256="+digest,
				"AIGW_CANDIDATE_SOURCE="+source,
			)
			output, err := command.CombinedOutput()
			cancel()
			if err != nil || !bytes.Contains(output, []byte("PASS")) {
				t.Fatalf("native PowerShell argument witness failed for release %q, client %q: %v\n%s", release, client, err, output)
			}
		}
	}
}
