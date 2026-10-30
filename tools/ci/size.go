package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// sizeBudget is the most lines that SCC may count in one file, and what they
// are.
type sizeBudget struct {
	maximum int
	unit    string
}

func checkSourceSize(root string, runner commandRunner) (result error) {
	files, err := currentRepositoryFiles(root, "Go source", "*.go")
	if err != nil {
		return err
	}
	specifications, err := measuredSpecifications(root)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(root, ".config", "checks", "scc", "policy.toml"))
	if err != nil {
		return err
	}
	var policy struct {
		MaxCodeLines          int `toml:"max_code_lines"`
		MaxSpecificationLines int `toml:"max_specification_lines"`
	}
	decoder := toml.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&policy); err != nil {
		return fmt.Errorf("read source-size policy: %w", err)
	}
	if policy.MaxCodeLines <= 0 || policy.MaxSpecificationLines <= 0 {
		return errors.New("max_code_lines and max_specification_lines must be positive")
	}
	budgets := make(map[string]sizeBudget, len(files)+len(specifications))
	for _, file := range files {
		budgets[filepath.Clean(file)] = sizeBudget{maximum: policy.MaxCodeLines, unit: "code lines"}
	}
	for _, file := range specifications {
		budgets[filepath.Clean(file)] = sizeBudget{maximum: policy.MaxSpecificationLines, unit: "non-blank lines"}
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
	arguments = append(arguments, specifications...)
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
	return validateSourceSizes(measured, budgets)
}

// measuredSpecifications lists the main specifications that no active Change
// rewrites and the active Changes' deltas.
func measuredSpecifications(root string) ([]string, error) {
	files, err := repositoryFiles(root, "OpenSpec specifications", "openspec/specs/*/spec.md", "openspec/changes/*/specs/*/spec.md")
	if err != nil {
		return nil, err
	}
	relatives := make([]string, len(files))
	for index, file := range files {
		relative, err := filepath.Rel(root, file)
		if err != nil {
			return nil, fmt.Errorf("resolve specification relative to checkout: %w", err)
		}
		relatives[index] = filepath.ToSlash(relative)
	}
	transition, err := readOpenSpecTransition(root, relatives)
	if err != nil {
		return nil, err
	}
	measured := make([]string, 0, len(files))
	for index, file := range files {
		if !strings.HasPrefix(relatives[index], "openspec/changes/archive/") && !transition.rewritten[relatives[index]] {
			measured = append(measured, file)
		}
	}
	return measured, nil
}

func validateSourceSizes(raw []byte, budgets map[string]sizeBudget) error {
	var groups []struct {
		Files []struct {
			Location string `json:"Location"`
			Code     *int   `json:"Code"`
		} `json:"Files"`
	}
	if err := json.Unmarshal(raw, &groups); err != nil {
		return fmt.Errorf("decode SCC measurement: %w", err)
	}
	seen := make(map[string]bool, len(budgets))
	var findings []string
	for _, group := range groups {
		for _, file := range group.Files {
			path := filepath.Clean(file.Location)
			budget, selected := budgets[path]
			if !selected || seen[path] || file.Code == nil || *file.Code < 0 {
				return fmt.Errorf("invalid, duplicate or unselected SCC measurement for %s", path)
			}
			seen[path] = true
			if *file.Code > budget.maximum {
				findings = append(findings, fmt.Sprintf("%s: %s %d > %d", path, budget.unit, *file.Code, budget.maximum))
			}
		}
	}
	for _, path := range slices.Sorted(maps.Keys(budgets)) {
		if !seen[path] {
			return fmt.Errorf("SCC measurement is missing %s", path)
		}
	}
	if len(findings) != 0 {
		slices.Sort(findings)
		return errors.New(strings.Join(findings, "\n"))
	}
	return nil
}
