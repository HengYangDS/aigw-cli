package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// exitFunc is swapped in tests so main itself stays coverage-reachable
// without terminating the test process.
var exitFunc = os.Exit

func main() {
	exitFunc(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("architecture", flag.ContinueOnError)
	flags.SetOutput(stderr)
	policyPath := flags.String("policy", defaultPolicyPath, "architecture policy TOML")
	root := flags.String("root", ".", "repository root to scan")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "architecture gate does not accept positional arguments")
		return 2
	}

	absRoot, err := filepath.Abs(*root)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "resolve root: %v\n", err)
		return 1
	}
	resolvedPolicy := *policyPath
	if !filepath.IsAbs(resolvedPolicy) {
		resolvedPolicy = filepath.Join(absRoot, resolvedPolicy)
	}
	pol, err := loadPolicy(resolvedPolicy)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load architecture policy: %v\n", err)
		return 1
	}

	// Emit policy-relative path as declared when under root; keep stable POSIX form.
	reportPolicyPath := toPOSIX(*policyPath)
	if filepath.IsAbs(*policyPath) {
		if rel, relErr := filepath.Rel(absRoot, resolvedPolicy); relErr == nil && !filepath.IsAbs(rel) && !startsWithDotDot(rel) {
			reportPolicyPath = toPOSIX(rel)
		} else {
			reportPolicyPath = toPOSIX(resolvedPolicy)
		}
	}

	report, err := analyzeRepository(absRoot, pol, reportPolicyPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "analyze repository: %v\n", err)
		return 1
	}
	if err := writeReport(stdout, report); err != nil {
		_, _ = fmt.Fprintf(stderr, "write report: %v\n", err)
		return 1
	}
	if !report.OK {
		return 1
	}
	return 0
}

func startsWithDotDot(rel string) bool {
	return rel == ".." || len(rel) >= 3 && (rel[:3] == ".."+string(filepath.Separator) || rel[:3] == "../")
}
