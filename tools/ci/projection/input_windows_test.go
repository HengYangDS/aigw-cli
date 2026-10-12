package projection

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"
)

const hermesInstallerFixture = `param([string]$Commit, [string]$HermesHome, [string]$InstallDir, [switch]$NonInteractive, [switch]$SkipBrowser, [switch]$SkipComputerUse, [switch]$Json, [string]$Stage)
$ErrorActionPreference = 'Stop'
if ($env:HERMES_HOME -ne $HermesHome -or $env:HERMES_RUNTIME_DIR -ne (Join-Path $HermesHome 'tools')) { throw 'Hermes PM inherited operator state instead of the admitted home' }
if ($env:UV_PYTHON_INSTALL_DIR -notlike ($env:AIGW_TEST_OPERATION + '*') -or $env:UV_CACHE_DIR -notlike ($env:AIGW_TEST_OPERATION + '*')) { throw 'Hermes Python supply escaped operation ownership' }
if (-not $NonInteractive -or -not $SkipBrowser -or -not $SkipComputerUse) { throw 'Hermes unattended selection was lost' }
if ($Stage -eq 'repository') { [void](New-Item -ItemType Directory -Path $InstallDir -Force) }
if ($Stage -eq 'python-deps') { Write-Output 'Installing dependencies (hash-verified via uv.lock)' }
@{stage=$Stage; ok=$true; skipped=$false} | ConvertTo-Json -Compress
`

