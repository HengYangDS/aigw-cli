package main

import (
	"bytes"
	"fmt"
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
	".go":  {"semantic_name_go", regexp.MustCompile(`^[a-z][a-z0-9_]*\.go$`)},
	".md":  {"semantic_name_markdown", regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*\.md$`)},
	".ps1": {"semantic_name_powershell", regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*\.ps1$`)},
	".sh":  {"semantic_name_shell", regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*\.sh$`)},
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
		if err := checkPortableText(root, relative, report); err != nil {
			return err
		}
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

func checkPortableText(root, relative string, report *Report) error {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		return nil
	}
	fixture := strings.HasPrefix(relative, "scripts/tests/") || strings.HasSuffix(relative, "_test.go") || strings.Contains("/"+relative+"/", "/testdata/") || strings.Contains("/"+relative+"/", "/fixtures/")
	if fixture {
		return nil
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
	return nil
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
