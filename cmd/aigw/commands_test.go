package main

import (
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestEveryCommandHasItsPackageAndScript holds the CLI to its command tree.
// Each top-level command runs only functions from the internal/cli package
// named after it, and checks its arguments and prepares its run through that
// package, the CLI's shared invocation package or cobra. The acceptance
// script named after the command holds every case that missingCases lists.
func TestEveryCommandHasItsPackageAndScript(t *testing.T) {
	root := cli.NewRoot(&cli.App{Clients: clients.Registry(), Out: io.Discard, Err: io.Discard})
	if len(root.Commands()) == 0 {
		t.Fatal("the root registers no command")
	}
	for _, command := range root.Commands() {
		name := command.Name()
		owner := "aigw-cli/internal/cli/" + name
		runs, hooks := commandFunctions(command)
		if len(runs) == 0 {
			t.Errorf("command %q has no run function", name)
		}
		for _, run := range runs {
			if got := functionPackage(run); got != owner {
				t.Errorf("command %q runs a function from %s instead of its package %s", name, got, owner)
			}
		}
		for _, hook := range hooks {
			switch got := functionPackage(hook); got {
			case owner, "aigw-cli/internal/cli/invocation", "github.com/spf13/cobra":
			default:
				t.Errorf("command %q checks its arguments or prepares its run through a function from %s", name, got)
			}
		}
		script, err := os.ReadFile(filepath.Join("testdata", "script", name+".txtar"))
		if err != nil {
			t.Errorf("command %q has no acceptance script: %v", name, err)
			continue
		}
		for _, missing := range missingCases(name, offersJSON(command), string(script)) {
			t.Errorf("the acceptance script of command %q lacks %s", name, missing)
		}
	}
}

func TestMissingCasesNamesEachAbsentCase(t *testing.T) {
	complete := "# A comment does not count: exec aigw demo\n" +
		"exec aigw demo --help\n" +
		"exec aigw demo run --json\n" +
		"exitcode 1 aigw demo run broken\n" +
		"exitcode 1 aigw demo run broken --json\n" +
		"-- expected.txt --\nexec aigw demo\n"
	for _, test := range []struct {
		name   string
		script string
		json   bool
		want   []string
	}{
		{"complete", complete, true, nil},
		{"no JSON", "exec aigw demo --help\nexec aigw demo run\nexitcode 1 aigw demo run broken\n", false, nil},
		{"empty", "", false, []string{"its help", "a success", "a refusal with its exit status"}},
		{"only conditional", "[!windows] exec aigw demo --help\n[windows] exec aigw demo run\n[darwin] exitcode 1 aigw demo broken\n", false,
			[]string{"its help", "a success", "a refusal with its exit status"}},
		{"unchecked refusal", "exec aigw demo --help\nexec aigw demo run\n! exec aigw demo broken\n", false, []string{"a refusal with its exit status"}},
		{"another command", "exec aigw other --help\nexec aigw other\nexitcode 1 aigw other broken\n", false,
			[]string{"its help", "a success", "a refusal with its exit status"}},
		{"help is no success", "exec aigw demo --help\nexitcode 1 aigw demo broken\n", false, []string{"a success"}},
		{"no JSON refusal", "exec aigw demo --help\nexec aigw demo --json\nexitcode 1 aigw demo broken\n", true, []string{"a JSON refusal"}},
		{"no JSON success", "exec aigw demo --help\nexec aigw demo\nexitcode 1 aigw demo broken --json\n", true, []string{"a JSON success"}},
		{"after the archive", "-- note.txt --\nexec aigw demo --help\nexec aigw demo\nexitcode 1 aigw demo broken\n", false,
			[]string{"its help", "a success", "a refusal with its exit status"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := missingCases("demo", test.json, test.script); !slices.Equal(got, test.want) {
				t.Fatalf("missingCases() = %q, want %q", got, test.want)
			}
		})
	}
}

// missingCases names the cases that a command's acceptance script lacks: its
// help, a success and a refusal with its exit status, and, for a command that
// offers --json, a JSON success and a JSON refusal. Only a line outside any
// platform condition counts. A refusal runs through exitcode, which checks the
// exit status. The credential command succeeds when apikeyhelper runs the
// helper that AIGW projects into Claude Code, which runs `aigw credential`.
func missingCases(name string, json bool, script string) []string {
	var help, success, refusal, jsonSuccess, jsonRefusal bool
	commands, _, _ := strings.Cut("\n"+script, "\n-- ")
	for line := range strings.SplitSeq(commands, "\n") {
		words := strings.Fields(line)
		status := 0
		switch {
		case len(words) > 2 && words[0] == "exec" && words[1] == "aigw":
			words = words[2:]
		case len(words) > 3 && words[0] == "exitcode" && words[2] == "aigw":
			code, err := strconv.Atoi(words[1])
			if err != nil {
				continue
			}
			status, words = code, words[3:]
		case len(words) == 2 && words[0] == "apikeyhelper" && name == "credential":
			success = true
			continue
		default:
			continue
		}
		if words[0] != name {
			continue
		}
		withJSON := slices.Contains(words, "--json")
		switch {
		case slices.Contains(words, "--help"):
			help = true
		case status == 0:
			success, jsonSuccess = true, jsonSuccess || withJSON
		default:
			refusal, jsonRefusal = true, jsonRefusal || withJSON
		}
	}
	var missing []string
	for _, item := range []struct {
		present bool
		name    string
	}{
		{help, "its help"},
		{success, "a success"},
		{refusal, "a refusal with its exit status"},
		{jsonSuccess || !json, "a JSON success"},
		{jsonRefusal || !json, "a JSON refusal"},
	} {
		if !item.present {
			missing = append(missing, item.name)
		}
	}
	return missing
}

// offersJSON reports whether command or one of its subcommands has --json.
func offersJSON(command *cobra.Command) bool {
	if command.Flags().Lookup("json") != nil {
		return true
	}
	return slices.ContainsFunc(command.Commands(), offersJSON)
}

// commandFunctions returns the run functions of command and its subcommands,
// and the functions that check their arguments or run before or after them.
func commandFunctions(command *cobra.Command) (runs, hooks []any) {
	for _, run := range []any{command.RunE, command.Run} {
		if !reflect.ValueOf(run).IsNil() {
			runs = append(runs, run)
		}
	}
	for _, hook := range []any{
		command.Args, command.PreRunE, command.PreRun, command.PostRunE, command.PostRun,
		command.PersistentPreRunE, command.PersistentPreRun, command.PersistentPostRunE, command.PersistentPostRun,
	} {
		if !reflect.ValueOf(hook).IsNil() {
			hooks = append(hooks, hook)
		}
	}
	for _, child := range command.Commands() {
		childRuns, childHooks := commandFunctions(child)
		runs, hooks = append(runs, childRuns...), append(hooks, childHooks...)
	}
	return runs, hooks
}

// functionPackage returns the import path of the package that defines
// function.
func functionPackage(function any) string {
	name := runtime.FuncForPC(reflect.ValueOf(function).Pointer()).Name()
	slash := strings.LastIndex(name, "/") + 1
	if dot := strings.Index(name[slash:], "."); dot >= 0 {
		return name[:slash+dot]
	}
	return name
}
