package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunRejectsMissingAndUnknownCommands(t *testing.T) {
	for name, args := range map[string][]string{
		"missing": nil,
		"unknown": {"unknown"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := runContract(args); err == nil {
				t.Fatal("run unexpectedly succeeded")
			}
		})
	}
}

func TestRunUsesCurrentDirectoryByDefault(t *testing.T) {
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := repository(t, map[string]string{
		".gitlab-ci.yml": "${AIGW_GOPROXY:-https://goproxy.cn|https://proxy.golang.org|direct}",
	})
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	// Register restoration after TempDir's cleanup so Windows can remove the
	// fixture directory without the process still holding it as cwd.
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
	if err := runContract([]string{"proxy-policy"}); err != nil {
		t.Fatalf("default root failed: %v", err)
	}
}

func TestRunDispatchesWorkflowContracts(t *testing.T) {
	root := fixtureRepository(t)
	for _, command := range []string{"toolchain", "github-verify", "github-release"} {
		if err := runContract([]string{command, root}); err != nil {
			t.Fatalf("%s failed: %v", command, err)
		}
	}
}

func TestGitHubWorkflowContractRequiresDefaultBranchConfiguration(t *testing.T) {
	for _, command := range []string{"github-verify", "github-release"} {
		t.Run(command, func(t *testing.T) {
			files := fixtureFiles()
			workflow := ".github/workflows/verify.yml"
			if command == "github-release" {
				workflow = ".github/workflows/release.yml"
			}
			files[workflow] = strings.Replace(
				files[workflow],
				"env:\n  GIT_CONFIG_COUNT: \"1\"\n  GIT_CONFIG_KEY_0: init.defaultBranch\n  GIT_CONFIG_VALUE_0: main\n",
				"",
				1,
			)
			err := runContract([]string{command, repository(t, files)})
			if err == nil || !strings.Contains(err.Error(), "default branch") {
				t.Fatalf("missing default-branch configuration accepted: %v", err)
			}
		})
	}
}

func TestGitHubWorkflowContractRequiresMiseBeforeEveryOwnedCommand(t *testing.T) {
	files := fixtureFiles()
	workflow := files[".github/workflows/verify.yml"]
	workflow = strings.Replace(
		workflow,
		"      - uses: jdx/mise-action@1234567890abcdef1234567890abcdef12345678\n        with:\n          install: true\n          cache: false\n      - name: Run source and policy gates",
		"      - name: Refresh annotated release tags\n        run: mise exec --locked -- go run ./tools/ci fetch-tags\n      - uses: jdx/mise-action@1234567890abcdef1234567890abcdef12345678\n        with:\n          install: true\n          cache: false\n      - name: Run source and policy gates",
		1,
	)
	files[".github/workflows/verify.yml"] = workflow

	err := githubWorkflowContract(repository(t, files), false)
	if err == nil || !strings.Contains(err.Error(), "before mise bootstrap") {
		t.Fatalf("pre-bootstrap mise command accepted: %v", err)
	}
}

func TestMiseCommandRequiresLockedExecution(t *testing.T) {
	for name, command := range map[string]string{
		"plain":              "mise exec -- go run ./tools/ci source",
		"option before lock": "mise exec --quiet -- go run ./tools/ci source",
		"short alias":        "mise x -- go test ./...",
	} {
		t.Run(name, func(t *testing.T) {
			err := validateMiseCommand(command)
			if err == nil || !strings.Contains(err.Error(), "locked mise") {
				t.Fatalf("unlocked mise command accepted: %v", err)
			}
		})
	}
	for _, command := range []string{
		"mise exec --locked -- go run ./tools/ci source",
		"mise exec --quiet --locked -- go test ./...",
		"go test ./...",
	} {
		if err := validateMiseCommand(command); err != nil {
			t.Fatalf("valid command %q rejected: %v", command, err)
		}
	}
}

