// Command catalogcheck verifies, against a real installed Codex client, that
// the entry which describes a selected model is in the catalog that the client
// renders, carries the model's declared context window in both window fields,
// and otherwise has the metadata of the client's own bundled entry: its
// family's for a model that the client's table does not hold, whose alias must
// also be shown under a name that no other entry has, and the model's own for
// a model that the table holds. A model takes metadata only from its own entry
// or its family's, never from a prefix or suffix of its ID, so -family is
// required; -name is the display name that AIGW gives an alias, the model's
// label without its provider; -context-window is the model's declared window,
// and without it the window of a model that declares none.
//
// It exists because that claim cannot be proven against a fake client: the
// package tests pin the catalog's content and every decision around it, but only
// the client itself can show that it loaded the projected catalog. Run it when
// changing the catalog projection or when qualifying a new client build, for an
// alias and for a model that is itself an entry of the client's table:
//
//	go run ./tools/codex/catalogcheck -model gpt-6.1-sol-cdx -family gpt-6.1-sol -name 'GPT-6.1 Sol · cdx'
//	go run ./tools/codex/catalogcheck -model gpt-6.1-sol -family gpt-6.1-sol -name 'GPT-6.1 Sol' -context-window 500000
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
	"strings"
	"text/tabwriter"

	"aigw-cli/internal/domain"
)

var verifyModelCatalog = verifyCatalog
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
		_, _ = fmt.Fprintf(errOut, "codex catalog: %v\n", err)
	}
	return code
}

func run(args []string, out io.Writer) (int, error) {
	flags := flag.NewFlagSet("codex-catalog", flag.ContinueOnError)
	flags.SetOutput(out)
	model := flags.String("model", "", "exact Codex model id to verify")
	family := flags.String("family", "", "the model's family: the Codex model whose bundled metadata it takes")
	name := flags.String("name", "", "the display name that AIGW gives the alias, the model's label without its provider")
	window := flags.Int("context-window", domain.DefaultContextWindow, "the model's declared context window in tokens, which its entry must carry")
	executable := flags.String("codex", "", "path to the Codex executable (default: codex from PATH)")
	asJSON := flags.Bool("json", false, "print the measurements as JSON")
	if err := flags.Parse(args); err != nil {
		return exitPrerequisiteMissing, err
	}
	if *model == "" {
		return exitPrerequisiteMissing, errors.New("-model is required")
	}
	if *family == "" {
		return exitPrerequisiteMissing, errors.New("-family is required: a model takes Codex metadata only from its family")
	}
	if *name == "" {
		return exitPrerequisiteMissing, errors.New("-name is required: the display name that AIGW gives the alias, the model's label without its provider")
	}
	if err := domain.ValidateContextWindow(*window); err != nil {
		return exitPrerequisiteMissing, fmt.Errorf("-context-window: %w", err)
	}
	resolved, err := resolveExecutable(*executable)
	if err != nil {
		return exitPrerequisiteMissing, err
	}
	verification, err := verifyModelCatalog(resolved, *model, *family, *name, *window)
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

func report(out io.Writer, verification verificationResult, asJSON bool) error {
	if asJSON {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(verification)
	}
	if _, err := fmt.Fprintf(out, "client   %s\nsha256   %s\nmodel    %s\nbase     %s\nwindow   declared %d; rendered context_window %d, max_context_window %d\n\n",
		verification.ClientVersion, verification.ClientSHA256, verification.Model, verification.BaseSlug,
		verification.DeclaredWindow, verification.ContextWindow, verification.MaxContextWindow); err != nil {
		return err
	}
	var rendered bytes.Buffer
	table := tabwriter.NewWriter(&rendered, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(table, "selection\tmodel\tstate\tmetadata sha256")
	for _, row := range []struct {
		label string
		probe modelProbe
	}{
		{"base, client's own table", verification.Reference},
		{"model, no catalog", verification.Unadapted},
		{"model, generated catalog", verification.Adapted},
		{"unknown model, generated catalog", verification.Unknown},
	} {
		state := "absent"
		if row.probe.Present {
			state = "present"
		}
		_, _ = fmt.Fprintf(table, "%s\t%s\t%s\t%s\n", row.label, row.probe.Model, state, row.probe.MetadataSHA256)
	}
	_ = table.Flush()
	if _, err := rendered.WriteTo(out); err != nil {
		return err
	}
	shared := "no other entry"
	if len(verification.SharedName) > 0 {
		shared = strings.Join(verification.SharedName, ", ")
	}
	_, err := fmt.Fprintf(out, "\nname shared with  %s\n", shared)
	return err
}
