package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var governanceFiles = []string{
	"AGENTS.md",
	"LICENSE",
	"CONTRIBUTING.md",
	"docs/README.md",
	"docs/architecture/authority-and-projection-boundary.md",
	"docs/governance/change-and-release-policy.md",
	"docs/governance/terminal-experience-contract.md",
	"docs/decisions/README.md",
	"docs/evidence/README.md",
	"docs/concepts/README.md",
	"docs/guides/team-rollout.md",
	"docs/governance/security.md",
	"docs/operations/release-readiness.md",
	".config/checks/architecture/policy.toml",
	".config/checks/coverage/policy.toml",
	".config/ci/verify-gates.toml",
	".ethos/profile.toml",
	".ethos/release.toml",
	".github/workflows/verify.yml",
	"tools/forge/main.go",
}

var localVerificationCommands = []string{
	"go run ./tools/ci source",
}

func checkGovernance(root string) error {
	for _, relative := range governanceFiles {
		if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); err != nil || info.IsDir() {
			return fmt.Errorf("missing governance file: %s", relative)
		}
	}
	for _, relative := range []string{"README.md", "CONTRIBUTING.md", "AGENTS.md"} {
		text, err := readText(root, relative)
		if err != nil {
			return err
		}
		for _, command := range localVerificationCommands {
			if !containsExactLine(text, command) {
				return fmt.Errorf("%s must list local verification command exactly: %s", relative, command)
			}
		}
		if strings.Contains(text, "go test -race ./...") {
			return fmt.Errorf("%s bypasses the required coverage gate", relative)
		}
		if strings.Contains(text, `test -z "$(gofmt -l cmd internal tools)"`) {
			return fmt.Errorf("%s retains a shell-owned verification command", relative)
		}
	}
	releaseReadiness, err := readText(root, "docs/operations/release-readiness.md")
	if err != nil {
		return err
	}
	if strings.Contains(strings.ToLower(releaseReadiness), "branch coverage") {
		return fmt.Errorf("docs/operations/release-readiness.md claims unsupported branch coverage")
	}
	for _, term := range []string{"statement coverage", "strictly above 95 percent"} {
		if !strings.Contains(strings.ToLower(releaseReadiness), term) {
			return fmt.Errorf("docs/operations/release-readiness.md is missing truthful coverage term %q", term)
		}
	}
	for _, relative := range []string{".ethos/profile.toml", ".ethos/release.toml"} {
		text, err := readText(root, relative)
		if err != nil {
			return err
		}
		if containsShellAutomation(text) {
			return fmt.Errorf("%s: shell-owned automation remains", relative)
		}
	}
	if output, err := gitOutput(root, "ls-files", "--cached", "--others", "--exclude-standard", "--modified", ".githooks"); err != nil {
		return fmt.Errorf("inspect tracked hook runtime: %w", err)
	} else if pathsExist(root, strings.Fields(output)) {
		return fmt.Errorf("tracked hook runtime remains: .githooks")
	}
	if err := checkProductSurface(root); err != nil {
		return err
	}
	if err := checkEnglishText(root); err != nil {
		return err
	}
	if err := checkCredentials(root); err != nil {
		return err
	}
	for _, relative := range []string{"docs/history", "docs/superpowers", "docs/design", "docs/reviews", "docs/specs"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); err == nil {
			return fmt.Errorf("retired documentary path remains: %s", relative)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	ignore, err := readText(root, ".gitignore")
	if err != nil {
		return err
	}
	if !containsExactLine(ignore, ".serena/") {
		return fmt.Errorf(".gitignore must exclude local Serena project metadata")
	}
	return nil
}

func pathsExist(root string, paths []string) bool {
	for _, relative := range paths {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); err == nil {
			return true
		} else if !os.IsNotExist(err) {
			return true
		}
	}
	return false
}

func containsShellAutomation(text string) bool {
	for _, marker := range []string{
		`["sh",`, `["bash",`, `["zsh",`, `["pwsh",`, `["powershell",`,
		".sh", ".bash", ".zsh", ".ps1", ".cmd", ".bat",
	} {
		if strings.Contains(strings.ToLower(text), marker) {
			return true
		}
	}
	return false
}

func containsExactLine(text, expected string) bool {
	for line := range strings.SplitSeq(text, "\n") {
		if strings.TrimSpace(line) == expected {
			return true
		}
	}
	return false
}

func readText(root, relative string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		return "", fmt.Errorf("read %s: %w", relative, err)
	}
	return string(data), nil
}
