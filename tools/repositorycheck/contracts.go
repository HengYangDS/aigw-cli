package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var apiKeyShape = regexp.MustCompile(`sk-[A-Za-z0-9_-]{24,}`)
var bearerShape = regexp.MustCompile(`(?i)authorization:\s*bearer\s+[A-Za-z0-9_-]{24,}`)

func checkCredentials(root string) error {
	files, err := trackedRepositoryFiles(root)
	if err != nil {
		return err
	}
	for _, relative := range files {
		if strings.HasPrefix(relative, "tools/repositorycheck/") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			continue
		}
		if !apiKeyShape.Match(data) && !bearerShape.Match(data) {
			continue
		}
		if strings.HasSuffix(relative, "_test.go") {
			return fmt.Errorf("%s: credential-shaped test fixture found; use an aigw-test-* sentinel", relative)
		}
		return fmt.Errorf("%s: credential-shaped literal found outside test source", relative)
	}
	for relative, sentinel := range map[string]string{
		"internal/secrets/store_test.go":     "aigw-test-secret-never-leaks",
		"internal/diagnostics/probe_test.go": "aigw-test-gateway-token-never-leaks",
	} {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil || !strings.Contains(string(data), sentinel) {
			return fmt.Errorf("%s: required redaction sentinel is missing", relative)
		}
	}
	return nil
}

func checkProductSurface(root string) error {
	required := map[string][]string{
		"LICENSE":         {"MIT License\n\nCopyright (c) 2026 AIGW CLI Contributors", `THE SOFTWARE IS PROVIDED "AS IS"`},
		"README.md":       {"[MIT](LICENSE)", "MIT License"},
		"CHANGELOG.md":    {"## [Unreleased]"},
		"CONTRIBUTING.md": {"MIT License"},
		"docs/README.md":  {"[LICENSE](../LICENSE)"},
	}
	for relative, tokens := range required {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			return fmt.Errorf("product surface: read %s: %w", relative, err)
		}
		text := strings.ReplaceAll(string(data), "\r\n", "\n")
		for _, token := range tokens {
			if !strings.Contains(text, token) {
				return fmt.Errorf("product surface: %s is missing %q", relative, token)
			}
		}
	}
	for _, relative := range []string{"README.md", "CHANGELOG.md", "CONTRIBUTING.md", "docs", ".github", ".gitlab-ci.yml"} {
		path := filepath.Join(root, filepath.FromSlash(relative))
		err := filepath.WalkDir(path, func(filePath string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				if os.IsNotExist(walkErr) {
					return nil
				}
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			data, err := os.ReadFile(filePath)
			if err != nil {
				return err
			}
			if strings.Contains(string(data), "Proprietary") {
				return fmt.Errorf("product surface: proprietary licensing residue found in %s", filePath)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func trackedRepositoryFiles(root string) ([]string, error) {
	output, err := gitOutput(root, "ls-files", "-z")
	if err != nil {
		return nil, err
	}
	return strings.FieldsFunc(output, func(character rune) bool { return character == 0 }), nil
}
