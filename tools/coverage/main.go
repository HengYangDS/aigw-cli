// Package main measures and enforces AIGW's Go coverage policy.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"maps"
	"math"
	"os"
	"os/exec"
	pathpkg "path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
	"golang.org/x/tools/cover"
)

// policyPath is where the coverage policy lives in the checkout that the tool
// runs from.
const policyPath = ".config/checks/coverage/policy.toml"

type coveragePolicy struct {
	MinimumStatementPercent float64  `toml:"minimum_statement_percent"`
	Comparison              string   `toml:"comparison"`
	ThresholdScopes         []string `toml:"threshold_scopes"`
	PackageObservation      string   `toml:"package_observation"`
	CoverMode               string   `toml:"covermode"`
	PackageTimeout          string   `toml:"package_timeout"`
	Packages                []string `toml:"packages"`
	Owner                   string   `toml:"owner"`
	Source                  string   `toml:"source"`
	RiskModel               string   `toml:"risk_model"`
	Measurement             string   `toml:"measurement"`
	FalsePositiveCost       string   `toml:"false_positive_cost"`
	Remediation             string   `toml:"remediation"`
	ReviewCondition         string   `toml:"review_condition"`
}

type coverageResult struct {
	Covered  int64
	Total    int64
	Packages map[string]coverageCount
}

type coverageCount struct {
	Covered int64
	Total   int64
}

func (result coverageResult) Percent() float64 {
	return float64(result.Covered) * 100 / float64(result.Total)
}

func (c coverageCount) Percent() float64 {
	return float64(c.Covered) * 100 / float64(c.Total)
}

type commandRunner interface {
	Run(name string, args []string, stdout, stderr io.Writer) error
}

type systemRunner struct{}

// Run keeps the child in this process's group: in a group of its own, a signal
// here would kill go test and the test binaries before they stop the children
// that they started in groups of their own.
func (systemRunner) Run(name string, args []string, stdout, stderr io.Writer) error {
	command := exec.Command(name, args...)
	command.Stdout = stdout
	command.Stderr = stderr
	return command.Run()
}

func main() {
	os.Exit(realMain(os.Args[1:], os.Stdout, os.Stderr, systemRunner{}))
}