func TestWindowsClientSupplyPreservesPortableProductInputOwner(t *testing.T) {
	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		t.Fatalf("Windows native input requires PowerShell: %v", err)
	}
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	data, err := projectionCommand(root, "nativeWindowsClientSupply").Output()
	requireProjectionInput(t, err)
	var script string
	requireProjectionInput(t, yaml.Unmarshal(data, &script))
	workflowData, err := projectionCommand(root, "githubVerify").Output()
	requireProjectionInput(t, err)
	var workflow struct {
		Jobs map[string]struct{ Steps []struct{ Name, Run string } }
	}
	requireProjectionInput(t, yaml.Unmarshal(workflowData, &workflow))
	var githubSupply string
	for _, step := range workflow.Jobs["native-windows"].Steps {
		if step.Name == "Prepare official Windows clients" {
			githubSupply = step.Run
		}
	}
	if githubSupply == "" {
		t.Fatal("GitHub has no official Windows client supply")
	}
	operation := t.TempDir()
	systemRoot := os.Getenv("SystemRoot")
	if systemRoot == "" {
		systemRoot = operation
	}
	project := filepath.Join(operation, "project")
	job := filepath.Join(operation, "aigw-ci-mise-123")
	for _, directory := range []string{project, job} {
		requireProjectionInput(t, os.Mkdir(directory, 0o700))
	}
	pack := func(files map[string][]byte, compressed bool) []byte {
		var archive bytes.Buffer
		var output io.Writer = &archive
		var zipped *gzip.Writer
		if compressed {
			zipped = gzip.NewWriter(&archive)
			output = zipped
		}
		writer := tar.NewWriter(output)
		for name, data := range files {
			requireProjectionInput(t, writer.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(data))}))
			_, err := writer.Write(data)
			requireProjectionInput(t, err)
		}
		requireProjectionInput(t, writer.Close())
		if zipped != nil {
			requireProjectionInput(t, zipped.Close())
		}
		return archive.Bytes()
	}
	// Direct platform archives retain their package name, not npm's optional alias.
	archive := pack(map[string][]byte{
		"candidate/checksums.txt": []byte("test"),
		"baseline/checksums.txt":  []byte("test"),
		"suppliers/windows-codex-0.162.1-claude-2.1.296-native-packages.tar.gz": pack(map[string][]byte{
			"clients/node_modules/@openai/codex-win32-x64/vendor/x86_64-pc-windows-msvc/bin/codex.exe":     []byte("codex fixture"),
			"clients/node_modules/@openai/codex-win32-x64/vendor/x86_64-pc-windows-msvc/codex-path/rg.exe": []byte("resource fixture"),
			"clients/node_modules/@anthropic-ai/claude-code-win32-x64/claude.exe":                          []byte("claude fixture"),
		}, true),
		"suppliers/official-hermes-71d34bad-source.tar": pack(map[string][]byte{
			"scripts/install.ps1": []byte(hermesInstallerFixture),
		}, false),
	}, false)
	input, trust := filepath.Join(operation, "inputs.tar"), filepath.Join(operation, "trust")
	requireProjectionInput(t, os.WriteFile(input, archive, 0o600))
	requireProjectionInput(t, os.WriteFile(trust, []byte("fixture-only public trust"), 0o600))
	bash := filepath.Join(operation, "git", "bin", "bash.exe")
	requireProjectionInput(t, os.MkdirAll(filepath.Dir(bash), 0o700))
	requireProjectionInput(t, os.WriteFile(bash, []byte("Git Bash fixture"), 0o600))
	installer := filepath.Join(operation, "installer.ps1")
	requireProjectionInput(t, os.WriteFile(installer, []byte(hermesInstallerFixture), 0o600))
	launcher := filepath.Join(operation, "launcher.ps1")
	requireProjectionInput(t, os.WriteFile(launcher, []byte("param([string]$Directory)\n$ErrorActionPreference = 'Stop'\nif ($env:HERMES_RUNTIME_DIR -ne (Join-Path $env:HERMES_HOME 'tools')) { throw 'launcher inherited a different PM store' }\n[void](New-Item -ItemType Directory -Path $Directory -Force)\n[IO.File]::WriteAllText((Join-Path $Directory 'hermes.exe'), 'hermes fixture')\n"), 0o600))
	outputs := filepath.Join(operation, "github-env")
	requireProjectionInput(t, os.WriteFile(outputs, nil, 0o600))
	prelude := "$ErrorActionPreference = 'Stop'\n$PSNativeCommandUseErrorActionPreference = $true\n" +
		"if ($env:SystemRoot -cne $env:AIGW_TEST_SYSTEM_ROOT) { throw 'native SystemRoot was replaced by a fixture' }\n" +
		"function Get-Command { [pscustomobject]@{ Source = $(if ($args[0] -eq 'node.exe') { $env:AIGW_TEST_NODE_PATH } else { $env:AIGW_TEST_GIT_PATH }) } }\n" +
		"function Get-FileHash { param([string]$LiteralPath, [string]$Algorithm); if ((Split-Path -Leaf $LiteralPath) -eq 'install.ps1' -or $LiteralPath -eq $env:AIGW_HERMES_INSTALLER) { return [pscustomobject]@{Hash='61824d6b47be29ab793d0e72872cf56cd7a3e99cbe40abeb177a6c9eef52f485'} }; Microsoft.PowerShell.Utility\\Get-FileHash -LiteralPath $LiteralPath -Algorithm $Algorithm }\n" +
		"function git { if ($args -contains 'rev-parse') { return '71d34bad1045610252eda02ee79549436896e784' }; $global:LASTEXITCODE = 0 }\n" +
		"function mise { if ($args[0] -eq 'which') { return $env:AIGW_TEST_NODE_PATH }; " +
		"if ($args[2] -eq 'uv') { if ($args -notcontains '--print-runtime-command' -or $env:HERMES_RUNTIME_DIR -ne (Join-Path $env:HERMES_HOME 'tools')) { throw 'Hermes PM or publication bypassed the admitted home' }; $out = $args[-1]; @($env:AIGW_TEST_PWSH, '-NoProfile', '-File', $env:AIGW_TEST_LAUNCHER, $out) | ConvertTo-Json -Compress; $global:LASTEXITCODE = 0; return }; " +
		"if ($args[2] -eq 'npm') { if ($args[3] -eq 'install') { $prefix = $args[[array]::IndexOf($args, '--prefix') + 1]; foreach ($item in @{ 'node_modules/@openai/codex-win32-x64/vendor/x86_64-pc-windows-msvc/bin/codex.exe'='codex fixture'; 'node_modules/@anthropic-ai/claude-code-win32-x64/claude.exe'='claude fixture' }.GetEnumerator()) { $file=Join-Path $prefix $item.Key; [void](New-Item -ItemType Directory -Path (Split-Path $file) -Force); [IO.File]::WriteAllText($file, $item.Value) } }; $global:LASTEXITCODE=0; return }; " +
		"if ($args[0] -ne 'exec' -or $args[2] -ne 'glab') { throw 'unselected native client tool invoked' }; " +
		"Copy-Item -LiteralPath $env:AIGW_TEST_PUBLIC_ARCHIVE -Destination $args[[array]::IndexOf($args, '--path') + 1]; $global:LASTEXITCODE = 0 }\n"
	environment := append(os.Environ(), "AIGW_NATIVE_INPUT_PACKAGE=fixture", "AIGW_CANDIDATE_SOURCE=0123456789012345678901234567890123456789",
		"AIGW_NATIVE_PLATFORM=windows", "AIGW_NATIVE_DIAGNOSTIC_CLIENT=",
		"AIGW_RELEASE_ALLOWED_SIGNERS_FILE="+trust, "AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE="+trust, "AIGW_RELEASE_ARTIFACT_SIGNER=fixture@example.invalid",
		"AIGW_NATIVE_INPUT_SHA256="+fmt.Sprintf("%x", sha256.Sum256(archive)), "AIGW_TEST_PUBLIC_ARCHIVE="+input,
		"AIGW_TEST_SYSTEM_ROOT="+systemRoot,
		"AIGW_TEST_OPERATION="+operation, "AIGW_TEST_PWSH="+pwsh, "AIGW_TEST_LAUNCHER="+launcher, "AIGW_HERMES_INSTALLER="+installer,
		"RUNNER_TEMP="+operation, "GITHUB_ENV="+outputs, "HERMES_HOME="+filepath.Join(operation, "operator-home"), "HERMES_RUNTIME_DIR="+filepath.Join(operation, "operator-store"),
		"AIGW_TEST_GIT_PATH="+filepath.Join(operation, "git", "cmd", "git.exe"), "AIGW_TEST_NODE_PATH="+filepath.Join(operation, "node", "node.exe"), "SystemRoot="+systemRoot,
		"CI_PROJECT_DIR="+project, "CI_PROJECT_URL=https://gitlab.test/team/aigw", "CI_JOB_ID=123")
	clientAssertion := "foreach ($client in @{ codex = $env:AIGW_ACCEPTANCE_CODEX; claude = $env:AIGW_ACCEPTANCE_CLAUDE; hermes = $env:AIGW_ACCEPTANCE_HERMES }.GetEnumerator()) { " +
		"if (-not (Test-Path -LiteralPath $client.Value -PathType Leaf) -or (Get-Content -LiteralPath $client.Value -Raw) -cne ($client.Key + ' fixture')) { throw 'selected native client identity is unavailable' } }; " +
		"if ($env:AIGW_ACCEPTANCE_HERMES -ne (Join-Path $env:HERMES_HOME 'hermes-agent/.hermes/bin/hermes.exe')) { throw 'Hermes bypassed its published launcher' }; " +
		"if (Test-Path (Join-Path $env:AIGW_TEST_OPERATION 'operator-store')) { throw 'operator PM state was changed' }"
	for _, mode := range []struct{ name, clients, performance, script, assertion string }{
		{"artifact-only", "false", "true", script, "if ($env:AIGW_CANDIDATE_ARTIFACTS -or $env:AIGW_BASELINE_ARTIFACTS -or (Get-ChildItem -LiteralPath $env:AIGW_TEST_JOB_ROOT).Count -ne 0) { throw 'product inputs escaped their portable release owner' }"},
		{"selected-clients", "true", "false", script, clientAssertion},
		{"github-clients", "true", "false", githubSupply, "foreach ($line in Get-Content -LiteralPath $env:GITHUB_ENV) { $key, $value = $line -split '=', 2; if ($key -like 'AIGW_ACCEPTANCE_*') { Set-Item -Path ('Env:' + $key) -Value $value } }; " + clientAssertion},
	} {
		t.Run(mode.name, func(t *testing.T) {
			requireProjectionInput(t, os.RemoveAll(filepath.Join(job, "native-input")))
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, pwsh, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", prelude+mode.script+"\n"+mode.assertion)
			command.Dir = operation
			command.Env = append(slices.Clone(environment), "AIGW_TEST_JOB_ROOT="+job, "AIGW_CANDIDATE_ARTIFACTS=", "AIGW_BASELINE_ARTIFACTS=", "AIGW_NATIVE_CLIENTS="+mode.clients, "AIGW_NATIVE_PERFORMANCE="+mode.performance)
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("native input preparation failed: %v (context: %v)\n%s", err, ctx.Err(), output)
			}
		})
	}
}