func TestPipelineContractRequiresImmutableOfficialGitLabMiseImage(t *testing.T) {
	for name, image := range map[string]string{
		"missing":   "",
		"mutable":   "ghcr.io/jdx/mise:2026.8.3",
		"unowned":   "example.invalid/mise@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"malformed": "ghcr.io/jdx/mise@sha256:not-a-digest",
	} {
		t.Run(name, func(t *testing.T) {
			files := fixtureFiles()
			files[".gitlab-ci.yml"] = strings.Replace(
				files[".gitlab-ci.yml"],
				"  image: ghcr.io/jdx/mise@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef\n",
				func() string {
					if image == "" {
						return ""
					}
					return "  image: " + image + "\n"
				}(),
				1,
			)
			err := pipelineContract(repository(t, files))
			if err == nil || !strings.Contains(err.Error(), "GitLab mise image") {
				t.Fatalf("invalid GitLab mise image accepted: %v", err)
			}
		})
	}
}

func TestProxyPolicy(t *testing.T) {
	const fallback = "${AIGW_GOPROXY:-https://goproxy.cn|https://proxy.golang.org|direct}"
	for name, content := range map[string]string{
		"valid":          fallback,
		"missing_chain":  "GOPROXY: direct",
		"obsolete_chain": fallback + "\n${AIGW_GOPROXY:-https://goproxy.cn,direct}",
	} {
		t.Run(name, func(t *testing.T) {
			root := repository(t, map[string]string{".gitlab-ci.yml": content})
			err := runContract([]string{"proxy-policy", root})
			if name == "valid" && err != nil {
				t.Fatalf("proxy policy failed: %v", err)
			}
			if name != "valid" && err == nil {
				t.Fatal("proxy policy unexpectedly succeeded")
			}
		})
	}

	if err := proxyPolicy(t.TempDir()); err == nil || !strings.Contains(err.Error(), "read .gitlab-ci.yml") {
		t.Fatalf("missing file error = %v", err)
	}
}

func TestActiveCommandsRejectInertAndNonblockingGates(t *testing.T) {
	required := []string{"mise exec --locked -- go run ./tools/ci source"}
	valid := []map[string]any{{"name": "Run source and policy gates", "run": required[0]}}
	if err := activeCommands(valid, required); err != nil {
		t.Fatalf("valid active commands failed: %v", err)
	}
	for name, steps := range map[string][]map[string]any{
		"missing":     {{"name": "other", "run": strings.Join(required, "\n")}},
		"inert_env":   {{"name": "Run source and policy gates", "env": map[string]any{"manifest": strings.Join(required, "\n")}, "run": "other"}},
		"conditional": {{"name": "Run source and policy gates", "if": false, "run": strings.Join(required, "\n")}},
		"nonblocking": {{"name": "Run source and policy gates", "continue-on-error": true, "run": strings.Join(required, "\n")}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := activeCommands(steps, required); err == nil {
				t.Fatal("invalid gate projection unexpectedly succeeded")
			}
		})
	}
}

func TestContractsRejectMultilineRunBlocksAndExplicitShells(t *testing.T) {
	for name, mutate := range map[string]func(map[string]string){
		"multiline run": func(files map[string]string) {
			files[".github/workflows/verify.yml"] = strings.Replace(files[".github/workflows/verify.yml"], "run: mise exec --locked -- go run ./tools/ci source", "run: |\n          mise exec --locked -- go run ./tools/ci source\n          go vet ./...", 1)
		},
		"explicit shell": func(files map[string]string) {
			files[".github/workflows/verify.yml"] = strings.Replace(files[".github/workflows/verify.yml"], "run: mise exec --locked -- go run ./tools/ci source", "shell: pwsh\n        run: mise exec --locked -- go run ./tools/ci source", 1)
		},
	} {
		t.Run(name, func(t *testing.T) {
			files := fixtureFiles()
			mutate(files)
			if err := pipelineContract(repository(t, files)); err == nil {
				t.Fatal("shell-owned workflow projection accepted")
			}
		})
	}
}