func realMain(args []string, stdout, stderr io.Writer, runner commandRunner) (code int) {
	flags := flag.NewFlagSet("coverage", flag.ContinueOnError)
	flags.SetOutput(stderr)
	race := flags.Bool("race", false, "enable Go's race detector")
	jsonOutput := flags.Bool("json", false, "preserve native Go test JSON on stdout; coverage reports use stderr")
	tags := flags.String("tags", "", "Go build tags for package observation and tests")
	profileOutput := flags.String("profile-output", "", "retain the Go coverage profile at this path")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "coverage gate does not accept positional arguments")
		return 2
	}

	policy, err := loadPolicy()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load coverage policy: %v\n", err)
		return 1
	}
	var packageOutput bytes.Buffer
	// Test-only packages still run. Every production package is observed;
	// a missing profile requires positive declaration-only source evidence.
	listArgs := append([]string{"list", "-f", "{{if .GoFiles}}{{.ImportPath}}\t{{.Module.Path}}{{end}}", "-tags=" + *tags}, policy.Packages...)
	if err := runner.Run("go", listArgs, &packageOutput, stderr); err != nil {
		_, _ = fmt.Fprintf(stderr, "go list failed: %v\n", err)
		return 1
	}
	expectedPackages, err := parsePackageList(packageOutput.String())
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "parse package list: %v\n", err)
		return 1
	}
	profile, err := os.CreateTemp("", "aigw-coverage-*.out")
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "create coverage profile: %v\n", err)
		return 1
	}
	profilePath := profile.Name()
	defer func() {
		if err := os.Remove(profilePath); err != nil && !os.IsNotExist(err) {
			_, _ = fmt.Fprintf(stderr, "remove coverage profile %s: %v\n", profilePath, err)
			code = 1
		}
	}()
	if err := profile.Close(); err != nil {
		_, _ = fmt.Fprintf(stderr, "close coverage profile: %v\n", err)
		return 1
	}

	// Bind instrumentation to the same canonical package inventory as observation.
	// A relative coverpkg pattern also matches a toolchain installed below root.
	// The policy's package limit replaces Go's default, so a slow shared host still finishes.
	goArgs := []string{
		"test", "-count=1", "-timeout=" + policy.PackageTimeout, "-tags=" + *tags, "-covermode=" + policy.CoverMode,
		"-coverpkg=" + strings.Join(expectedPackages, ","), "-coverprofile=" + profilePath,
	}
	if *race {
		goArgs = append(goArgs, "-race")
	}
	reportOutput := stdout
	if *jsonOutput {
		goArgs = append(goArgs, "-json")
		reportOutput = stderr
	}
	goArgs = append(goArgs, policy.Packages...)
	testErr := runner.Run("go", goArgs, stdout, stderr)
	retainErr := retainCoverageProfile(profilePath, *profileOutput)
	if err := errors.Join(testErr, retainErr); err != nil {
		_, _ = fmt.Fprintf(stderr, "native coverage attempt failed: %v\n", err)
		return 1
	}

	result, err := readCoverage(profilePath, policy.CoverMode)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "read coverage profile: %v\n", err)
		return 1
	}
	if err := result.requireCoverage(expectedPackages, policy.MinimumStatementPercent, runner, reportOutput, "-tags="+*tags); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func (result coverageResult) requireCoverage(expected []string, minimum float64, runner commandRunner, stdout io.Writer, selection ...string) error {
	var missing error
	selected := make(map[string]bool, len(expected))
	for _, name := range expected {
		selected[name] = true
		if _, observed := result.Packages[name]; observed {
			continue
		}
		declarations, err := declarationOnlyPackage(name, runner, selection...)
		if err == nil && declarations {
			if _, err := fmt.Fprintf(stdout, "package %s statement coverage: not applicable (declarations only)\n", name); err != nil {
				return fmt.Errorf("write package observation: %w", err)
			}
			continue
		}
		missing = errors.Join(missing, fmt.Errorf("package %s is absent from the coverage profile", name))
		if err != nil {
			missing = errors.Join(missing, fmt.Errorf("inspect package source: %w", err))
		}
	}
	if missing != nil {
		return missing
	}
	for _, name := range slices.Sorted(maps.Keys(result.Packages)) {
		if !selected[name] {
			return fmt.Errorf("package %s is outside the selected coverage scope", name)
		}
		count := result.Packages[name]
		if count.Total == 0 {
			if _, err := fmt.Fprintf(stdout, "package %s statement coverage: not applicable (0 measured statements)\n", name); err != nil {
				return fmt.Errorf("write package observation: %w", err)
			}
			continue
		}
		if count.Covered == 0 {
			return fmt.Errorf("package %s has no executed statements (%d total)", name, count.Total)
		}
		if _, err := fmt.Fprintf(stdout, "package %s statement coverage: %.2f%% (%d/%d statements)\n", name, count.Percent(), count.Covered, count.Total); err != nil {
			return fmt.Errorf("write package observation: %w", err)
		}
	}
	percent := result.Percent()
	if percent <= minimum {
		return fmt.Errorf("coverage %.2f%% does not exceed %.2f%% (%d/%d statements)", percent, minimum, result.Covered, result.Total)
	}
	if _, err := fmt.Fprintf(stdout, "statement coverage: %.2f%% (%d/%d statements), required > %.2f%%\n", percent, result.Covered, result.Total, minimum); err != nil {
		return fmt.Errorf("write coverage result: %w", err)
	}
	return nil
}

