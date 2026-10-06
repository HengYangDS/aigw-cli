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
	"runtime"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"
)

func TestNativePublicInputPreparesSelectedArtifactAndClientRoots(t *testing.T) {
	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		if runtime.GOOS == "windows" {
			t.Fatal(err)
		}
		t.Skip("PowerShell native execution is optional outside Windows")
	}
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	data, err := projectionCommand(root, "nativePublicInputWindows").Output()
	if err != nil {
		t.Fatal(err)
	}
	var script string
	if err := yaml.Unmarshal(data, &script); err != nil {
		t.Fatal(err)
	}
	operation := t.TempDir()
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
		"suppliers/windows-codex-0.160.1-claude-2.1.291-native-packages.tar.gz": pack(map[string][]byte{
			"clients/node_modules/@openai/codex/vendor/x86_64-pc-windows-msvc/bin/codex.exe":     []byte("codex fixture"),
			"clients/node_modules/@openai/codex/vendor/x86_64-pc-windows-msvc/codex-path/rg.exe": []byte("resource fixture"),
			"clients/node_modules/@anthropic-ai/claude-code-win32-x64/claude.exe":                []byte("claude fixture"),
		}, true),
		"suppliers/official-hermes-f97608f1-source.tar": pack(map[string][]byte{
			".venv/Scripts/hermes.exe": []byte("hermes fixture"),
		}, false),
	}, false)
	input, trust := filepath.Join(operation, "inputs.tar"), filepath.Join(operation, "trust")
	requireProjectionInput(t, os.WriteFile(input, archive, 0o600))
	requireProjectionInput(t, os.WriteFile(trust, []byte("fixture-only public trust"), 0o600))
	bash := filepath.Join(operation, "git", "bin", "bash.exe")
	requireProjectionInput(t, os.MkdirAll(filepath.Dir(bash), 0o700))
	requireProjectionInput(t, os.WriteFile(bash, []byte("Git Bash fixture"), 0o600))
	prelude := "$ErrorActionPreference = 'Stop'\n$PSNativeCommandUseErrorActionPreference = $true\n" +
		"function Get-Command { [pscustomobject]@{ Source = $env:AIGW_TEST_GIT_PATH } }\n" +
		"function mise { if ($args[0] -eq 'which') { return $env:AIGW_TEST_NODE_PATH }; " +
		"if ($args[2] -eq 'uv' -and $env:AIGW_NATIVE_CLIENTS -eq 'true') { $global:LASTEXITCODE = 0; return }; " +
		"if ($args[0] -ne 'exec' -or $args[2] -ne 'glab') { throw 'unselected native client tool invoked' }; " +
		"Copy-Item -LiteralPath $env:AIGW_TEST_PUBLIC_ARCHIVE -Destination $args[[array]::IndexOf($args, '--path') + 1]; $global:LASTEXITCODE = 0 }\n"
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, pwsh, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", prelude+script+
		"\nif ($env:AIGW_CANDIDATE_ARTIFACTS -ne (Join-Path $fixture 'candidate') -or $env:AIGW_BASELINE_ARTIFACTS -ne (Join-Path $fixture 'baseline')) { throw 'artifact identities were not prepared' }")
	command.Dir = operation
	environment := append(os.Environ(), "AIGW_NATIVE_INPUT_PACKAGE=fixture", "AIGW_CANDIDATE_SOURCE=0123456789012345678901234567890123456789",
		"AIGW_NATIVE_PLATFORM=windows", "AIGW_NATIVE_CLIENTS=false", "AIGW_NATIVE_DIAGNOSTIC_CLIENT=", "AIGW_NATIVE_PERFORMANCE=true",
		"AIGW_RELEASE_ALLOWED_SIGNERS_FILE="+trust, "AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE="+trust, "AIGW_RELEASE_ARTIFACT_SIGNER=fixture@example.invalid",
		"AIGW_NATIVE_INPUT_SHA256="+fmt.Sprintf("%x", sha256.Sum256(archive)), "AIGW_TEST_PUBLIC_ARCHIVE="+input,
		"AIGW_TEST_GIT_PATH="+filepath.Join(operation, "git", "cmd", "git.exe"), "AIGW_TEST_NODE_PATH="+filepath.Join(operation, "node", "node.exe"), "SystemRoot="+operation,
		"CI_PROJECT_DIR="+project, "CI_PROJECT_URL=https://gitlab.test/team/aigw", "CI_JOB_ID=123")
	command.Env = environment
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("artifact-only native preparation failed: %v\n%s", err, output)
	}
	entries, err := os.ReadDir(filepath.Join(job, "native-input"))
	if err != nil || len(entries) != 2 || entries[0].Name() != "baseline" || entries[1].Name() != "candidate" {
		t.Fatalf("native preparation did not preserve the two selected artifact roots: %v, %v", entries, err)
	}
	requireProjectionInput(t, os.RemoveAll(filepath.Join(job, "native-input")))
	command = exec.CommandContext(ctx, pwsh, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", prelude+script+
		"\nforeach ($client in @{ codex = $env:AIGW_ACCEPTANCE_CODEX; claude = $env:AIGW_ACCEPTANCE_CLAUDE; hermes = $env:AIGW_ACCEPTANCE_HERMES }.GetEnumerator()) { "+
		"if (-not (Test-Path -LiteralPath $client.Value -PathType Leaf) -or (Get-Content -LiteralPath $client.Value -Raw) -cne ($client.Key + ' fixture')) { throw 'selected native client identity is unavailable' } }")
	command.Dir = operation
	command.Env = append(environment, "AIGW_NATIVE_CLIENTS=true", "AIGW_NATIVE_PERFORMANCE=false")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("selected native client preparation failed: %v\n%s", err, output)
	}
}

func requireProjectionInput(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestNativePublicInputAdmission(t *testing.T) {
	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		if runtime.GOOS == "windows" {
			t.Fatalf("Windows native input requires PowerShell: %v", err)
		}
		t.Skip("PowerShell native execution is optional outside Windows")
	}
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	command := projectionCommand(root, "nativePublicInputWindows")
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
		if runtime.GOOS == "windows" {
			t.Fatal(err)
		}
		t.Skip("PowerShell native execution is optional outside Windows")
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
