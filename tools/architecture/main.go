// Package main verifies AIGW's repository architecture and ownership policy.
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
	root := flags.String("root", ".", "repository root to scan")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "architecture gate does not accept positional arguments")
		return 2
	}

	absRoot, _ := filepath.Abs(*root)
	pol, err := loadPolicy(filepath.Join(absRoot, filepath.FromSlash(policyPath)))
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load architecture policy: %v\n", err)
		return 1
	}

	report, err := analyzeRepository(absRoot, pol)
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
