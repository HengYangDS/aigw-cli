package projection

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
)

func TestGitLabQualityAndControlRunnerSelectors(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var pipeline struct {
		Parity         gitLabJob `yaml:"accepted-ref-parity"`
		Quality        gitLabJob `yaml:"quality"`
		NativeDarwin   gitLabJob `yaml:"native-darwin"`
		ReleaseVersion gitLabJob `yaml:"release-version"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	want := declaredMacOSRunnerTags(t).Protected
	for name, job := range map[string]gitLabJob{
		"accepted-ref-parity": pipeline.Parity,
		"native-darwin":       pipeline.NativeDarwin,
		"release-version":     pipeline.ReleaseVersion,
	} {
		if !slices.Equal(job.Tags, want) {
			t.Errorf("%s runner tags = %q, want %q", name, job.Tags, want)
		}
	}
	if want := []string{"$AIGW_CI_LINUX_RUNNER_TAG"}; !slices.Equal(pipeline.Quality.Tags, want) {
		t.Errorf("quality runner tags = %q, want %q", pipeline.Quality.Tags, want)
	}
}

// The native command alone reads the review resource, and only on macOS.
func TestGitLabNativeReviewResourceBelongsToTheNativeCommand(t *testing.T) {
	projections := canonicalProjections(t)
	var pipeline struct {
		Darwin       gitLabJob `yaml:"native-darwin"`
		DarwinReview gitLabJob `yaml:"native-darwin-review"`
		Linux        gitLabJob `yaml:"native-linux"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	for name, job := range map[string]gitLabJob{
		"native-darwin": pipeline.Darwin, "native-darwin-review": pipeline.DarwinReview,
		"native-linux": pipeline.Linux,
	} {
		platform := strings.TrimSuffix(strings.TrimPrefix(name, "native-"), "-review")
		if !slices.ContainsFunc(job.Script, isNativeJobCommand(platform)) {
			t.Errorf("%s does not hand its inputs to the native command", name)
		}
		if strings.Contains(strings.Join(append(slices.Clone(job.BeforeScript), job.Script...), "\n"), "AIGW_REVIEW_PROTECTED_FILE") {
			t.Errorf("%s reads the review resource outside the native command", name)
		}
	}
}

func TestGitLabLinuxJobsSelectRunnerByRefTrust(t *testing.T) {
	projections := canonicalProjections(t)
	var pipeline map[string]yaml.Node
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	const selector = "AIGW_CI_LINUX_RUNNER_TAG"
	for _, name := range []string{"quality", "native-linux", "linux-secret-service", "release-assets"} {
		node := pipeline[name]
		var job gitLabJob
		if err := node.Decode(&job); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(job.Tags, []string{"$" + selector}) {
			t.Errorf("%s relies on an external runner selector: %q", name, job.Tags)
		}
		for _, rule := range job.Rules {
			if _, duplicated := rule.Variables[selector]; duplicated {
				t.Errorf("%s duplicates workflow runner policy", name)
			}
		}
	}
	var workflow struct {
		Rules []struct {
			If        string            `yaml:"if"`
			When      string            `yaml:"when"`
			Variables map[string]string `yaml:"variables"`
		} `yaml:"rules"`
	}
	node := pipeline["workflow"]
	if err := node.Decode(&workflow); err != nil {
		t.Fatal(err)
	}
	if len(workflow.Rules) != 6 {
		t.Fatal("workflow must cover tag, review, protected push and both manual ref states")
	}
	for _, rule := range workflow.Rules {
		if rule.When == "never" {
			continue
		}
		want := "ci-linux-arm64-container-protected"
		if strings.Contains(rule.If, "merge_request_event") || strings.Contains(rule.If, `$CI_COMMIT_REF_PROTECTED == "false"`) {
			want = "ci-linux-arm64-container"
		}
		if got := rule.Variables[selector]; got != want {
			t.Errorf("workflow event %q selects %q, want %q", rule.If, got, want)
		}
	}
}

func TestGitHubNativeJobsUseHostedRunners(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}

	var workflow struct {
		On struct {
			Dispatch struct {
				Inputs map[string]struct {
					Type        string    `yaml:"type"`
					Default     yaml.Node `yaml:"default"`
					Description string    `yaml:"description"`
				} `yaml:"inputs"`
			} `yaml:"workflow_dispatch"`
		} `yaml:"on"`
		Jobs map[string]struct {
			RunsOn yaml.Node `yaml:"runs-on"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &workflow); err != nil {
		t.Fatal(err)
	}

	for _, input := range []string{"self_hosted_linux_arm64"} {
		if _, present := workflow.On.Dispatch.Inputs[input]; present {
			t.Errorf("GitHub Verify retains obsolete input %q", input)
		}
	}
	// Each native job runs on its platform's declared runner, and every other
	// job on the Linux one.
	var runners map[string]string
	declaredValue(t, "{for platform, evidence in nativeEvidence {(platform): evidence.github.runner}}", &runners)
	if len(runners) == 0 {
		t.Fatal("the declaration names no GitHub runner")
	}
	expected := map[string]string{
		"accepted-ref-parity":  runners["linux"],
		"linux-secret-service": runners["linux"],
		"quality":              runners["linux"],
	}
	for platform, runner := range runners {
		expected["native-"+platform] = runner
	}
	for name, runner := range expected {
		if got := workflow.Jobs[name].RunsOn.Value; got != runner {
			t.Errorf("%s runner = %q, want %q", name, got, runner)
		}
	}
}

func TestForgeProjectionsIncludeTheCompleteNativeMatrix(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var gitlab struct {
		NativeDarwin  *gitLabJob `yaml:"native-darwin"`
		NativeLinux   *gitLabJob `yaml:"native-linux"`
		SecretService *gitLabJob `yaml:"linux-secret-service"`
		Assets        gitLabJob  `yaml:"release-assets"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &gitlab); err != nil {
		t.Fatal(err)
	}
	if gitlab.NativeDarwin == nil || gitlab.NativeLinux == nil || gitlab.SecretService == nil {
		t.Fatal("GitLab projection lacks required native evidence")
	}
	if strings.Contains(projections[0].Content, "allow_failure:") {
		t.Fatal("GitLab projection weakens a native job with allow_failure")
	}
	var gotNeeds []string
	for _, need := range gitlab.Assets.Needs {
		gotNeeds = append(gotNeeds, need.Job)
	}
	if want := []string{"quality", "native-darwin", "native-linux", "linux-secret-service", "release-version"}; !slices.Equal(gotNeeds, want) {
		t.Fatalf("GitLab release dependencies = %q, want %q", gotNeeds, want)
	}

	for _, projectionIndex := range []int{1} {
		var github struct {
			Jobs map[string]any `yaml:"jobs"`
		}
		if err := yaml.Unmarshal([]byte(projections[projectionIndex].Content), &github); err != nil {
			t.Fatal(err)
		}
		for _, platform := range []string{"darwin", "linux"} {
			if _, present := github.Jobs["native-"+platform]; !present {
				t.Fatalf("GitHub projection %d lacks required native %s evidence", projectionIndex, platform)
			}
		}
	}
}

// macosRunnerMise is the runner-owned Mise that the CUE authority pins for
// GitLab's macOS ARM64 host.
type macosRunnerMise struct {
	Version    string `yaml:"version"`
	Executable string `yaml:"executable"`
	Directory  string `yaml:"directory"`
}

func pinnedMacOSRunnerMise(t *testing.T, root string) macosRunnerMise {
	t.Helper()
	output, err := projectionCommand(root, "{version: miseVersion, executable: miseMacosArm64ExecutableSHA256, directory: miseMacosArm64Directory}").Output()
	if err != nil {
		t.Fatal(err)
	}
	var identity macosRunnerMise
	if err := yaml.Unmarshal(output, &identity); err != nil {
		t.Fatal(err)
	}
	if identity.Version == "" || len(identity.Executable) != 64 {
		t.Fatal("CUE must declare an exact macOS Mise version and executable digest")
	}
	// Homebrew's Mise belongs to every project on the host, so AIGW's pinned
	// release must live in a directory of its own.
	if !strings.HasPrefix(identity.Directory, "/opt/mise-aigw-cli/") {
		t.Fatalf("CUE must name an AIGW-only runner Mise directory, not %q", identity.Directory)
	}
	// The directory names its release, so the host can hold the next release
	// beside the one that running jobs verify.
	if !slices.Contains(strings.Split(identity.Directory, "/"), identity.Version) {
		t.Fatalf("CUE must name the runner Mise directory after its release %q, not %q", identity.Version, identity.Directory)
	}
	return identity
}

func TestGitLabMacOSVerifiesRunnerOwnedMiseBeforeRepositoryTools(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	identity := pinnedMacOSRunnerMise(t, root)
	tags := declaredMacOSRunnerTags(t)
	var gitlab map[string]yaml.Node
	if err := yaml.Unmarshal([]byte(projections[0].Content), &gitlab); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"default", "before_script"} {
		if _, found := gitlab[key]; found {
			t.Fatalf("a top-level %s could run commands before the macOS Mise preflight", key)
		}
	}
	var macos []string
	steps := map[string]bool{}
	for name, node := range gitlab {
		var job struct {
			Tags         []string  `yaml:"tags"`
			Extends      yaml.Node `yaml:"extends"`
			BeforeScript []string  `yaml:"before_script"`
			Script       []string  `yaml:"script"`
		}
		if node.Kind != yaml.MappingNode {
			continue
		}
		if err := node.Decode(&job); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !slices.Equal(job.Tags, tags.Protected) && !slices.Equal(job.Tags, tags.Review) {
			continue
		}
		macos = append(macos, name)
		install := slices.IndexFunc(job.Script, func(command string) bool { return strings.Contains(command, "mise install --locked") })
		// A template that the job extends could bring a before_script of its own.
		if install < 1 || len(job.BeforeScript) != 0 || job.Extends.Kind != 0 {
			t.Errorf("%s: no Mise preflight precedes every command: %v %v", name, job.BeforeScript, job.Script)
			continue
		}
		// One text serves every macOS job, so the test that runs native-darwin's
		// step covers them all.
		steps[job.Script[0]] = true
		for _, required := range []string{
			"mise='" + identity.Directory + "/mise'",
			`shasum -a 256 "$mise"`,
			"'" + identity.Executable + "'",
			"'" + identity.Version + " '*)",
			`export PATH="` + identity.Directory + `:$PATH"`,
			"unset -f mise",
			`"$(command -v mise)" != "$mise"`,
			"Runner-owned Mise verified: ",
		} {
			if !strings.Contains(job.Script[0], required) {
				t.Errorf("%s: macOS Mise preflight omits %q", name, required)
			}
		}
	}
	slices.Sort(macos)
	if want := []string{"accepted-ref-parity", "native-darwin", "native-darwin-review", "release-version"}; !slices.Equal(macos, want) {
		t.Fatalf("macOS GitLab jobs = %v, want %v", macos, want)
	}
	if len(steps) != 1 {
		t.Fatalf("macOS GitLab jobs carry %d different Mise preflights, want one", len(steps))
	}
}
