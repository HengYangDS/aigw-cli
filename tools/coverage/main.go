// Package main measures and enforces AIGW's Go coverage policy.
package main

import (
	"bufio"
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
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const defaultPolicyPath = ".config/checks/coverage/policy.toml"

type coveragePolicy struct {
	MinimumStatementPercent float64  `toml:"minimum_statement_percent"`
	Comparison              string   `toml:"comparison"`
	ThresholdScopes         []string `toml:"threshold_scopes"`
	PackageObservation      string   `toml:"package_observation"`
	CoverMode               string   `toml:"covermode"`
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

func (systemRunner) Run(name string, args []string, stdout, stderr io.Writer) error {
	command := exec.Command(name, args...)
	command.Stdout = stdout
	command.Stderr = stderr
	return command.Run()
}

func main() {
	os.Exit(realMain(os.Args[1:], os.Stdout, os.Stderr, systemRunner{}))
}

// createCoverageProfile is overridden in tests to exercise temporary-file
// creation and closure failures without depending on platform-specific
// temporary-directory environment variables (TMPDIR is not honored on
// Windows).
var createCoverageProfile = func() (*os.File, error) {
	return os.CreateTemp("", "aigw-coverage-*.out")
}

var removeCoverageProfile = os.Remove

func realMain(args []string, stdout, stderr io.Writer, runner commandRunner) (code int) {
	flags := flag.NewFlagSet("coverage", flag.ContinueOnError)
	flags.SetOutput(stderr)
	policyPath := flags.String("policy", defaultPolicyPath, "coverage policy TOML")
	race := flags.Bool("race", false, "enable Go's race detector")
	profileOutput := flags.String("profile-output", "", "retain the Go coverage profile at this path")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "coverage gate does not accept positional arguments")
		return 2
	}

	policy, err := loadPolicy(*policyPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load coverage policy: %v\n", err)
		return 1
	}
	var packageOutput bytes.Buffer
	// Test-only packages still run. Every production package is observed;
	// a missing profile requires positive declaration-only source evidence.
	listArgs := append([]string{"list", "-f", "{{if .GoFiles}}{{.ImportPath}}\t{{.Module.Path}}{{end}}"}, policy.Packages...)
	if err := runner.Run("go", listArgs, &packageOutput, stderr); err != nil {
		_, _ = fmt.Fprintf(stderr, "go list failed: %v\n", err)
		return 1
	}
	expectedPackages, err := parsePackageList(packageOutput.String())
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "parse package list: %v\n", err)
		return 1
	}
	profile, err := createCoverageProfile()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "create coverage profile: %v\n", err)
		return 1
	}
	profilePath := profile.Name()
	defer func() {
		if err := removeCoverageProfile(profilePath); err != nil && !os.IsNotExist(err) {
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
	goArgs := []string{
		"test", "-count=1", "-covermode=" + policy.CoverMode,
		"-coverpkg=" + strings.Join(expectedPackages, ","), "-coverprofile=" + profilePath,
	}
	if *race {
		goArgs = append(goArgs, "-race")
	}
	goArgs = append(goArgs, policy.Packages...)
	if err := runner.Run("go", goArgs, stdout, stderr); err != nil {
		_, _ = fmt.Fprintf(stderr, "go test failed: %v\n", err)
		return 1
	}

	result, err := readCoverage(profilePath, policy.CoverMode)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "read coverage profile: %v\n", err)
		return 1
	}
	if err := result.requirePackages(expectedPackages, runner, stdout); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	percent := result.Percent()
	if percent <= policy.MinimumStatementPercent {
		_, _ = fmt.Fprintf(stderr, "coverage %.2f%% does not exceed %.2f%% (%d/%d statements)\n", percent, policy.MinimumStatementPercent, result.Covered, result.Total)
		return 1
	}
	if err := retainCoverageProfile(profilePath, *profileOutput); err != nil {
		_, _ = fmt.Fprintf(stderr, "retain coverage profile: %v\n", err)
		return 1
	}
	if _, err := fmt.Fprintf(stdout, "statement coverage: %.2f%% (%d/%d statements), required > %.2f%%\n", percent, result.Covered, result.Total, policy.MinimumStatementPercent); err != nil {
		_, _ = fmt.Fprintf(stderr, "write coverage result: %v\n", err)
		return 1
	}
	return 0
}

func (result coverageResult) requirePackages(expected []string, runner commandRunner, stdout io.Writer) error {
	var missing error
	selected := make(map[string]bool, len(expected))
	for _, name := range expected {
		selected[name] = true
		if _, observed := result.Packages[name]; observed {
			continue
		}
		declarations, err := declarationOnlyPackage(name, runner)
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
	return nil
}

// declarationOnlyPackage proves missing counters are justified using Go's
// platform-selected source files, without implementing statement counting.
func declarationOnlyPackage(name string, runner commandRunner) (bool, error) {
	var output, diagnostics bytes.Buffer
	if err := runner.Run("go", []string{"list", "-json", name}, &output, &diagnostics); err != nil {
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

func loadPolicy(path string) (coveragePolicy, error) {
	data, err := os.ReadFile(path)
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

func coveragePercent(covered, total int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(covered) * 100 / float64(total)
}

func readCoverage(path, expectedMode string) (coverageResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return coverageResult{}, err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return coverageResult{}, fmt.Errorf("profile is empty")
	}
	if header := scanner.Text(); header != "mode: "+expectedMode {
		return coverageResult{}, fmt.Errorf("profile mode %q does not match %q", header, expectedMode)
	}

	result := coverageResult{Packages: map[string]coverageCount{}}
	type sourceRange struct {
		packageName string
		statements  int64
		covered     bool
	}
	ranges := map[string]sourceRange{}
	line := 1
	for scanner.Scan() {
		line++
		fields := strings.Fields(scanner.Text())
		if len(fields) != 3 {
			return coverageResult{}, fmt.Errorf("line %d must have three fields", line)
		}
		separator := strings.LastIndex(fields[0], ":")
		if separator <= 0 {
			return coverageResult{}, fmt.Errorf("line %d has invalid source range", line)
		}
		packageName := pathpkg.Dir(fields[0][:separator])
		statements, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil || statements < 0 {
			return coverageResult{}, fmt.Errorf("line %d has invalid statement count", line)
		}
		count, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil || count < 0 {
			return coverageResult{}, fmt.Errorf("line %d has invalid execution count", line)
		}
		rangeID := fields[0]
		covered := count > 0
		if existing, ok := ranges[rangeID]; ok {
			if existing.packageName != packageName || existing.statements != statements {
				return coverageResult{}, fmt.Errorf("line %d conflicts with repeated source range", line)
			}
			existing.covered = existing.covered || covered
			ranges[rangeID] = existing
			continue
		}
		ranges[rangeID] = sourceRange{packageName: packageName, statements: statements, covered: covered}
	}
	if err := scanner.Err(); err != nil {
		return coverageResult{}, err
	}
	for _, source := range ranges {
		if source.statements > math.MaxInt64-result.Total {
			return coverageResult{}, fmt.Errorf("coverage profile overflows statement total")
		}
		result.Total += source.statements
		packageCoverage := result.Packages[source.packageName]
		packageCoverage.Total += source.statements
		if source.covered {
			result.Covered += source.statements
			packageCoverage.Covered += source.statements
		}
		result.Packages[source.packageName] = packageCoverage
	}
	if result.Total == 0 {
		return coverageResult{}, fmt.Errorf("profile contains no statements")
	}
	return result, nil
}