func TestContractsRejectGitLabShellOrchestration(t *testing.T) {
	for name, mutation := range map[string]string{
		"multiline script": "verify:\n  script:\n    - |\n      go test ./...\n      go vet ./...\n",
		"before script":    "before_script:\n  - go env GOPROXY\nverify:\n  script:\n    - mise exec --locked -- go run ./tools/ci source\n",
	} {
		t.Run(name, func(t *testing.T) {
			files := fixtureFiles()
			files[".gitlab-ci.yml"] = mutation
			if err := validateGitLabCommandProjection(repository(t, files)); err == nil {
				t.Fatal("GitLab shell orchestration accepted")
			}
		})
	}
}

func TestGitLabProjectionRejectsMalformedAndNestedShellOwnership(t *testing.T) {
	for name, pipeline := range map[string]string{
		"malformed yaml":        "jobs: [",
		"default before script": "default:\n  before_script:\n    - go env GOPROXY\n",
		"job before script":     "verify:\n  before_script:\n    - go env GOPROXY\n  script:\n    - mise exec --locked -- go run ./tools/ci source\n",
		"scalar script":         "verify:\n  script: mise exec --locked -- go run ./tools/ci source\n",
	} {
		t.Run(name, func(t *testing.T) {
			root := repository(t, map[string]string{".gitlab-ci.yml": pipeline})
			if err := validateGitLabCommandProjection(root); err == nil {
				t.Fatal("invalid GitLab projection accepted")
			}
		})
	}
	if err := validateGitLabCommandProjection(t.TempDir()); err == nil {
		t.Fatal("missing GitLab projection accepted")
	}
}

func TestGitHubContractRejectsStepAndWorkflowOwnershipDrift(t *testing.T) {
	for name, mutate := range map[string]func(map[string]string){
		"source gate shell": func(files map[string]string) {
			files[".github/workflows/verify.yml"] = strings.Replace(files[".github/workflows/verify.yml"], "        run: mise exec --locked -- go run ./tools/ci source", "        shell: bash\n        run: mise exec --locked -- go run ./tools/ci source", 1)
		},
		"source gate multiline": func(files map[string]string) {
			files[".github/workflows/verify.yml"] = strings.Replace(files[".github/workflows/verify.yml"], "run: mise exec --locked -- go run ./tools/ci source", "run: 'mise exec --locked -- go run ./tools/ci source\\ngo vet ./...'", 1)
		},
		"missing runner": func(files map[string]string) {
			files[".github/workflows/verify.yml"] = strings.Replace(files[".github/workflows/verify.yml"], "${{ fromJSON(vars.AIGW_VERIFY_RUNNER) }}", "ubuntu-latest", 1)
		},
		"floating master": func(files map[string]string) {
			files[".github/workflows/verify.yml"] += "\n# @master\n"
		},
		"self hosted literal": func(files map[string]string) {
			files[".github/workflows/verify.yml"] += "\n# runs-on: [self-hosted\n"
		},
	} {
		t.Run(name, func(t *testing.T) {
			files := fixtureFiles()
			mutate(files)
			if err := githubWorkflowContract(repository(t, files), false); err == nil {
				t.Fatal("invalid GitHub workflow accepted")
			}
		})
	}
}

func TestPipelineContract(t *testing.T) {
	root := fixtureRepository(t)
	if err := runContract([]string{"pipeline", root}); err != nil {
		t.Fatalf("valid pipeline failed: %v", err)
	}

	files := fixtureFiles()
	delete(files, "go.mod")
	root = repository(t, files)
	if err := pipelineContract(root); err == nil || !strings.Contains(err.Error(), "read go.mod") {
		t.Fatalf("missing projection error = %v", err)
	}

	files["go.mod"] = "module aigw-cli\ngo 1.26.5\n"
	files[".config/ci/verify-gates.toml"] = "[common]"
	root = repository(t, files)
	if err := pipelineContract(root); err == nil {
		t.Fatalf("incomplete SSOT error = %v", err)
	}

	if err := pipelineContract(t.TempDir()); err == nil || !strings.Contains(err.Error(), "verify-gates.toml") {
		t.Fatalf("missing SSOT error = %v", err)
	}
}