// declarationOnlyPackage proves missing counters are justified using Go's
// platform-selected source files, without implementing statement counting.
func declarationOnlyPackage(name string, runner commandRunner, selection ...string) (bool, error) {
	var output, diagnostics bytes.Buffer
	if err := runner.Run("go", append(append([]string{"list", "-json"}, selection...), name), &output, &diagnostics); err != nil {
		return false, fmt.Errorf("go list %s: %w: %s", name, err, &diagnostics)
	}
	var pkg struct {
		Dir        string   `json:"Dir"`
		ImportPath string   `json:"ImportPath"`
		GoFiles    []string `json:"GoFiles"`
		CgoFiles   []string `json:"CgoFiles"`
	}
	if err := json.Unmarshal(output.Bytes(), &pkg); err != nil {
		return false, err
	}
	files := append(pkg.GoFiles, pkg.CgoFiles...)
	if pkg.ImportPath != name || pkg.Dir == "" || len(files) == 0 {
		return false, fmt.Errorf("go list returned incomplete source for %s", name)
	}
	for _, name := range files {
		source, err := parser.ParseFile(token.NewFileSet(), filepath.Join(pkg.Dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			return false, err
		}
		for node := range ast.Preorder(source) {
			switch node := node.(type) {
			case *ast.FuncDecl:
				if node.Body != nil {
					return false, nil
				}
			case *ast.FuncLit:
				return false, nil
			}
		}
	}
	return true, nil
}

func retainCoverageProfile(source, target string) error {
	if target == "" {
		return nil
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.WriteFile(target, data, 0o600)
}

func loadPolicy() (coveragePolicy, error) {
	data, err := os.ReadFile(policyPath)
	if err != nil {
		return coveragePolicy{}, err
	}
	var policy coveragePolicy
	decoder := toml.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&policy); err != nil {
		return coveragePolicy{}, err
	}
	if policy.MinimumStatementPercent <= 0 || policy.MinimumStatementPercent >= 100 {
		return coveragePolicy{}, fmt.Errorf("minimum_statement_percent must be greater than 0 and less than 100")
	}
	if policy.Comparison != "greater-than" {
		return coveragePolicy{}, fmt.Errorf("comparison must be greater-than")
	}
	if len(policy.ThresholdScopes) != 1 || policy.ThresholdScopes[0] != "aggregate" {
		return coveragePolicy{}, fmt.Errorf("threshold_scopes must contain exactly aggregate")
	}
	if policy.PackageObservation != "required" {
		return coveragePolicy{}, fmt.Errorf("package_observation must be required")
	}
	if policy.CoverMode != "atomic" {
		return coveragePolicy{}, fmt.Errorf("covermode must be atomic")
	}
	if timeout, err := time.ParseDuration(policy.PackageTimeout); err != nil || timeout <= 0 {
		return coveragePolicy{}, fmt.Errorf("package_timeout must be a positive Go duration")
	}
	if len(policy.Packages) != 1 || policy.Packages[0] != "./..." {
		return coveragePolicy{}, fmt.Errorf("packages must contain exactly ./...")
	}
	if strings.TrimSpace(policy.Owner) == "" || strings.TrimSpace(policy.Source) == "" {
		return coveragePolicy{}, fmt.Errorf("owner and source must be non-empty")
	}
	if strings.TrimSpace(policy.RiskModel) == "" ||
		strings.TrimSpace(policy.Measurement) == "" ||
		strings.TrimSpace(policy.FalsePositiveCost) == "" ||
		strings.TrimSpace(policy.Remediation) == "" ||
		strings.TrimSpace(policy.ReviewCondition) == "" {
		return coveragePolicy{}, fmt.Errorf("risk rationale fields must be non-empty")
	}
	return policy, nil
}

func parsePackageList(output string) ([]string, error) {
	seen := map[string]bool{}
	var packages []string
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 2 || fields[0] == "" || fields[1] == "" {
			return nil, fmt.Errorf("package row must contain import and module paths")
		}
		name, module := fields[0], fields[1]
		if name != module && !strings.HasPrefix(name, module+"/") {
			return nil, fmt.Errorf("package %q is outside module %q", name, module)
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		packages = append(packages, name)
	}
	if len(packages) == 0 {
		return nil, fmt.Errorf("go list returned no packages")
	}
	slices.Sort(packages)
	return packages, nil
}

func readCoverage(path, expectedMode string) (coverageResult, error) {
	profiles, err := cover.ParseProfiles(path)
	if err != nil {
		return coverageResult{}, err
	}
	result := coverageResult{Packages: map[string]coverageCount{}}
	for _, profile := range profiles {
		if profile.Mode != expectedMode {
			return coverageResult{}, fmt.Errorf("profile mode %q does not match %q", "mode: "+profile.Mode, expectedMode)
		}
		packageName := pathpkg.Dir(profile.FileName)
		packageCoverage := result.Packages[packageName]
		for _, block := range profile.Blocks {
			statements := int64(block.NumStmt)
			if statements > math.MaxInt64-result.Total {
				return coverageResult{}, fmt.Errorf("coverage profile overflows statement total")
			}
			result.Total += statements
			packageCoverage.Total += statements
			if block.Count > 0 {
				result.Covered += statements
				packageCoverage.Covered += statements
			}
		}
		result.Packages[packageName] = packageCoverage
	}
	if result.Total == 0 {
		return coverageResult{}, fmt.Errorf("profile contains no statements")
	}
	return result, nil
}
