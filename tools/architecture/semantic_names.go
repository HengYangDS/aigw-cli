package main

import (
	"bufio"
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
	".go": {"semantic_name_go", regexp.MustCompile(`^[a-z][a-z0-9_]*\.go$`)},
	".md": {"semantic_name_markdown", regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*\.md$`)},
}

var nativeCarrierNames = map[string]bool{
	"AGENTS.md": true, "CHANGELOG.md": true, "CONTRIBUTING.md": true, "README.md": true,
}

func checkSemanticNames(root string, report *Report) error {
	files, err := trackedFiles(root)
	if err != nil {
		return err
	}
	for _, relative := range files {
		name := path.Base(relative)
		grammar, managed := semanticNameGrammars[strings.ToLower(path.Ext(name))]
		if !managed || nativeCarrierNames[name] || isOpenSpecCarrier(relative, name) {
			continue
		}
		if !grammar.pattern.MatchString(name) {
			report.addFinding(Finding{Rule: grammar.rule, Path: relative, Message: "project-owned file name does not follow its carrier grammar"})
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

func readModuleIdentity(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}
	defer func() { _ = file.Close() }()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && fields[0] == "module" && strings.TrimSpace(fields[1]) != "" {
			return fields[1], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("go.mod has no module declaration")
}