func TestContractsRejectProjectionDrift(t *testing.T) {
	for name, mutate := range map[string]func(map[string]string){
		"gitlab_command": func(files map[string]string) {
			files[".gitlab-ci.yml"] = strings.Replace(files[".gitlab-ci.yml"], "    - mise exec --locked -- go run ./tools/ci source\n", "", 1)
		},
		"github_inert": func(files map[string]string) {
			files[".github/workflows/verify.yml"] = strings.Replace(files[".github/workflows/verify.yml"], "        run: mise exec --locked -- go run ./tools/ci source\n", "", 1)
		},
		"github_release_permission": func(files map[string]string) {
			files[".github/workflows/verify.yml"] = strings.Replace(files[".github/workflows/verify.yml"], "contents: read", "contents: write", 1)
		},
		"github_runner": func(files map[string]string) {
			files[".github/workflows/release.yml"] = strings.Replace(files[".github/workflows/release.yml"], "${{ fromJSON(vars.AIGW_RELEASE_RUNNER) }}", "ubuntu-latest", 1)
		},
		"github_permission": func(files map[string]string) {
			files[".github/workflows/release.yml"] = strings.Replace(files[".github/workflows/release.yml"], "contents: write", "contents: read", 1)
		},
		"github_missing_command": func(files map[string]string) {
			files[".github/workflows/release.yml"] = strings.Replace(files[".github/workflows/release.yml"], "      - run: mise exec --locked -- go run ./tools/release publish-github dist\n", "", 1)
		},
		"github_parse": func(files map[string]string) {
			files[".github/workflows/verify.yml"] = "jobs: ["
		},
		"github_floating_action": func(files map[string]string) {
			files[".github/workflows/verify.yml"] += "\n# @main\n"
		},
		"mutable_action": func(files map[string]string) {
			files[".config/ci/verify-gates.toml"] = strings.Replace(files[".config/ci/verify-gates.toml"], "actions/checkout@0123456789abcdef0123456789abcdef01234567", "actions/checkout@main", 1)
		},
		"mutable_mise_action": func(files map[string]string) {
			files[".config/ci/verify-gates.toml"] = strings.Replace(files[".config/ci/verify-gates.toml"], "jdx/mise-action@1234567890abcdef1234567890abcdef12345678", "jdx/mise-action@main", 1)
		},
		"missing_mise_lock": func(files map[string]string) {
			delete(files, "mise.lock")
		},
		"missing_mise_paths": func(files map[string]string) {
			files[".config/ci/verify-gates.toml"] = strings.Replace(files[".config/ci/verify-gates.toml"], "mise_config = \"mise.toml\"\nmise_lock = \"mise.lock\"\n", "", 1)
		},
		"empty_mise_tools": func(files map[string]string) {
			files["mise.toml"] = "min_version = \"2026.8.3\"\n"
		},
		"missing_locked_go": func(files map[string]string) {
			files["mise.lock"] = "[[tools.other]]\nversion = \"1\"\n"
		},
		"github_bypasses_mise": func(files map[string]string) {
			files[".github/workflows/verify.yml"] = strings.Replace(files[".github/workflows/verify.yml"], "mise exec --locked -- go run ./tools/ci source", "go run ./tools/ci source", 1)
		},
		"github_installs_tools_ad_hoc": func(files map[string]string) {
			files[".github/workflows/release.yml"] += "\n# brew install goreleaser syft\n"
		},
		"gitlab_bypasses_mise": func(files map[string]string) {
			files[".gitlab-ci.yml"] = strings.Replace(files[".gitlab-ci.yml"], "mise exec --locked -- go run ./tools/ci source", "go run ./tools/ci source", 1)
		},
	} {
		t.Run(name, func(t *testing.T) {
			files := fixtureFiles()
			mutate(files)
			if err := pipelineContract(repository(t, files)); err == nil {
				t.Fatal("projection drift accepted")
			}
		})
	}
}

