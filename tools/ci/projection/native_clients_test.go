package projection

import (
	"slices"
	"strings"
	"testing"

	hermesclient "aigw-cli/internal/client/hermes"
	"aigw-cli/tools/release/construction"

	"go.yaml.in/yaml/v3"
)

func TestNativeClientSupplyMatchesEachHostsJourneys(t *testing.T) {
	projections := canonicalProjections(t)
	type step struct {
		Name           string            `yaml:"name"`
		If             string            `yaml:"if"`
		Run            string            `yaml:"run"`
		TimeoutMinutes int               `yaml:"timeout-minutes"`
		Env            map[string]string `yaml:"env"`
	}
	var workflow struct {
		Jobs map[string]struct {
			TimeoutMinutes string `yaml:"timeout-minutes"`
			Steps          []step `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &workflow); err != nil {
		t.Fatal(err)
	}
	const selection = "github.event_name == 'workflow_dispatch' && inputs.native_clients && inputs.baseline_tag != ''"
	const hermes = " --hermes-commit bf0203a794a07aac3ff764972ab9087baa4e4f4e --hermes-installer-sha256 99cb3691d55019c6973a7acb775a915648666cc24e483ca827a1cdbb3531a674"
	for _, platform := range []string{"darwin", "linux", "windows"} {
		job := workflow.Jobs["native-"+platform]
		steps := job.Steps
		supplyIndex := slices.IndexFunc(steps, func(item step) bool { return item.Name == "Prepare official native clients" })
		nativeIndex := slices.IndexFunc(steps, func(item step) bool { return item.Name == "Run historical release acceptance" })
		cleanupIndex := slices.IndexFunc(steps, func(item step) bool { return item.Name == "Remove official native client supply" })
		if supplyIndex < 0 || nativeIndex <= supplyIndex || cleanupIndex <= nativeIndex {
			t.Fatalf("%s native supply ordering: supply=%d native=%d cleanup=%d", platform, supplyIndex, nativeIndex, cleanupIndex)
		}
		supply, prefix, deadline := steps[supplyIndex], "$", 20
		if platform == "windows" {
			prefix, deadline = "$env:", 12
		}
		pins := ""
		if slices.Contains(construction.NativeAcceptanceClients(platform), hermesclient.ID) {
			pins = hermes
		}
		for _, field := range []struct{ got, want string }{
			{supply.If, selection}, {steps[cleanupIndex].If, "always() && " + selection},
			{supply.Env["GIT_TERMINAL_PROMPT"], "0"},
			{supply.Run, `mise exec --locked -- go run ./tools/ci native-clients supply --root "` + prefix + `RUNNER_TEMP" --github-env "` + prefix + `GITHUB_ENV" --codex 0.164.0 --claude 2.1.310` + pins},
			{steps[cleanupIndex].Run, "mise exec --locked -- go run ./tools/ci native-clients retire"},
			{job.TimeoutMinutes, "${{ inputs.native_clients && 45 || 25 }}"},
		} {
			if field.got != field.want {
				t.Fatalf("%s official supply input = %q, want %q", platform, field.got, field.want)
			}
		}
		if supply.TimeoutMinutes != deadline {
			t.Fatalf("%s official supply deadline = %d minutes, want %d", platform, supply.TimeoutMinutes, deadline)
		}
		for _, key := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
			if value, present := supply.Env[key]; !present || value != "" {
				t.Fatalf("%s official installation inherits %s", platform, key)
			}
		}
		// Codex's Linux sandbox needs the distribution's bubblewrap and its
		// AppArmor profile on Ubuntu 24.04, as OpenAI's sandbox prerequisites
		// state; the runner's user-namespace restriction otherwise stays on.
		sandboxIndex := slices.IndexFunc(steps, func(item step) bool { return item.Name == "Prepare the Linux client sandbox" })
		if platform != "linux" {
			if sandboxIndex >= 0 {
				t.Fatalf("%s prepares the Linux client sandbox", platform)
			}
			continue
		}
		if sandboxIndex < 0 || sandboxIndex > supplyIndex {
			t.Fatalf("Linux client sandbox step = %d, supply step = %d", sandboxIndex, supplyIndex)
		}
		if steps[sandboxIndex].If != selection {
			t.Fatalf("Linux client sandbox condition = %q, want %q", steps[sandboxIndex].If, selection)
		}
		sandbox := steps[sandboxIndex].Run
		for _, command := range []string{
			"install --no-install-recommends -y bubblewrap apparmor-profiles apparmor-utils\n",
			"sudo -n install -m 0644 /usr/share/apparmor/extra-profiles/bwrap-userns-restrict /etc/apparmor.d/bwrap-userns-restrict\n",
			"sudo -n apparmor_parser -r /etc/apparmor.d/bwrap-userns-restrict",
		} {
			if !strings.Contains(sandbox, command) {
				t.Fatalf("Linux client sandbox lacks %q:\n%s", command, sandbox)
			}
		}
		if strings.Contains(sandbox, "apparmor_restrict_unprivileged_userns") {
			t.Fatalf("Linux client sandbox lifts the runner's user-namespace restriction:\n%s", sandbox)
		}
	}
}
