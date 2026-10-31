package main

import (
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/rogpeppe/go-internal/txtar"
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

// TestScriptDocumentsCarryTheContractHeader holds every JSON document that an
// acceptance script expects from `aigw ... --json` to the command-line
// contract's header: an integer schema_version and a boolean ok.
func TestScriptDocumentsCarryTheContractHeader(t *testing.T) {
	scripts, err := filepath.Glob(filepath.Join("testdata", "script", "*.txtar"))
	if err != nil || len(scripts) == 0 {
		t.Fatalf("acceptance scripts: %v, %d found", err, len(scripts))
	}
	documents := 0
	for _, path := range scripts {
		archive, err := txtar.ParseFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range scriptDocuments(string(archive.Comment)) {
			index := slices.IndexFunc(archive.Files, func(file txtar.File) bool { return file.Name == name })
			if index < 0 {
				t.Errorf("%s compares %s, which the archive does not hold", filepath.Base(path), name)
				continue
			}
			// normalize writes <varies> for a value that differs between runs.
			document := strings.ReplaceAll(string(archive.Files[index].Data), "<varies>", "0")
			for _, problem := range documentHeaderProblems([]byte(document)) {
				t.Errorf("%s: %s %s", filepath.Base(path), name, problem)
			}
			documents++
		}
	}
	if documents == 0 {
		t.Fatal("no acceptance script compares a JSON document")
	}
}

func TestDocumentHeaderProblemsNameEachGap(t *testing.T) {
	for _, test := range []struct {
		name, document string
		want           []string
	}{
		{"complete", `{"schema_version": 1, "ok": true}`, nil},
		{"no version", `{"ok": true}`, []string{"lacks an integer schema_version"}},
		{"text version", `{"schema_version": "1", "ok": false}`, []string{"lacks an integer schema_version"}},
		{"fractional version", `{"schema_version": 1.5, "ok": false}`, []string{"lacks an integer schema_version"}},
		{"no outcome", `{"schema_version": 1}`, []string{"lacks a boolean ok"}},
		{"text outcome", `{"schema_version": 1, "ok": "true"}`, []string{"lacks a boolean ok"}},
		{"null outcome", `{"schema_version": 1, "ok": null}`, []string{"lacks a boolean ok"}},
		{"null version", `{"schema_version": null, "ok": true}`, []string{"lacks an integer schema_version"}},
		{"not an object", `[1]`, []string{"is not a JSON object"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := documentHeaderProblems([]byte(test.document)); !slices.Equal(got, test.want) {
				t.Fatalf("documentHeaderProblems() = %q, want %q", got, test.want)
			}
		})
	}
	script := "exec aigw demo --json\ncmp stdout direct.json\n" +
		"[windows] exec $BIN/aigw demo --json=true\n[windows] cp stdout copied.out\n[windows] cmp copied.out copied.json\n" +
		"exec aigw demo --json\nnormalize stdout normalized.out\ncmpenv normalized.out normalized.json\n" +
		"exitcode 1 aigw demo --json\ncmpenv stdout refused.json\n" +
		"exec aigw demo\ncmp stdout text.json\n" +
		"exec aigw demo --json=false\ncmp stdout off.json\n" +
		"exec aigw demo --json\nexec claude aigw --json\ncmp stdout client.json\n"
	if got, want := scriptDocuments(script), []string{"direct.json", "copied.json", "normalized.json", "refused.json"}; !slices.Equal(got, want) {
		t.Fatalf("scriptDocuments() = %q, want %q", got, want)
	}
}

// scriptDocuments names the archive files that a script compares with the
// standard output of an `aigw ... --json` command, directly or through a copy
// that cp or normalize makes of it. Any later command replaces the standard
// output that cmp reads.
func scriptDocuments(script string) []string {
	var documents []string
	lastJSON := false
	fromJSON := map[string]bool{}
	for line := range strings.Lines(script) {
		words := strings.Fields(line)
		for len(words) > 0 && strings.HasPrefix(words[0], "[") && strings.HasSuffix(words[0], "]") {
			words = words[1:]
		}
		if len(words) > 0 && words[0] == "!" {
			words = words[1:]
		}
		switch {
		case len(words) == 0:
		case words[0] == "exec" || words[0] == "exitcode" || words[0] == "apikeyhelper":
			lastJSON = aigwJSON(words)
		case (words[0] == "normalize" || words[0] == "cp") && len(words) >= 3 && words[1] == "stdout":
			fromJSON[words[2]] = lastJSON
		case (words[0] == "cmp" || words[0] == "cmpenv") && len(words) == 3 && ((words[1] == "stdout" && lastJSON) || fromJSON[words[1]]):
			documents = append(documents, words[2])
		}
	}
	return documents
}

// aigwJSON reports whether a script command runs aigw with --json: `exec
// aigw ...` or `exitcode N aigw ...`, with the program named by any path.
func aigwJSON(words []string) bool {
	program := 1
	if words[0] == "exitcode" {
		program = 2
	}
	if words[0] == "apikeyhelper" || len(words) <= program || filepath.Base(words[program]) != "aigw" {
		return false
	}
	return slices.ContainsFunc(words[program+1:], func(word string) bool { return word == "--json" || word == "--json=true" })
}

// documentHeaderProblems names what a JSON document lacks of the contract's
// header.
func documentHeaderProblems(document []byte) []string {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(document, &fields); err != nil || fields == nil {
		return []string{"is not a JSON object"}
	}
	var problems []string
	var version any
	decoder := json.NewDecoder(strings.NewReader(string(fields["schema_version"])))
	decoder.UseNumber()
	number, isNumber := json.Number(""), false
	if decoder.Decode(&version) == nil {
		number, isNumber = version.(json.Number)
	}
	if _, err := strconv.ParseInt(string(number), 10, 64); !isNumber || err != nil {
		problems = append(problems, "lacks an integer schema_version")
	}
	var ok *bool
	if err := json.Unmarshal(fields["ok"], &ok); err != nil || ok == nil {
		problems = append(problems, "lacks a boolean ok")
	}
	return problems
}
