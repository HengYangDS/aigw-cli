package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

func checkSourceSize(root string, runner commandRunner) (result error) {
	files, err := currentRepositoryFiles(root, "Go source", "*.go")
	if err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(root, ".config", "checks", "go", "size.toml"))
	if err != nil {
		return err
	}
	var policy struct {
		MaxCodeLines int `toml:"max_code_lines"`
	}
	decoder := toml.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&policy); err != nil {
		return fmt.Errorf("read source-size policy: %w", err)
	}
	if policy.MaxCodeLines <= 0 {
		return errors.New("max_code_lines must be positive")
	}
	parent := filepath.Join(root, "build", "tmp")
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return err
	}
	workspace, err := os.MkdirTemp(parent, ".source-size-")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, os.RemoveAll(workspace)) }()
	output := filepath.Join(workspace, "sizes.json")
	arguments := []string{
		"--no-config", "--no-cocomo", "--no-complexity", "--no-gitignore", "--no-ignore",
		"--no-scc-ignore", "--include-symlinks", "--by-file", "--format", "json", "--output", output, "--",
	}
	arguments = append(arguments, files...)
	for i, argument := range arguments {
		// SCC's response-file tokenizer groups quotes but has no escape syntax.
		// Adjacent single/double-quoted segments preserve quote characters.
		arguments[i] = "'" + strings.ReplaceAll(argument, "'", "'\"'\"'") + "'"
	}
	input := filepath.Join(workspace, "arguments.txt")
	if err := os.WriteFile(input, []byte(strings.Join(arguments, "\n")), 0o600); err != nil {
		return err
	}
	if err := runner(command{Name: "scc", Dir: root, Args: []string{"@" + input}}); err != nil {
		return err
	}
	measured, err := os.ReadFile(output)
	if err != nil {
		return fmt.Errorf("read SCC measurement: %w", err)
	}
	return validateSourceSizes(measured, files, policy.MaxCodeLines)
}

func validateSourceSizes(raw []byte, files []string, maximum int) error {
	var groups []struct {
		Files []struct {
			Location string `json:"Location"`
			Code     *int   `json:"Code"`
		} `json:"Files"`
	}
	if err := json.Unmarshal(raw, &groups); err != nil {
		return fmt.Errorf("decode SCC measurement: %w", err)
	}
	expected := make(map[string]bool, len(files))
	for _, file := range files {
		expected[filepath.Clean(file)] = false
	}
	var findings []string
	for _, group := range groups {
		for _, file := range group.Files {
			path := filepath.Clean(file.Location)
			seen, selected := expected[path]
			if !selected || seen || file.Code == nil || *file.Code < 0 {
				return fmt.Errorf("invalid, duplicate or unselected SCC measurement for %s", path)
			}
			expected[path] = true
			if *file.Code > maximum {
				findings = append(findings, fmt.Sprintf("%s: code lines %d > %d", path, *file.Code, maximum))
			}
		}
	}
	for _, file := range files {
		if !expected[filepath.Clean(file)] {
			return fmt.Errorf("SCC measurement is missing %s", file)
		}
	}
	if len(findings) != 0 {
		slices.Sort(findings)
		return errors.New(strings.Join(findings, "\n"))
	}
	return nil
}
