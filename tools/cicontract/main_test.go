package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecuteReturnsProcessStatus(t *testing.T) {
	stderr, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stderr.Close() })

	if status := execute([]string{"unknown"}, stderr); status != 1 {
		t.Fatalf("failure status = %d", status)
	}
	if status := execute([]string{"proxy-policy", repository(t, map[string]string{
		".gitlab-ci.yml": "${AIGW_GOPROXY:-https://goproxy.cn|https://proxy.golang.org|direct}",
	})}, stderr); status != 0 {
		t.Fatalf("success status = %d", status)
	}
	if _, err := stderr.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	message, err := os.ReadFile(stderr.Name())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(message), "unknown cicontract command") {
		t.Fatalf("stderr = %q", message)
	}
}

func TestRunRejectsMissingAndUnknownCommands(t *testing.T) {
	for name, args := range map[string][]string{
		"missing": nil,
		"unknown": {"unknown"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := run(args); err == nil {
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
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
	root := repository(t, map[string]string{
		".gitlab-ci.yml": "${AIGW_GOPROXY:-https://goproxy.cn|https://proxy.golang.org|direct}",
	})
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"proxy-policy"}); err != nil {
		t.Fatalf("default root failed: %v", err)
	}
}

func TestRunDispatchesWorkflowContracts(t *testing.T) {
	root := fixtureRepository(t)
	for _, command := range []string{"github-verify", "github-release"} {
		if err := run([]string{command, root}); err != nil {
			t.Fatalf("%s failed: %v", command, err)
		}
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
			err := run([]string{"proxy-policy", root})
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
	required := []string{"go test ./...", "go vet ./..."}
	valid := []map[string]any{{"name": "Run source and policy gates", "run": strings.Join(required, "\n")}}
	if err := activeCommands(valid, required); err != nil {
		t.Fatalf("valid active commands failed: %v", err)
	}
	for name, steps := range map[string][]map[string]any{
		"missing":     {{"name": "other", "run": strings.Join(required, "\n")}},
		"inert_env":   {{"name": "Run source and policy gates", "env": map[string]any{"manifest": strings.Join(required, "\n")}, "run": required[0]}},
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

func TestPipelineContract(t *testing.T) {
	root := fixtureRepository(t)
	if err := run([]string{"pipeline", root}); err != nil {
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
			files[".gitlab-ci.yml"] = strings.Replace(files[".gitlab-ci.yml"], "    - go test ./...\n", "", 1)
		},
		"github_inert": func(files map[string]string) {
			files[".github/workflows/verify.yml"] = strings.Replace(files[".github/workflows/verify.yml"], "          go test ./...\n", "", 1)
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
			files[".github/workflows/release.yml"] = strings.Replace(files[".github/workflows/release.yml"], "      - run: publish-github-release.sh\n", "", 1)
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
		"mutable_setup_action": func(files map[string]string) {
			files[".config/ci/verify-gates.toml"] = strings.Replace(files[".config/ci/verify-gates.toml"], "actions/setup-go@abcdef0123456789abcdef0123456789abcdef01", "actions/setup-go@main", 1)
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
			files[".github/workflows/release.yml"] += "\nsh scripts/release/publish/publish-gitlab-release.sh\n"
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
	const setup = "actions/setup-go@abcdef0123456789abcdef0123456789abcdef01"
	return map[string]string{
		".config/ci/verify-gates.toml": `[toolchain]
go_mod = "go.mod"
github_setup_go_cache = false
[toolchain.github_actions]
checkout = "` + checkout + `"
setup_go = "` + setup + `"
[common]
commands = ["go test ./..."]
[common.active_script_commands]
commands = ["go test ./..."]
[gitlab]
runner_tag_variable = "RUNNER"
git_depth = "0"
goproxy_fallback = "proxy|direct"
prepare_cache_script = "prepare.sh"
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
commands = ["go test ./..."]
[github.release]
runner = "${{ fromJSON(vars.AIGW_RELEASE_RUNNER) }}"
permissions = "contents: write"
[github.release.commands]
required = ["publish-github-release.sh"]
[github.release.forbid]
tokens = []
[native.linux]
required = []
[native.windows]
required = []
[native.macos]
staging_commands = []
`,
		".gitlab-ci.yml": `workflow:
  rules:
    - if: '$CI_COMMIT_BRANCH =~ /^release\//'
      when: never
    - if: '$CI_MERGE_REQUEST_SOURCE_BRANCH_NAME =~ /^release\//'
      when: never
verify:
  script:
    - go test ./...
package:
  script: [true]
release:
  script: [true]
`,
		".github/workflows/verify.yml": `name: Verify
permissions:
  contents: read
jobs:
  verify:
    runs-on: ${{ fromJSON(vars.AIGW_VERIFY_RUNNER) }}
    steps:
      - uses: ` + checkout + `
      - uses: ` + setup + `
        with:
          go-version-file: go.mod
          cache: false
      - name: Run source and policy gates
        run: |
          go test ./...
`,
		".github/workflows/release.yml": `name: Release
permissions:
  contents: write
jobs:
  package:
    runs-on: ${{ fromJSON(vars.AIGW_RELEASE_RUNNER) }}
    steps:
      - uses: ` + checkout + `
      - uses: ` + setup + `
        with:
          go-version-file: go.mod
          cache: false
      - run: publish-github-release.sh
`,
		"go.mod": "module aigw-cli\ngo 1.26.5\n",
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
