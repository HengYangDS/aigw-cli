package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	pathpkg "path"
	"sort"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const defaultPolicyPath = ".config/checks/coverage/policy.toml"

type coveragePolicy struct {
	MinimumStatementPercent float64  `toml:"minimum_statement_percent"`
	Comparison              string   `toml:"comparison"`
	CoverMode               string   `toml:"covermode"`
	Packages                []string `toml:"packages"`
	Owner                   string   `toml:"owner"`
	Source                  string   `toml:"source"`
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

func (r coverageResult) Percent() float64 {
	return float64(r.Covered) * 100 / float64(r.Total)
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

func realMain(args []string, stdout, stderr io.Writer, runner commandRunner) int {
	flags := flag.NewFlagSet("coveragegate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	policyPath := flags.String("policy", defaultPolicyPath, "coverage policy TOML")
	race := flags.Bool("race", false, "enable Go's race detector")
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
	listArgs := append([]string{"list", "-f", "{{.ImportPath}}"}, policy.Packages...)
	if err := runner.Run("go", listArgs, &packageOutput, stderr); err != nil {
		_, _ = fmt.Fprintf(stderr, "go list failed: %v\n", err)
		return 1
	}
	expectedPackages := strings.Fields(packageOutput.String())
	if len(expectedPackages) == 0 {
		_, _ = fmt.Fprintln(stderr, "go list returned no packages")
		return 1
	}
	sort.Strings(expectedPackages)
	profile, err := createCoverageProfile()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "create coverage profile: %v\n", err)
		return 1
	}
	profilePath := profile.Name()
	if err := profile.Close(); err != nil {
		_ = os.Remove(profilePath)
		_, _ = fmt.Fprintf(stderr, "close coverage profile: %v\n", err)
		return 1
	}
	defer func() { _ = os.Remove(profilePath) }()

	goArgs := []string{"test", "-count=1"}
	if *race {
		goArgs = append(goArgs, "-race")
	}
	goArgs = append(goArgs, "-covermode="+policy.CoverMode, "-coverprofile="+profilePath)
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
	packagesMissing := false
	for index, name := range expectedPackages {
		if index > 0 && name == expectedPackages[index-1] {
			continue
		}
		if _, ok := result.Packages[name]; !ok {
			_, _ = fmt.Fprintf(stderr, "package %s is absent from the coverage profile\n", name)
			packagesMissing = true
		}
	}
	if packagesMissing {
		return 1
	}
	percent := result.Percent()
	if percent <= policy.MinimumStatementPercent {
		_, _ = fmt.Fprintf(stderr, "coverage %.2f%% does not exceed %.2f%% (%d/%d statements)\n", percent, policy.MinimumStatementPercent, result.Covered, result.Total)
		return 1
	}
	packageNames := make([]string, 0, len(result.Packages))
	for name := range result.Packages {
		packageNames = append(packageNames, name)
	}
	sort.Strings(packageNames)
	packagesFailed := false
	for _, name := range packageNames {
		count := result.Packages[name]
		if count.Percent() <= policy.MinimumStatementPercent {
			_, _ = fmt.Fprintf(stderr, "package %s coverage %.2f%% does not exceed %.2f%% (%d/%d statements)\n", name, count.Percent(), policy.MinimumStatementPercent, count.Covered, count.Total)
			packagesFailed = true
		}
	}
	if packagesFailed {
		return 1
	}
	if _, err := fmt.Fprintf(stdout, "coverage: %.2f%% (%d/%d statements), required > %.2f%%\n", percent, result.Covered, result.Total, policy.MinimumStatementPercent); err != nil {
		_, _ = fmt.Fprintf(stderr, "write coverage result: %v\n", err)
		return 1
	}
	return 0
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
	if policy.Comparison != "strictly-greater-than" {
		return coveragePolicy{}, fmt.Errorf("comparison must be strictly-greater-than")
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
	return policy, nil
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
		if statements > math.MaxInt64-result.Total {
			return coverageResult{}, fmt.Errorf("line %d overflows statement total", line)
		}
		result.Total += statements
		packageCoverage := result.Packages[packageName]
		packageCoverage.Total += statements
		if count > 0 {
			result.Covered += statements
			packageCoverage.Covered += statements
		}
		result.Packages[packageName] = packageCoverage
	}
	if err := scanner.Err(); err != nil {
		return coverageResult{}, err
	}
	if result.Total == 0 {
		return coverageResult{}, fmt.Errorf("profile contains no statements")
	}
	return result, nil
}