func requireProjectionInput(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestWindowsClientSupplyAdmission(t *testing.T) {
	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		t.Fatalf("Windows native input requires PowerShell: %v", err)
	}
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	command := projectionCommand(root, "nativeWindowsClientSupply")
	data, err := command.Output()
	if err != nil {
		t.Fatalf("native public input has no original job prerequisite: %v", err)
	}
	var script string
	if err := yaml.Unmarshal(data, &script); err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	archive := filepath.Join(workspace, "corrupt-public-input.tar")
	if err := os.WriteFile(archive, []byte("corrupt public bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	trust := filepath.Join(workspace, "public-signers")
	if err := os.WriteFile(trust, []byte("fixture-only public trust"), 0o600); err != nil {
		t.Fatal(err)
	}
	prelude := "$ErrorActionPreference = 'Stop'\n$PSNativeCommandUseErrorActionPreference = $true\n" +
		"function mise { if ($args[0] -ne 'exec' -or $args[2] -ne 'glab') { throw 'unexpected native tool' }; " +
		"if ($env:GLAB_CONFIG_DIR -ne (Join-Path (Split-Path $env:CI_PROJECT_DIR) 'aigw-ci-mise-123/glab')) { throw 'download configuration escaped job ownership' }; " +
		"$target = $args[[array]::IndexOf($args, '--path') + 1]; Copy-Item -LiteralPath $env:AIGW_TEST_PUBLIC_ARCHIVE -Destination $target; $global:LASTEXITCODE = 0 }\n" +
		"function tar { if ($env:GLAB_CONFIG_DIR -ne 'original-config' -or $env:GLAB_ENABLE_CI_AUTOLOGIN -ne 'original-login') { throw 'download configuration was not restored' }; throw 'admitted extraction reached' }\n"
	for _, test := range []struct{ name, platform, input, signers, want string }{
		{"tampered", "windows", "admitted public bytes", trust, "Native public input checksum mismatch"},
		{"admitted", "windows", "corrupt public bytes", trust, "admitted extraction reached"},
		{"wrong platform", "linux", "corrupt public bytes", trust, "Native public input requires the Windows platform"},
		{"missing trust", "windows", "corrupt public bytes", "", "Native public input requires configured source and artifact trust"},
	} {
		t.Run(test.name, func(t *testing.T) {
			operation := t.TempDir()
			project := filepath.Join(operation, "project")
			for _, directory := range []string{project, filepath.Join(operation, "aigw-ci-mise-123")} {
				if err := os.Mkdir(directory, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			process := exec.CommandContext(ctx, pwsh, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", prelude+script)
			process.Dir = operation
			process.Env = append(os.Environ(),
				"AIGW_NATIVE_CLIENTS=true", "AIGW_NATIVE_DIAGNOSTIC_CLIENT=",
				"AIGW_NATIVE_INPUT_PACKAGE=fixture", "AIGW_CANDIDATE_SOURCE=0123456789012345678901234567890123456789",
				"AIGW_RELEASE_ALLOWED_SIGNERS=", "AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS=",
				"AIGW_NATIVE_PLATFORM="+test.platform, "AIGW_RELEASE_ALLOWED_SIGNERS_FILE="+test.signers,
				"AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE="+test.signers, "AIGW_RELEASE_ARTIFACT_SIGNER=fixture@example.invalid",
				"AIGW_NATIVE_INPUT_SHA256="+fmt.Sprintf("%x", sha256.Sum256([]byte(test.input))),
				"GLAB_CONFIG_DIR=original-config", "GLAB_ENABLE_CI_AUTOLOGIN=original-login",
				"AIGW_TEST_PUBLIC_ARCHIVE="+archive, "CI_PROJECT_DIR="+project, "CI_PROJECT_URL=https://gitlab.test/team/aigw", "CI_JOB_ID=123")
			output, err := process.CombinedOutput()
			if err == nil || !bytes.Contains(output, []byte(test.want)) {
				t.Fatalf("native input admission changed: %v\n%s", err, output)
			}
		})
	}
}

func TestWindowsPublicInputToolsPreserveSourceScope(t *testing.T) {
	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		t.Fatalf("Windows native input requires PowerShell: %v", err)
	}
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	data, err := projectionCommand(root, `(#NativeGitLabJob & {_platform: "windows", tags: [], rules: []})._selectTools`).Output()
	if err != nil {
		t.Fatal(err)
	}
	var script string
	if err := yaml.Unmarshal(data, &script); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ name, pack, full, refresh, clients, diagnostic, performance, want string }{
		{"source", "", "false", "false", "false", "", "false", "source-tools"},
		{"public package", "fixture", "false", "false", "false", "", "false", "go,gh,glab,github:goreleaser/goreleaser"},
		{"performance", "fixture", "false", "false", "false", "", "true", "go,gh,glab,github:goreleaser/goreleaser,github:sharkdp/hyperfine"},
		{"clients", "fixture", "false", "false", "true", "", "false", "go,gh,glab,github:goreleaser/goreleaser,node,uv"},
		{"diagnostic", "fixture", "false", "false", "false", "hermes", "false", "go,gh,glab,github:goreleaser/goreleaser,node,uv"},
		{"full quality", "fixture", "true", "false", "false", "", "false", "source-tools"},
		{"lock refresh", "fixture", "false", "true", "false", "", "false", "source-tools"},
	} {
		t.Run(test.name, func(t *testing.T) {
			command := exec.CommandContext(t.Context(), pwsh, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", script+"\nWrite-Output $env:MISE_ENABLE_TOOLS")
			command.Env = append(os.Environ(), "MISE_ENABLE_TOOLS=source-tools", "AIGW_NATIVE_INPUT_PACKAGE="+test.pack,
				"AIGW_CANDIDATE_ARTIFACTS=", "AIGW_CANDIDATE_TAG=", "AIGW_FULL_NATIVE_QUALITY="+test.full, "AIGW_REFRESH_LOCKS="+test.refresh,
				"AIGW_NATIVE_CLIENTS="+test.clients, "AIGW_NATIVE_DIAGNOSTIC_CLIENT="+test.diagnostic, "AIGW_NATIVE_PERFORMANCE="+test.performance)
			output, err := command.CombinedOutput()
			if err != nil || strings.TrimSpace(string(output)) != test.want {
				t.Fatalf("Windows public input tool scope: %v, %s; want %s", err, output, test.want)
			}
		})
	}
}

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
			"${{ inputs.windows_clients && inputs.diagnostic_client == '' }}", "false",
			"${{ inputs.performance_attribution }}", "false",
			"${{ inputs.input_release != '' }}", fmt.Sprint(release != ""),
		).Replace(suffix)
		script := "& '" + strings.ReplaceAll(executable, "'", "''") + "' '-test.run=^TestNativePowerShellAcceptancePreservesDeclaredEmptyTag$' -- " + arguments
		for _, client := range []string{"", "hermes"} {
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