func TestContractsReportUnreadableAndMalformedOwnedInputs(t *testing.T) {
	files := fixtureFiles()
	files[".config/ci/verify-gates.toml"] = "invalid = ["
	if _, err := loadGates(repository(t, files)); err == nil {
		t.Fatal("malformed gate configuration accepted")
	}

	files = fixtureFiles()
	delete(files, ".github/workflows/verify.yml")
	if err := githubWorkflowContract(repository(t, files), false); err == nil {
		t.Fatal("missing verify workflow accepted")
	}

	files = fixtureFiles()
	files[".github/workflows/release.yml"] = strings.Replace(files[".github/workflows/release.yml"], "      - run: mise exec --locked -- go run ./tools/release publish-github dist", "      - shell: bash\n        run: mise exec --locked -- go run ./tools/release publish-github dist", 1)
	if err := githubWorkflowContract(repository(t, files), true); err == nil {
		t.Fatal("release workflow shell accepted")
	}

	files = fixtureFiles()
	files[".github/workflows/release.yml"] = strings.Replace(files[".github/workflows/release.yml"], "run: mise exec --locked -- go run ./tools/release publish-github dist", "run: |\n          mise exec --locked -- go run ./tools/release publish-github dist\n          go vet ./...", 1)
	if err := githubWorkflowContract(repository(t, files), true); err == nil {
		t.Fatal("release workflow multiline command accepted")
	}

	files = fixtureFiles()
	delete(files, ".gitlab-ci.yml")
	if err := pipelineContract(repository(t, files)); err == nil {
		t.Fatal("missing GitLab pipeline accepted")
	}

	files = fixtureFiles()
	files[".config/ci/verify-gates.toml"] = strings.Replace(files[".config/ci/verify-gates.toml"], "[gitlab.commands]\nrequired = []", "[gitlab.commands]\nrequired = [\"missing-command\"]", 1)
	if err := pipelineContract(repository(t, files)); err == nil {
		t.Fatal("missing GitLab verification command accepted")
	}
}

func TestContractsRejectEveryPolicyBoundary(t *testing.T) {
	mutations := []func(map[string]string){
		func(files map[string]string) { files[".gitlab-ci.yml"] += "\nallow_failure: true\n" },
		func(files map[string]string) { files[".gitlab-ci.yml"] += "\nmirror-github:\n" },
		func(files map[string]string) { files[".gitlab-ci.yml"] += "\nwindows-native-acceptance:\n" },
		func(files map[string]string) { files[".gitlab-ci.yml"] += "\nmacos-native-acceptance:\n" },
		func(files map[string]string) {
			files[".gitlab-ci.yml"] = strings.Replace(files[".gitlab-ci.yml"], "when: never", "when: always", 1)
		},
		func(files map[string]string) {
			files[".gitlab-ci.yml"] = strings.Replace(files[".gitlab-ci.yml"], "    - if: '$CI_MERGE_REQUEST_SOURCE_BRANCH_NAME =~ /^release\\//'\n      when: never\n", "", 1)
		},
		func(files map[string]string) {
			files[".config/ci/verify-gates.toml"] = strings.Replace(files[".config/ci/verify-gates.toml"], "[gitlab.package]\nrequired = []", "[gitlab.package]\nrequired = [\"package-required\"]", 1)
		},
		func(files map[string]string) {
			files[".config/ci/verify-gates.toml"] = strings.Replace(files[".config/ci/verify-gates.toml"], "[gitlab.release]\nrequired = []", "[gitlab.release]\nrequired = [\"release-required\"]", 1)
		},
		func(files map[string]string) { files[".github/workflows/verify.yml"] += "\n# AIGW_GOPROXY\n" },
		func(files map[string]string) { files[".github/workflows/release.yml"] += "\n# gitlab-ci\n" },
		func(files map[string]string) {
			files[".github/workflows/release.yml"] += "\ngo run ./tools/release publish-gitlab dist\n"
		},
		func(files map[string]string) {
			files[".github/workflows/verify.yml"] = strings.Replace(files[".github/workflows/verify.yml"], "  verify:\n", "  verify:\n    if: false\n", 1)
		},
		func(files map[string]string) {
			files[".github/workflows/verify.yml"] = strings.Replace(files[".github/workflows/verify.yml"], "      - name: Run source and policy gates\n", "      - name: Run source and policy gates\n        continue-on-error: true\n", 1)
		},
		func(files map[string]string) { delete(files, ".github/workflows/verify.yml") },
		func(files map[string]string) { delete(files, ".github/workflows/release.yml") },
	}
	for index, mutate := range mutations {
		files := fixtureFiles()
		mutate(files)
		if err := pipelineContract(repository(t, files)); err == nil {
			t.Fatalf("policy drift %d accepted", index)
		}
	}
}

