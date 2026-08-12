package main

import (
	"bufio"
	"bytes"
	"encoding/xml"
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
	MinimumBranchPercent    float64  `toml:"minimum_branch_percent"`
	Comparison              string   `toml:"comparison"`
	CoverMode               string   `toml:"covermode"`
	Packages                []string `toml:"packages"`
	BranchAnalyzer          string   `toml:"branch_analyzer"`
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

type inputCommandRunner interface {
	RunInput(name string, args []string, stdin io.Reader, stdout, stderr io.Writer) error
}

type systemRunner struct{}

func (systemRunner) Run(name string, args []string, stdout, stderr io.Writer) error {
	command := exec.Command(name, args...)
	command.Stdout = stdout
	command.Stderr = stderr
	return command.Run()
}

func (systemRunner) RunInput(name string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	command := exec.Command(name, args...)
	command.Stdin = stdin
	command.Stdout = stdout
	command.Stderr = stderr
	return command.Run()
}

type packageInfo struct {
	ImportPath string
	ModulePath string
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
	flags := flag.NewFlagSet("coverage", flag.ContinueOnError)
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
	// Pure acceptance-test packages intentionally own no production statements.
	// They still run and contribute cross-package coverage, but only packages
	// with production Go files participate in the per-package floor.
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
	if len(expectedPackages) == 0 {
		_, _ = fmt.Fprintln(stderr, "go list returned no packages")
		return 1
	}
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
	// Run the module once. coverpkg instruments every source package so
	// black-box and acceptance tests are credited to the code they execute;
	// it does not repeat the suite package by package.
	coverPackages := strings.Join(policy.Packages, ",")
	goArgs = append(goArgs, "-covermode="+policy.CoverMode, "-coverpkg="+coverPackages, "-coverprofile="+profilePath)
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
	for _, pkg := range expectedPackages {
		name := pkg.ImportPath
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
	if err := runBranchCoverage(profilePath, expectedPackages, policy, stdout, stderr, runner); err != nil {
		_, _ = fmt.Fprintf(stderr, "branch coverage failed: %v\n", err)
		return 1
	}
	if _, err := fmt.Fprintf(stdout, "statement coverage: %.2f%% (%d/%d statements), required > %.2f%%\n", percent, result.Covered, result.Total, policy.MinimumStatementPercent); err != nil {
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
	if policy.MinimumBranchPercent <= 0 || policy.MinimumBranchPercent >= 100 {
		return coveragePolicy{}, fmt.Errorf("minimum_branch_percent must be greater than 0 and less than 100")
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
	if policy.BranchAnalyzer != "go-bcov" {
		return coveragePolicy{}, fmt.Errorf("branch_analyzer must be go-bcov")
	}
	if strings.TrimSpace(policy.Owner) == "" || strings.TrimSpace(policy.Source) == "" {
		return coveragePolicy{}, fmt.Errorf("owner and source must be non-empty")
	}
	return policy, nil
}

func parsePackageList(output string) ([]packageInfo, error) {
	seen := map[string]bool{}
	var packages []packageInfo
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 2 || fields[0] == "" || fields[1] == "" {
			return nil, fmt.Errorf("package row must contain import and module paths")
		}
		pkg := packageInfo{ImportPath: fields[0], ModulePath: fields[1]}
		if pkg.ImportPath != pkg.ModulePath && !strings.HasPrefix(pkg.ImportPath, pkg.ModulePath+"/") {
			return nil, fmt.Errorf("package %q is outside module %q", pkg.ImportPath, pkg.ModulePath)
		}
		if seen[pkg.ImportPath] {
			continue
		}
		seen[pkg.ImportPath] = true
		packages = append(packages, pkg)
	}
	sort.Slice(packages, func(i, j int) bool { return packages[i].ImportPath < packages[j].ImportPath })
	return packages, nil
}

type branchXML struct {
	Files []branchFile `xml:"file"`
}

type branchFile struct {
	Path  string       `xml:"path,attr"`
	Lines []branchLine `xml:"lineToCover"`
}

type branchLine struct {
	Covered int64 `xml:"coveredBranches,attr"`
	Total   int64 `xml:"branchesToCover,attr"`
}

func runBranchCoverage(profilePath string, packages []packageInfo, policy coveragePolicy, stdout, stderr io.Writer, runner commandRunner) error {
	inputRunner, ok := runner.(inputCommandRunner)
	if !ok {
		return fmt.Errorf("branch analyzer runner does not support standard input")
	}
	profile, err := os.Open(profilePath)
	if err != nil {
		return fmt.Errorf("open coverage profile for branch analysis: %w", err)
	}
	defer func() { _ = profile.Close() }()
	var output bytes.Buffer
	if err := inputRunner.RunInput("go", []string{"tool", policy.BranchAnalyzer, "-format", "sonar-cover-report"}, profile, &output, stderr); err != nil {
		return fmt.Errorf("analyzer execution: %w", err)
	}
	counts, err := parseBranchReport(output.Bytes(), packages)
	if err != nil {
		return err
	}
	var aggregate coverageCount
	failed := false
	for _, pkg := range packages {
		count := counts[pkg.ImportPath]
		percent := branchPercent(count)
		if count.Total > 0 && percent <= policy.MinimumBranchPercent {
			_, _ = fmt.Fprintf(stderr, "package %s branch coverage %.2f%% does not exceed %.2f%% (%d/%d branches)\n", pkg.ImportPath, percent, policy.MinimumBranchPercent, count.Covered, count.Total)
			failed = true
		}
		aggregate.Covered += count.Covered
		aggregate.Total += count.Total
		_, _ = fmt.Fprintf(stdout, "package %s branch coverage: %.2f%% (%d/%d branches)\n", pkg.ImportPath, percent, count.Covered, count.Total)
	}
	percent := branchPercent(aggregate)
	if aggregate.Total == 0 || percent <= policy.MinimumBranchPercent {
		_, _ = fmt.Fprintf(stderr, "aggregate branch coverage %.2f%% does not exceed %.2f%% (%d/%d branches)\n", percent, policy.MinimumBranchPercent, aggregate.Covered, aggregate.Total)
		failed = true
	}
	if failed {
		return fmt.Errorf("branch coverage policy is not satisfied")
	}
	_, _ = fmt.Fprintf(stdout, "branch coverage: %.2f%% (%d/%d branches), required > %.2f%%\n", percent, aggregate.Covered, aggregate.Total, policy.MinimumBranchPercent)
	return nil
}

func parseBranchReport(data []byte, packages []packageInfo) (map[string]coverageCount, error) {
	var report branchXML
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = true
	if err := decoder.Decode(&report); err != nil {
		return nil, fmt.Errorf("decode branch report: %w", err)
	}
	counts := make(map[string]coverageCount, len(packages))
	pathOwners := make(map[string]string, len(packages))
	seenPackages := make(map[string]bool, len(packages))
	for _, pkg := range packages {
		counts[pkg.ImportPath] = coverageCount{}
		relative := strings.TrimPrefix(pkg.ImportPath, pkg.ModulePath)
		relative = strings.TrimPrefix(relative, "/")
		pathOwners[relative] = pkg.ImportPath
	}
	seenFiles := map[string]bool{}
	for _, file := range report.Files {
		relative := strings.ReplaceAll(file.Path, `\`, "/")
		relative = pathpkg.Clean(relative)
		if relative == "." || pathpkg.IsAbs(relative) || strings.HasPrefix(relative, "../") || strings.Contains(relative, ":") || seenFiles[relative] {
			return nil, fmt.Errorf("branch report contains invalid or repeated file path %q", file.Path)
		}
		seenFiles[relative] = true
		owner, ok := pathOwners[pathpkg.Dir(relative)]
		if !ok && pathpkg.Dir(relative) == "." {
			owner, ok = pathOwners[""]
		}
		if !ok {
			return nil, fmt.Errorf("branch report file %q has no listed package owner", file.Path)
		}
		count := counts[owner]
		for _, line := range file.Lines {
			if line.Total < 0 || line.Covered < 0 || line.Covered > line.Total {
				return nil, fmt.Errorf("branch report file %q has invalid branch counts", file.Path)
			}
			count.Covered += line.Covered
			count.Total += line.Total
		}
		seenPackages[owner] = true
		counts[owner] = count
	}
	for _, pkg := range packages {
		if !seenPackages[pkg.ImportPath] {
			return nil, fmt.Errorf("package %q is absent from the branch report", pkg.ImportPath)
		}
	}
	return counts, nil
}

func branchPercent(count coverageCount) float64 {
	if count.Total == 0 {
		return 100
	}
	return count.Percent()
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
