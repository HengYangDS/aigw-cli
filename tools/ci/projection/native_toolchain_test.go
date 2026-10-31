package projection

import (
	"maps"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestNativeJobsEnableTheirExactCommandToolClosure(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	checkGitLabNativeToolClosure(t, projections[0].Content)
	for _, item := range projections[1:] {
		if item.Path == ".github/workflows/verify.yml" {
			checkGitHubNativeToolClosure(t, item)
			return
		}
	}
	t.Fatal("GitHub verification projection is missing")
}

func TestLinuxSecretServiceUsesOnlyItsLockedExecutableClosure(t *testing.T) {
	projections := canonicalProjections(t)
	const tools = "go,github:goreleaser/goreleaser"
	var gitlab struct {
		SecretService gitLabJob `yaml:"linux-secret-service"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &gitlab); err != nil {
		t.Fatal(err)
	}
	if got := gitlab.SecretService.Variables["MISE_ENABLE_TOOLS"]; got != tools {
		t.Fatalf("GitLab Secret Service toolchain = %q, want %q", got, tools)
	}
	if len(gitlab.SecretService.Script) != 2 || !strings.Contains(gitlab.SecretService.Script[0], "TestNativeProductJourney/system_credential_store") ||
		!strings.Contains(gitlab.SecretService.Script[1], "Mise job-owned supply state retired.") {
		t.Fatal("GitLab Secret Service job runs more than its focused qualification")
	}
	var github struct {
		Jobs map[string]struct {
			Env   map[string]string `yaml:"env"`
			Steps []struct {
				Name string `yaml:"name"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &github); err != nil {
		t.Fatal(err)
	}
	job, present := github.Jobs["linux-secret-service"]
	if !present || job.Env["MISE_ENABLE_TOOLS"] != tools {
		t.Fatalf("GitHub Secret Service toolchain = %q, present=%t", job.Env["MISE_ENABLE_TOOLS"], present)
	}
	if len(job.Steps) != 3 || job.Steps[2].Name != "Qualify Linux Secret Service" {
		t.Fatal("GitHub Secret Service job runs more than its focused qualification")
	}
}

func TestNativePrebuiltAcceptanceKeepsBootstrapConditional(t *testing.T) {
	projections := canonicalProjections(t)
	var gitlab struct {
		Linux   gitLabJob `yaml:"native-linux"`
		Darwin  gitLabJob `yaml:"native-darwin-review"`
		Windows gitLabJob `yaml:"native-windows-review"`
		Quality gitLabJob `yaml:"quality"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &gitlab); err != nil {
		t.Fatal(err)
	}
	manualQuality := gitlab.Quality.Rules[len(gitlab.Quality.Rules)-2].If
	for _, required := range []string{"AIGW_CANDIDATE_ARTIFACTS", "AIGW_CANDIDATE_TAG", "AIGW_FULL_NATIVE_QUALITY", "AIGW_REFRESH_LOCKS"} {
		if !strings.Contains(manualQuality, required) {
			t.Errorf("manual quality admission ignores artifact scope %s", required)
		}
	}
	for name, job := range map[string]gitLabJob{"native-linux": gitlab.Linux, "native-darwin-review": gitlab.Darwin, "native-windows-review": gitlab.Windows} {
		for _, script := range job.Script {
			if script == "mise run bootstrap" {
				t.Errorf("%s installs the source toolchain unconditionally before prebuilt acceptance", name)
			}
		}
		if !slices.ContainsFunc(job.Rules, func(rule gitLabRule) bool {
			return strings.Contains(rule.If, "$AIGW_CANDIDATE_ARTIFACTS") && strings.HasPrefix(rule.Variables["MISE_ENABLE_TOOLS"], "go,glab,github:goreleaser/goreleaser")
		}) {
			t.Errorf("%s lacks the explicit prebuilt tool closure", name)
		}
	}
	var github struct {
		Jobs map[string]struct {
			If    string `yaml:"if"`
			Steps []struct {
				Run string `yaml:"run"`
				If  string `yaml:"if"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &github); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(github.Jobs["quality"].If, "inputs.candidate_tag") || !strings.Contains(github.Jobs["quality"].If, "inputs.full_quality") {
		t.Fatal("manual prebuilt acceptance still installs the quality toolchain")
	}
	for _, name := range []string{"native-linux", "native-darwin", "native-windows"} {
		for _, step := range github.Jobs[name].Steps {
			if step.Run == "mise run bootstrap" && !strings.Contains(step.If, "inputs.candidate_tag") {
				t.Errorf("%s does not scope source bootstrap away from explicit prebuilt inputs", name)
			}
		}
	}
}

// Each admission rule of a native job is preceded by its artifact-only scopes,
// so GitLab selects the same tool closure the former shell selected.
func TestGitLabArtifactScopesPrecedeEachNativeAdmission(t *testing.T) {
	projections := canonicalProjections(t)
	var pipeline map[string]yaml.Node
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	const prebuilt = `($AIGW_NATIVE_INPUT_PACKAGE || $AIGW_CANDIDATE_ARTIFACTS || $AIGW_CANDIDATE_TAG) && $AIGW_FULL_NATIVE_QUALITY != "true" && $AIGW_REFRESH_LOCKS != "true"`
	const performance, clients = ` && $AIGW_NATIVE_PERFORMANCE == "true"`, ` && $AIGW_NATIVE_INPUT_PACKAGE && ($AIGW_NATIVE_CLIENTS == "true" || $AIGW_NATIVE_DIAGNOSTIC_CLIENT)`
	for _, name := range []string{"native-linux", "native-darwin", "native-darwin-review", "native-windows", "native-windows-review"} {
		t.Run(name, func(t *testing.T) {
			platform := strings.TrimSuffix(strings.TrimPrefix(name, "native-"), "-review")
			// A GitLab artifact journey prepares glab, never the GitHub CLI.
			tools := "go,glab,github:goreleaser/goreleaser"
			if platform == "darwin" {
				tools += ",github:indygreg/apple-platform-rs"
			}
			type scope struct{ condition, tools string }
			scopes := []scope{{prebuilt + performance, tools + ",github:sharkdp/hyperfine"}, {prebuilt, tools}}
			if platform == "windows" {
				scopes = []scope{
					{prebuilt + performance + clients, tools + ",github:sharkdp/hyperfine,node"},
					{prebuilt + performance, tools + ",github:sharkdp/hyperfine"},
					{prebuilt + clients, tools + ",node"},
					{prebuilt, tools},
				}
			}
			var job gitLabJob
			node := pipeline[name]
			if err := node.Decode(&job); err != nil {
				t.Fatal(err)
			}
			admitted := admissionRules(job.Rules)
			if len(admitted) < 2 || admitted[len(admitted)-1].When != "never" || len(job.Rules) != (len(admitted)-1)*(len(scopes)+1)+1 {
				t.Fatalf("%s rules = %#v", name, job.Rules)
			}
			for index, admission := range admitted[:len(admitted)-1] {
				start := index * (len(scopes) + 1)
				for offset, selected := range scopes {
					want := map[string]string{"MISE_ENABLE_TOOLS": selected.tools}
					if platform == "linux" {
						want["CGO_ENABLED"], want["AIGW_LINUX_COMPILER_PACKAGES"] = "0", ""
					}
					if rule := job.Rules[start+offset]; rule.If != "("+admission.If+") && "+selected.condition || !maps.Equal(rule.Variables, want) {
						t.Fatalf("%s scope %d of %q = %#v, want %q with %v", name, offset, admission.If, rule, selected.condition, want)
					}
				}
				if job.Rules[start+len(scopes)].If != admission.If {
					t.Fatalf("%s admission %q does not follow its artifact scopes", name, admission.If)
				}
			}
		})
	}
}

func TestLinuxSecretServiceHasItsOwnRequiredJob(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			If    string `yaml:"if"`
			Steps []struct {
				Name string `yaml:"name"`
				Run  string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &workflow); err != nil {
		t.Fatal(err)
	}
	prepared := false
	for _, step := range workflow.Jobs["native-linux"].Steps {
		if step.Name == "Prepare native Secret Service" {
			prepared = strings.Contains(step.Run, "dbus-x11 gnome-keyring libglib2.0-bin")
		}
		if step.Name == "Qualify Linux Secret Service" {
			t.Fatal("native Linux job duplicates the Secret Service qualification")
		}
	}
	if !prepared {
		t.Fatal("Linux source coverage lacks its native Secret Service preparation")
	}
	secretService := workflow.Jobs["linux-secret-service"]
	if len(secretService.Steps) != 3 || secretService.Steps[2].Name != "Qualify Linux Secret Service" {
		t.Fatal("GitHub requires independent native Linux Secret Service qualification")
	}
	if secretService.If != workflow.Jobs["quality"].If {
		t.Fatal("GitHub Secret Service qualification must follow source admission, not artifact-only native selection")
	}
	githubQualification := secretService.Steps[2].Run
	if strings.Contains(githubQualification, "./internal/secret/native") {
		t.Fatal("standalone Secret Service job repeats source-level credential qualification")
	}
	for _, required := range []string{
		"dbus-x11 gnome-keyring",
		"sudo -n timeout --verbose --kill-after=5s 240s",
		"Acquire::http::Timeout=30",
		"dbus-run-session",
		"SetAlias default /org/freedesktop/secrets/collection/session",
		"AIGW_VERIFY_SYSTEM_KEYRING=1",
		"TestNativeProductJourney/system_credential_store",
		"AIGW_VERIFY_SYSTEM_KEYRING=1 mise exec --locked -- go test ./tools/release -run \"^TestNativeProductJourney/system_credential_store$\" -count=1 -v\nAIGW_SECRET_SERVICE",
	} {
		if !strings.Contains(githubQualification, required) {
			t.Fatalf("Linux Secret Service qualification omits %q", required)
		}
	}
	var gitlab struct {
		Quality       gitLabJob  `yaml:"quality"`
		Linux         gitLabJob  `yaml:"native-linux"`
		SecretService *gitLabJob `yaml:"linux-secret-service"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &gitlab); err != nil {
		t.Fatal(err)
	}
	if gitlab.SecretService == nil {
		t.Fatal("GitLab lacks independent Linux Secret Service evidence")
	}
	if strings.Contains(strings.Join(gitlab.Linux.Script, "\n"), "TestNativeProductJourney/system_credential_store") {
		t.Fatal("GitLab native Linux job duplicates Secret Service qualification")
	}
	if !slices.Equal(gitlab.SecretService.Extends, []string{".linux-toolchain"}) ||
		!slices.Equal(gitlab.SecretService.Tags, gitlab.Linux.Tags) ||
		!reflect.DeepEqual(gitlab.SecretService.Rules, gitlab.Quality.Rules) {
		t.Fatal("GitLab Secret Service job must use its Linux runner and source admission")
	}
	if len(gitlab.SecretService.Script) != 2 {
		t.Fatal("GitLab native Linux CI does not qualify real Secret Service")
	}
	gitlabQualification := gitlab.SecretService.Script[0]
	for _, required := range []string{
		"DEBIAN_FRONTEND=noninteractive timeout --verbose --kill-after=5s 240s",
		"Acquire::http::Timeout=30",
		"install --no-install-recommends -y dbus-x11 gnome-keyring libglib2.0-bin",
	} {
		if !strings.Contains(gitlabQualification, required) {
			t.Fatalf("GitLab Secret Service preparation omits %q", required)
		}
	}
	githubBus := strings.Index(githubQualification, "dbus-run-session")
	gitlabBus := strings.Index(gitlabQualification, "dbus-run-session")
	if githubBus < 0 || gitlabBus < 0 || githubQualification[githubBus:] != gitlabQualification[gitlabBus:] {
		t.Fatal("GitHub and GitLab native Linux jobs must run the same Secret Service journey")
	}
}

func TestLinuxCompilerPrerequisitesBelongOnlyToNativeRaceExecution(t *testing.T) {
	projections := canonicalProjections(t)
	var pipeline struct {
		Toolchain     gitLabJob `yaml:".linux-toolchain"`
		Quality       gitLabJob `yaml:"quality"`
		SecretService gitLabJob `yaml:"linux-secret-service"`
		NativeLinux   gitLabJob `yaml:"native-linux"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	for name, job := range map[string]gitLabJob{
		"quality": pipeline.Quality, "linux-secret-service": pipeline.SecretService,
	} {
		if job.Variables["CGO_ENABLED"] != "0" {
			t.Errorf("%s adds a compiler without executing cgo or race", name)
		}
	}
	shared := strings.Join(pipeline.Toolchain.BeforeScript, "\n")
	for _, compiler := range []string{" gcc ", " libc6-dev "} {
		if strings.Contains(shared, compiler) {
			t.Errorf("shared bootstrap downloads native-race prerequisite %q", compiler)
		}
	}
	if pipeline.NativeLinux.Variables["CGO_ENABLED"] != "1" || pipeline.NativeLinux.Variables["AIGW_LINUX_COMPILER_PACKAGES"] != "gcc libc6-dev" || len(pipeline.NativeLinux.BeforeScript) < 2 {
		t.Fatal("native Linux race lacks its explicit compiler prerequisite")
	}
	if compiler := pipeline.NativeLinux.BeforeScript[1]; !strings.Contains(compiler, " install --no-install-recommends -y $AIGW_LINUX_COMPILER_PACKAGES dbus-x11 gnome-keyring libglib2.0-bin") {
		t.Fatalf("native Linux compiler preparation = %q", compiler)
	}
	if runtime.GOOS != "windows" {
		checkLinuxNativeSupplySelection(t, pipeline.NativeLinux.BeforeScript[1])
	}
	var github struct {
		Jobs map[string]struct {
			Env   map[string]string `yaml:"env"`
			Steps []struct {
				Name string            `yaml:"name"`
				Env  map[string]string `yaml:"env"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &github); err != nil {
		t.Fatal(err)
	}
	if github.Jobs["linux-secret-service"].Env["CGO_ENABLED"] != "0" {
		t.Fatal("GitHub adds cgo to the pure-Go Secret Service journey")
	}
	for _, step := range github.Jobs["quality"].Steps {
		if step.Name == "Run quality and governance" && step.Env["CGO_ENABLED"] != "0" {
			t.Fatal("GitHub static quality adds a compiler without running race")
		}
	}
}

func checkGitLabNativeToolClosure(t *testing.T, content string) {
	t.Helper()
	var pipeline struct {
		NativeDarwin  *gitLabJob `yaml:"native-darwin"`
		NativeLinux   *gitLabJob `yaml:"native-linux"`
		NativeWindows *gitLabJob `yaml:"native-windows"`
	}
	if err := yaml.Unmarshal([]byte(content), &pipeline); err != nil {
		t.Fatal(err)
	}
	if pipeline.NativeDarwin == nil || pipeline.NativeLinux == nil || pipeline.NativeWindows == nil {
		t.Fatal("GitLab must project all three native acceptance jobs")
	}
	for name, job := range map[string]gitLabJob{
		"native-darwin":  *pipeline.NativeDarwin,
		"native-linux":   *pipeline.NativeLinux,
		"native-windows": *pipeline.NativeWindows,
	} {
		enabled := strings.Split(job.Variables["MISE_ENABLE_TOOLS"], ",")
		// The native Go suite includes real glab loopback tests in internal/program.
		for _, tool := range []string{"go", "node", "npm", "github:golangci/golangci-lint", "github:goreleaser/goreleaser", "github:anchore/syft", "github:lycheeverse/lychee", "gh", "glab"} {
			if !slices.Contains(enabled, tool) {
				t.Errorf("GitLab %s lacks native acceptance tool %s", name, tool)
			}
		}
		hasDarwinSigner := slices.Contains(enabled, "github:indygreg/apple-platform-rs")
		if hasDarwinSigner != (name == "native-darwin") {
			t.Errorf("GitLab %s Darwin signer presence = %t", name, hasDarwinSigner)
		}
		// The native command bootstraps locked dependencies before source qualification.
		native := slices.Index(job.Script, "mise exec --locked -- go run ./tools/ci native --platform "+strings.TrimPrefix(name, "native-")+" --from-env")
		if native < 0 || native >= len(job.Script)-1 {
			t.Errorf("GitLab %s must prepare locked dependencies before native acceptance", name)
		}
	}
}

func checkGitHubNativeToolClosure(t *testing.T, projection projection) {
	t.Helper()
	var workflow struct {
		Jobs map[string]struct {
			Env   map[string]string `yaml:"env"`
			Steps []struct {
				Run string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projection.Content), &workflow); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"native-darwin", "native-linux", "native-windows"} {
		job := workflow.Jobs[name]
		tools := job.Env["MISE_ENABLE_TOOLS"]
		for _, required := range []string{"go,node,npm", "github:golangci/golangci-lint", "github:goreleaser/goreleaser", "github:anchore/syft", "github:lycheeverse/lychee", "gh", "glab"} {
			if !strings.Contains(tools, required) {
				t.Errorf("%s %s tool closure lacks %q: %q", projection.Path, name, required, tools)
			}
		}
		hasDarwinSigner := strings.Contains(tools, "github:indygreg/apple-platform-rs")
		if hasDarwinSigner != (name == "native-darwin") {
			t.Errorf("%s %s Darwin signer presence = %t", projection.Path, name, hasDarwinSigner)
		}
		clientTools := strings.Contains(tools, "inputs.native_clients && ',node'")
		if !clientTools {
			t.Errorf("%s %s lacks the selected native client tool closure", projection.Path, name)
		}
		bootstrap := false
		for _, step := range job.Steps {
			if strings.Contains(step.Run, "./tools/ci native") && !bootstrap {
				t.Errorf("%s %s runs native acceptance before locked dependency preparation", projection.Path, name)
			}
			bootstrap = bootstrap || step.Run == "mise run bootstrap"
		}
	}
}

func TestGitHubWindowsUsesThePortableNativeToolClosure(t *testing.T) {
	projections := canonicalProjections(t)
	var workflow struct {
		Jobs map[string]struct {
			RunsOn yaml.Node         `yaml:"runs-on"`
			Env    map[string]string `yaml:"env"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &workflow); err != nil {
		t.Fatal(err)
	}

	job := workflow.Jobs["native-windows"]
	const runner = "windows-2025"
	if job.RunsOn.Value != runner {
		t.Fatalf("native Windows runner selector = %q, want %q", job.RunsOn.Value, runner)
	}
	tools := job.Env["MISE_ENABLE_TOOLS"]
	for _, required := range []string{"go", "node", "npm", "github:golangci/golangci-lint", "github:goreleaser/goreleaser", "github:anchore/syft", "github:lycheeverse/lychee"} {
		if !strings.Contains(tools, required) {
			t.Errorf("native Windows tool closure lacks %q: %q", required, tools)
		}
	}
	if strings.Contains(tools, "github:indygreg/apple-platform-rs") {
		t.Errorf("Windows tool closure contains the macOS signer: %q", tools)
	}
}

func TestGitLabWindowsDeclaresItsPublicInputClientSupplier(t *testing.T) {
	projections := canonicalProjections(t)
	var jobs map[string]yaml.Node
	if err := yaml.Unmarshal([]byte(projections[0].Content), &jobs); err != nil {
		t.Fatal(err)
	}
	const supplier = "suppliers/windows-codex-0.165.0-claude-2.1.312-native-packages.tar.gz"
	for _, name := range []string{"native-darwin", "native-linux", "native-windows", "native-windows-review"} {
		var job gitLabJob
		node := jobs[name]
		if err := node.Decode(&job); err != nil {
			t.Fatal(err)
		}
		if got := job.Variables["AIGW_NATIVE_CLIENT_SUPPLIER"]; (got == supplier) != strings.HasPrefix(name, "native-windows") || got != "" && got != supplier {
			t.Fatalf("%s public input client supplier = %q", name, got)
		}
		if strings.Contains(strings.ToLower(strings.Join(job.Script, "\n")), "hermes") {
			t.Fatalf("%s supplies an unsupported native journey", name)
		}
	}
}

// Every GitHub native job supplies the official clients that the release
// owner's journey declares for its host, and retires them after any outcome.
func TestGitLabNativeAcceptanceForwardsPeerLocalArtifactAndClientInputs(t *testing.T) {
	projections := canonicalProjections(t)
	var jobs struct {
		Linux   gitLabJob `yaml:"native-linux"`
		Darwin  gitLabJob `yaml:"native-darwin-review"`
		Windows gitLabJob `yaml:"native-windows-review"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &jobs); err != nil {
		t.Fatal(err)
	}
	for name, job := range map[string]gitLabJob{"native-linux": jobs.Linux, "native-darwin-review": jobs.Darwin, "native-windows-review": jobs.Windows} {
		if job.Variables["GLAB_ENABLE_CI_AUTOLOGIN"] != "" {
			t.Fatal("CI login must be scoped to native downloads, not the source or test environment")
		}
		// The native command maps every declared release input and fences
		// diagnostics; its table tests cover each input.
		platform := strings.TrimSuffix(strings.TrimPrefix(name, "native-"), "-review")
		native := "mise exec --locked -- go run ./tools/ci native --platform " + platform + " --from-env"
		if strings.Contains(strings.Join(job.Script, "\n"), "gh release download") || !slices.Contains(job.Script, native) {
			t.Errorf("%s must use its own peer and the existing native controller", name)
		}
	}
}
