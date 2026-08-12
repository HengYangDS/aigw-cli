package main

import (
	"bufio"
	"bytes"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

var semanticNameGrammars = map[string]struct {
	rule    string
	pattern *regexp.Regexp
}{
	".go": {"semantic_name_go", regexp.MustCompile(`^[a-z][a-z0-9_]*\.go$`)},
	".md": {"semantic_name_markdown", regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*\.md$`)},
}

var nativeCarrierNames = map[string]bool{
	"AGENTS.md": true, "CHANGELOG.md": true, "CONTRIBUTING.md": true, "README.md": true,
}

var chronicleDateName = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}\.md$`)

func checkSemanticNames(root string, report *Report) error {
	files, err := trackedFiles(root)
	if err != nil {
		return err
	}
	for _, relative := range files {
		name := path.Base(relative)
		if path.Ext(name) == ".py" {
			report.addFinding(Finding{Rule: "python_source", Path: relative, Message: "Go repository tooling must not retain a Python source plane"})
			continue
		}
		if err := checkPythonExecution(root, relative, report); err != nil {
			return err
		}
		checkPortableText(root, relative, report)
		grammar, managed := semanticNameGrammars[strings.ToLower(path.Ext(name))]
		if !managed || nativeCarrierNames[name] || isOpenSpecCarrier(relative, name) || isChronicleCarrier(relative, name) {
			continue
		}
		if !grammar.pattern.MatchString(name) {
			report.addFinding(Finding{Rule: grammar.rule, Path: relative, Message: "project-owned file name does not follow its carrier grammar"})
		}
	}
	return nil
}

func isChronicleCarrier(relative, name string) bool {
	return strings.HasPrefix(relative, "evidence/chronicle/") && chronicleDateName.MatchString(name)
}

func checkPythonExecution(root, relative string, report *Report) error {
	switch strings.ToLower(path.Ext(relative)) {
	case ".sh", ".yml", ".yaml":
	default:
		return nil
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		return fmt.Errorf("read %s: %w", relative, err)
	}
	for index, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.Contains(trimmed, "python3") || strings.Contains(trimmed, "PYTHONDONTWRITEBYTECODE") {
			report.addFinding(Finding{Rule: "python_execution", Path: relative, Line: index + 1, Message: "Go repository tooling must not execute a parallel Python toolchain"})
		}
	}
	return nil
}

func checkPortableText(root, relative string, report *Report) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		return
	}
	fixture := strings.HasPrefix(relative, "scripts/tests/") || strings.HasSuffix(relative, "_test.go") || strings.Contains("/"+relative+"/", "/testdata/") || strings.Contains("/"+relative+"/", "/fixtures/")
	if fixture {
		return
	}
	patterns := []struct {
		rule, message string
		pattern       *regexp.Regexp
	}{
		{"absolute_user_home", "product text must not bind to a local user home", regexp.MustCompile(`(?:/Users/|/home/)[A-Za-z0-9_.-]+/`)},
		{"absolute_windows_user_home", "product text must not bind to a Windows user home", regexp.MustCompile(`(?i)[A-Z]:[\\/]+Users[\\/]+[A-Za-z0-9_.-]+[\\/]`)},
		{"private_ipv4", "product text must not bind to a private IPv4 address", regexp.MustCompile(`(?:10\.(?:\d{1,3}\.){2}\d{1,3}|192\.168\.\d{1,3}\.\d{1,3}|172\.(?:1[6-9]|2\d|3[01])\.\d{1,3}\.\d{1,3})`)},
		{"personal_ssh_path", "product text must not bind to a personal SSH path", regexp.MustCompile(`(?:~|\$HOME)/\.ssh/[A-Za-z0-9_.-]+`)},
	}
	for index, line := range strings.Split(string(data), "\n") {
		for _, candidate := range patterns {
			if candidate.pattern.MatchString(line) {
				report.addFinding(Finding{Rule: candidate.rule, Path: relative, Line: index + 1, Message: candidate.message})
			}
		}
	}
}

func trackedFiles(root string) ([]string, error) {
	command := exec.Command("git", "ls-files", "-z", "--cached")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		if _, statErr := os.Stat(filepath.Join(root, ".git")); os.IsNotExist(statErr) {
			return workspaceFiles(root)
		}
		return nil, fmt.Errorf("list tracked files: %w", err)
	}
	parts := bytes.Split(output, []byte{0})
	files := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		relative := string(part)
		if _, statErr := os.Lstat(filepath.Join(root, filepath.FromSlash(relative))); statErr != nil {
			if os.IsNotExist(statErr) {
				continue
			}
			return nil, fmt.Errorf("stat tracked file %s: %w", relative, statErr)
		}
		files = append(files, relative)
	}
	return files, nil
}

func workspaceFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if filePath != root && strings.HasPrefix(entry.Name(), ".git") {
				return filepath.SkipDir
			}
			return nil
		}
		relative, err := filepath.Rel(root, filePath)
		if err != nil {
			return err
		}
		files = append(files, toPOSIX(relative))
		return nil
	})
	return files, err
}

func isOpenSpecCarrier(relative, name string) bool {
	if !strings.HasPrefix(relative, "openspec/") {
		return false
	}
	switch name {
	case "design.md", "proposal.md", "spec.md", "tasks.md":
		return true
	default:
		return false
	}
}

var foreignInternalImport = regexp.MustCompile(`(?:https?://|ssh://|git@|[^"/\s]+\.[^"/\s]+/)[^"\s]*/internal/`)
var implicitPublicationIdentity = regexp.MustCompile(`AIGW_(?:GITLAB|GITHUB)_(?:AUTHOR_(?:NAME|EMAIL)|SIGNING_KEY):-[^}]`)
var fixedRunnerInventory = regexp.MustCompile(`(?i)(?:aigw-(?:release|github-(?:verify|release))-macos-arm64|runs-on:\s*\[[^]]*self-hosted)`)

func checkModuleIdentity(root string, report *Report) error {
	module, err := readModuleIdentity(filepath.Join(root, "go.mod"))
	if err != nil {
		return err
	}
	if module != "aigw-cli" || strings.ContainsAny(module, `:\`) || strings.HasPrefix(module, "/") || strings.Contains(module, ".") && strings.Contains(module, "/") {
		report.addFinding(Finding{Rule: "module_identity", Path: "go.mod", Message: "module must use the non-fetchable product build identity aigw-cli"})
	}
	return filepath.WalkDir(root, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(entry.Name()) != ".go" {
			return nil
		}
		relative, err := filepath.Rel(root, filePath)
		if err != nil {
			return err
		}
		relative = toPOSIX(relative)
		if !strings.Contains(relative, "/") || !strings.HasPrefix(relative, "internal/") && !strings.HasPrefix(relative, "cmd/") && !strings.HasPrefix(relative, "tools/") {
			report.addFinding(Finding{Rule: "public_go_package", Path: relative, Message: "public Go packages require an explicitly owned resolvable module identity"})
		}
		data, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), relative, data, parser.ImportsOnly)
		if err != nil {
			return nil
		}
		for _, imported := range parsed.Imports {
			if foreignInternalImport.MatchString(imported.Path.Value) {
				report.addFinding(Finding{Rule: "foreign_internal_import", Path: relative, Message: "internal imports must use the product build identity"})
			}
		}
		return nil
	})
}

func readModuleIdentity(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}
	defer func() { _ = file.Close() }()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && fields[0] == "module" {
			return fields[1], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("go.mod has no module declaration")
}

func checkPortability(root string, report *Report) error {
	files, err := trackedFiles(root)
	if err != nil {
		return err
	}
	for _, relative := range files {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			continue
		}
		text := string(data)
		if shellOwnedAutomation(relative, data) {
			report.addFinding(Finding{Rule: "shell_owned_automation", Path: relative, Message: "repository automation must be owned by portable Go commands and tests"})
		}
		if (strings.HasPrefix(relative, "scripts/forge/") || strings.HasPrefix(relative, "scripts/release/") || strings.HasPrefix(relative, "scripts/checks/forge/")) && implicitPublicationIdentity.MatchString(text) {
			report.addFinding(Finding{Rule: "implicit_publication_identity", Path: relative, Message: "publication identity must be explicit execution input"})
		}
		if strings.HasPrefix(relative, ".config/release/") && strings.HasSuffix(relative, "allowed-signers") {
			report.addFinding(Finding{Rule: "tracked_trust_anchor", Path: relative, Message: "publication trust anchors must be protected execution inputs"})
		}
		if (strings.HasPrefix(relative, ".config/ci/") || strings.HasPrefix(relative, ".github/") || relative == ".gitlab-ci.yml") && fixedRunnerInventory.MatchString(text) {
			report.addFinding(Finding{Rule: "fixed_runner_inventory", Path: relative, Message: "runner inventory must be supplied by the adopting Forge"})
		}
	}
	return nil
}

func shellOwnedAutomation(relative string, data []byte) bool {
	extension := strings.ToLower(filepath.Ext(relative))
	if extension == ".sh" || extension == ".bash" || extension == ".zsh" || extension == ".ps1" || extension == ".cmd" || extension == ".bat" {
		return true
	}
	first, _, _ := bytes.Cut(data, []byte{'\n'})
	shebang := strings.ToLower(string(first))
	return strings.HasPrefix(shebang, "#!") && (strings.Contains(shebang, "/sh") || strings.Contains(shebang, "bash") || strings.Contains(shebang, "zsh") || strings.Contains(shebang, "powershell") || strings.Contains(shebang, "pwsh"))
}
