// Command modelcatalog verifies, against a real installed Codex client, that the
// model catalog AIGW projects makes a provider-prefixed model entry identical
// to the bundled entry whose slug it wraps.
//
// It exists because that claim cannot be proven against a fake client: the
// package tests pin the catalog's content and every decision around it, but only
// the client itself can show that it loaded the projected catalog. Run it when
// changing the catalog projection or when qualifying a new client build:
//
//	go run ./tools/modelcatalog -model openai.gpt-5.6-sol
//
// The client renders its effective model catalog through a throwaway client
// home, so the run makes no model request. Nothing outside a temporary directory
// is written, and the user's Codex configuration is neither read nor changed.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"text/tabwriter"

	"aigw-cli/internal/codex"
)

var verifyModelCatalog = codex.VerifyModelCatalog
var lookupExecutable = exec.LookPath
var writeReport = report

const (
	exitVerificationFailed = 1
	// exitPrerequisiteMissing separates "this machine cannot run the check" from
	// "the check ran and the claim is false". A missing client must never read as
	// a pass, and must not read as a defect either.
	exitPrerequisiteMissing = 2
)

func main() { os.Exit(execute(os.Args[1:], os.Stdout, os.Stderr)) }

func execute(args []string, out, errOut io.Writer) int {
	code, err := run(args, out)
	if err != nil {
		_, _ = fmt.Fprintf(errOut, "modelcatalog: %v\n", err)
	}
	return code
}

func run(args []string, out io.Writer) (int, error) {
	flags := flag.NewFlagSet("modelcatalog", flag.ContinueOnError)
	flags.SetOutput(out)
	model := flags.String("model", "", "provider-prefixed model id to verify, for example openai.gpt-5.6-sol")
	executable := flags.String("codex", "", "path to the Codex executable (default: codex from PATH)")
	asJSON := flags.Bool("json", false, "print the measurements as JSON")
	if err := flags.Parse(args); err != nil {
		return exitPrerequisiteMissing, err
	}
	if *model == "" {
		return exitPrerequisiteMissing, errors.New("-model is required, for example -model openai.gpt-5.6-sol")
	}
	resolved, err := resolveExecutable(*executable)
	if err != nil {
		return exitPrerequisiteMissing, err
	}
	verification, err := verifyModelCatalog(resolved, *model)
	if err != nil {
		return exitPrerequisiteMissing, err
	}
	if err := writeReport(out, verification, *asJSON); err != nil {
		return exitVerificationFailed, err
	}
	// The verdict is reported after the measurements, so a failure is readable
	// rather than only asserted.
	if err := verification.Check(); err != nil {
		return exitVerificationFailed, err
	}
	return 0, nil
}

// resolveExecutable prefers an explicitly named client and otherwise takes the
// one on PATH. A missing client is a prerequisite the caller has to satisfy, so
// it is named as such rather than reported as a failed verification.
func resolveExecutable(named string) (string, error) {
	if named != "" {
		if _, err := os.Stat(named); err != nil {
			return "", fmt.Errorf("prerequisite: Codex executable %q is not usable: %w", named, err)
		}
		return named, nil
	}
	path, err := lookupExecutable("codex")
	if err != nil {
		return "", fmt.Errorf("prerequisite: no Codex executable found on PATH; install the client or pass -codex <path>: %w", err)
	}
	return path, nil
}

func report(out io.Writer, verification codex.ModelCatalogVerification, asJSON bool) error {
	if asJSON {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(verification)
	}
	if _, err := fmt.Fprintf(out, "client   %s\nsha256   %s\nmodel    %s\nbase     %s\n\n", verification.ClientVersion, verification.ClientSHA256, verification.Model, verification.BaseSlug); err != nil {
		return err
	}
	var rendered bytes.Buffer
	table := tabwriter.NewWriter(&rendered, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(table, "selection\tmodel\tstate\tmetadata sha256")
	for _, row := range []struct {
		label string
		probe codex.ModelCatalogProbe
	}{
		{"base slug, client's own table", verification.Reference},
		{"prefixed, no catalog", verification.Unadapted},
		{"prefixed, generated catalog", verification.Adapted},
		{"unknown model, generated catalog", verification.Unknown},
	} {
		state := "absent"
		if row.probe.Present {
			state = "present"
		}
		_, _ = fmt.Fprintf(table, "%s\t%s\t%s\t%s\n", row.label, row.probe.Model, state, row.probe.MetadataSHA256)
	}
	_ = table.Flush()
	_, err := rendered.WriteTo(out)
	return err
}