func fixtureRepository(t *testing.T) string { return repository(t, fixtureFiles()) }

func fixtureFiles() map[string]string {
	const checkout = "actions/checkout@0123456789abcdef0123456789abcdef01234567"
	const miseAction = "jdx/mise-action@1234567890abcdef1234567890abcdef12345678"
	return map[string]string{
		".config/ci/verify-gates.toml": `[toolchain]
go_mod = "go.mod"
mise_config = "mise.toml"
mise_lock = "mise.lock"
[toolchain.github_actions]
checkout = "` + checkout + `"
mise = "` + miseAction + `"
[common]
commands = ["mise exec --locked -- go run ./tools/ci source"]
[common.active_script_commands]
commands = ["mise exec --locked -- go run ./tools/ci source"]
[gitlab]
bootstrap_image = "ghcr.io/jdx/mise@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
runner_tag_variable = "RUNNER"
git_depth = "0"
goproxy_fallback = "proxy|direct"
suppress_untagged_release_branch = true
suppress_release_branch_merge_request = true
[gitlab.commands]
required = []
[gitlab.package]
required = []
[gitlab.release]
required = []
[github.verify]
runner = "${{ fromJSON(vars.AIGW_VERIFY_RUNNER) }}"
permissions = "contents: read"
[github.verify.commands]
required = []
[github.verify.active_script_commands]
commands = ["mise exec --locked -- go run ./tools/ci source"]
[github.release]
runner = "${{ fromJSON(vars.AIGW_RELEASE_RUNNER) }}"
permissions = "contents: write"
[github.release.commands]
required = ["mise exec --locked -- go run ./tools/release publish-github dist"]
[github.release.forbid]
tokens = []
[native.linux]
required = []
[native.windows]
required = []
[native.darwin]
required = []
`,
		".gitlab-ci.yml": `workflow:
  rules:
    - if: '$CI_COMMIT_BRANCH =~ /^release\//'
      when: never
    - if: '$CI_MERGE_REQUEST_SOURCE_BRANCH_NAME =~ /^release\//'
      when: never
default:
  image: ghcr.io/jdx/mise@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
verify:
  script:
    - mise exec --locked -- go run ./tools/ci source
package:
  script: [true]
release:
  script: [true]
`,
		".github/workflows/verify.yml": `name: Verify
permissions:
  contents: read
env:
  GIT_CONFIG_COUNT: "1"
  GIT_CONFIG_KEY_0: init.defaultBranch
  GIT_CONFIG_VALUE_0: main
jobs:
  verify:
    runs-on: ${{ fromJSON(vars.AIGW_VERIFY_RUNNER) }}
    steps:
      - uses: ` + checkout + `
      - uses: ` + miseAction + `
        with:
          install: true
          cache: false
      - name: Run source and policy gates
        run: mise exec --locked -- go run ./tools/ci source
`,
		".github/workflows/release.yml": `name: Release
permissions:
  contents: write
env:
  GIT_CONFIG_COUNT: "1"
  GIT_CONFIG_KEY_0: init.defaultBranch
  GIT_CONFIG_VALUE_0: main
jobs:
  package:
    runs-on: ${{ fromJSON(vars.AIGW_RELEASE_RUNNER) }}
    steps:
      - uses: ` + checkout + `
      - uses: ` + miseAction + `
        with:
          install: true
          cache: false
      - run: mise exec --locked -- go run ./tools/release publish-github dist
`,
		"go.mod":    "module aigw-cli\ngo 1.26.5\n",
		"mise.toml": "min_version = \"2026.8.3\"\n[settings]\nlocked = true\n[tools]\ngo = \"1.26.5\"\n",
		"mise.lock": "[[tools.go]]\nversion = \"1.26.5\"\nbackend = \"core:go\"\n",
	}
}

func repository(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for relative, content := range files {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create parent for %s: %v", relative, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", relative, err)
		}
	}
	return root
}
