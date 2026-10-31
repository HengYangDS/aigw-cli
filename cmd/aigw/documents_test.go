package main

import (
	"encoding/json"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/rogpeppe/go-internal/txtar"
)

// TestScriptDocumentsFollowTheContract holds every JSON document that an
// acceptance script expects from `aigw ... --json` to the command-line
// contract: an integer schema_version and a boolean ok, snake_case keys, no
// null list, a next_action that is a runnable aigw command, and dry_run: true
// in a preview's plan alone.
func TestScriptDocumentsFollowTheContract(t *testing.T) {
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
		for _, compared := range scriptDocuments(string(archive.Comment)) {
			name := compared.name
			index := slices.IndexFunc(archive.Files, func(file txtar.File) bool { return file.Name == name })
			if index < 0 {
				t.Errorf("%s compares %s, which the archive does not hold", filepath.Base(path), name)
				continue
			}
			// normalize writes <varies> for a value that differs between runs.
			document := []byte(strings.ReplaceAll(string(archive.Files[index].Data), "<varies>", "0"))
			for _, problem := range append(documentProblems(document), dryRunProblems(document, compared.preview)...) {
				t.Errorf("%s: %s %s", filepath.Base(path), name, problem)
			}
			documents++
		}
	}
	if documents == 0 {
		t.Fatal("no acceptance script compares a JSON document")
	}
}

func TestDocumentProblemsNameEachGap(t *testing.T) {
	for _, test := range []struct {
		name, document string
		want           []string
	}{
		{"complete", `{"schema_version": 1, "ok": true}`, nil},
		{"identifier keys", `{"schema_version": 2, "ok": true, "clients": {"claude-desktop": {"next_action": "aigw check"}}}`, nil},
		{"identifier-keyed list", `{"schema_version": 2, "ok": true, "routes": [{"id": "a-b", "routeLabel": "x"}]}`, []string{"has the key routes.0.routeLabel, which is not snake_case"}},
		{"camel key", `{"schema_version": 2, "ok": true, "nextAction": "aigw check"}`, []string{"has the key nextAction, which is not snake_case"}},
		{"null list", `{"schema_version": 2, "ok": true, "projected_clients": null}`, []string{"has null at projected_clients"}},
		{"null rollback", `{"schema_version": 1, "ok": true, "rollback": null}`, nil},
		{"prose next", `{"schema_version": 2, "ok": false, "next_action": "Restart Codex"}`, []string{"has a next_action that is not a runnable aigw command: Restart Codex"}},
		{"placeholder next", `{"schema_version": 2, "ok": false, "checks": [{"next_action": "aigw use --for codex <route>"}]}`, []string{"has a next_action that is not a runnable aigw command: aigw use --for codex <route>"}},
		{"prompting next", `{"schema_version": 2, "ok": false, "next_action": "aigw setup"}`, []string{"has a next_action that is not a runnable aigw command: aigw setup"}},
		{"token prompt next", `{"schema_version": 2, "ok": false, "next_action": "aigw rotate team"}`, []string{"has a next_action that is not a runnable aigw command: aigw rotate team"}},
		{"token input next", `{"schema_version": 2, "ok": false, "next_action": "aigw rotate team --token-stdin"}`, nil},
		{"defaulted account next", `{"schema_version": 2, "ok": false, "next_action": "aigw setup --route team --for claude"}`, []string{"has a next_action that is not a runnable aigw command: aigw setup --route team --for claude"}},
		{"bare diagnostics next", `{"schema_version": 2, "ok": false, "next_action": "aigw account diagnostics enable"}`, []string{"has a next_action that is not a runnable aigw command: aigw account diagnostics enable"}},
		{"import next", `{"schema_version": 2, "ok": true, "next_action": "aigw setup --from team.toml"}`, nil},
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
			if got := documentProblems([]byte(test.document)); !slices.Equal(got, test.want) {
				t.Fatalf("documentProblems() = %q, want %q", got, test.want)
			}
		})
	}
	script := "exec aigw demo --json\ncmp stdout direct.json\n" +
		"[windows] exec $BIN/aigw demo --json=true\n[windows] cp stdout copied.out\n[windows] cmp copied.out copied.json\n" +
		"exec aigw demo --json\nnormalize stdout normalized.out\ncmpenv normalized.out normalized.json\n" +
		"exitcode 1 aigw demo --json\ncmpenv stdout refused.json\n" +
		"exec aigw demo\ncmp stdout text.json\n" +
		"exec aigw demo --json=false\ncmp stdout off.json\n" +
		"exec aigw demo --json\nexec claude aigw --json\ncmp stdout client.json\n" +
		"exec aigw demo --dry-run --json\ncmp stdout preview.json\n"
	want := []scriptDocument{{"direct.json", false}, {"copied.json", false}, {"normalized.json", false}, {"refused.json", false}, {"preview.json", true}}
	if got := scriptDocuments(script); !slices.Equal(got, want) {
		t.Fatalf("scriptDocuments() = %v, want %v", got, want)
	}
	for _, test := range []struct {
		name, document string
		preview        bool
		want           []string
	}{
		{"preview", `{"schema_version": 2, "ok": true, "dry_run": true}`, true, nil},
		{"preview without dry_run", `{"schema_version": 2, "ok": false, "status": "blocked"}`, true, []string{"is a preview without dry_run: true"}},
		{"refused preview", `{"schema_version": 2, "ok": false, "error": "refused"}`, true, nil},
		{"result", `{"schema_version": 2, "ok": true}`, false, nil},
		{"result with dry_run", `{"schema_version": 2, "ok": true, "dry_run": false}`, false, []string{"has dry_run outside a preview"}},
	} {
		if got := dryRunProblems([]byte(test.document), test.preview); !slices.Equal(got, test.want) {
			t.Errorf("%s: dryRunProblems() = %q, want %q", test.name, got, test.want)
		}
	}
}

// scriptDocument is an archive file that a script compares with a JSON
// document, and whether the command that wrote it was a preview.
type scriptDocument struct {
	name    string
	preview bool
}

// dryRunProblems holds a document to the preview rule: a preview's plan says
// dry_run: true, and no other document carries dry_run. A preview refused
// before its plan writes the failure document, which has an error instead.
func dryRunProblems(document []byte, preview bool) []string {
	var fields map[string]any
	if json.Unmarshal(document, &fields) != nil {
		return nil
	}
	value, present := fields["dry_run"]
	_, refused := fields["error"]
	switch {
	case preview && !refused && value != true:
		return []string{"is a preview without dry_run: true"}
	case !preview && present:
		return []string{"has dry_run outside a preview"}
	}
	return nil
}

// scriptDocuments names the archive files that a script compares with the
// standard output of an `aigw ... --json` command, directly or through a copy
// that cp or normalize makes of it, and whether that command was a preview.
// Any later command replaces the standard output that cmp reads.
func scriptDocuments(script string) []scriptDocument {
	var documents []scriptDocument
	// Each file, and standard output after the last command, records whether
	// an aigw --json command wrote it and whether that command was a preview.
	fromJSON, fromPreview := map[string]bool{}, map[string]bool{}
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
			fromJSON["stdout"] = aigwJSON(words)
			fromPreview["stdout"] = fromJSON["stdout"] && slices.ContainsFunc(words, func(word string) bool { return word == "--dry-run" || word == "--dry-run=true" })
		case (words[0] == "normalize" || words[0] == "cp") && len(words) >= 3 && words[1] == "stdout":
			fromJSON[words[2]], fromPreview[words[2]] = fromJSON["stdout"], fromPreview["stdout"]
		case (words[0] == "cmp" || words[0] == "cmpenv") && len(words) == 3 && fromJSON[words[1]]:
			documents = append(documents, scriptDocument{words[2], fromPreview[words[1]]})
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

// documentProblems names where a JSON document departs from the contract:
// its header, its keys, a null value and its next actions.
func documentProblems(document []byte) []string {
	problems := documentHeaderProblems(document)
	var value any
	if err := json.Unmarshal(document, &value); err == nil {
		problems = append(problems, documentValueProblems("", "", value)...)
	}
	return problems
}

// identifierKeyed names the objects whose keys are identifiers, such as client
// IDs, rather than field names.
var identifierKeyed = map[string]bool{"clients": true, "selections": true, "adopted_selections": true, "recommendations": true, "accounts": true, "routes": true}

// snakeKey matches a snake_case field name.
var snakeKey = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// documentValueProblems names the non-snake_case keys, the null values other
// than a missing rollback copy and the header's, which documentHeaderProblems
// names, and the next actions that a script cannot run as they stand, below
// path. key is the name under which value appears, empty for a list item.
func documentValueProblems(path, key string, value any) []string {
	var problems []string
	join := func(name string) string {
		if path == "" {
			return name
		}
		return path + "." + name
	}
	switch typed := value.(type) {
	case nil:
		if key != "rollback" && path != "schema_version" && path != "ok" {
			problems = append(problems, "has null at "+path)
		}
	case map[string]any:
		for _, name := range slices.Sorted(maps.Keys(typed)) {
			if !identifierKeyed[key] && !snakeKey.MatchString(name) {
				problems = append(problems, "has the key "+join(name)+", which is not snake_case")
			}
			if command, ok := typed[name].(string); ok && name == "next_action" && !runnable(command) {
				problems = append(problems, "has a next_action that is not a runnable aigw command: "+command)
			}
			problems = append(problems, documentValueProblems(join(name), name, typed[name])...)
		}
	case []any:
		for index, item := range typed {
			problems = append(problems, documentValueProblems(join(strconv.Itoa(index)), "", item)...)
		}
	}
	return problems
}

// runnable reports whether a script can run command as it stands: an aigw
// command without a placeholder or prose that asks no person for a value.
// Setup asks for an Account's Token unless it only imports a manifest, and
// add, rotate and account diagnostics enable ask for their secret unless
// their standard-input flag supplies it; help asks for nothing.
func runnable(command string) bool {
	words := strings.Fields(command)
	if len(words) < 2 || words[0] != "aigw" || strings.ContainsAny(command, "<>`;,") {
		return false
	}
	has := func(word string) bool { return slices.Contains(words, word) }
	switch {
	case has("--help") || has("-h"):
		return true
	case words[1] == "setup":
		return has("--token-stdin") || has("--from") && !has("--account")
	case words[1] == "add" || words[1] == "rotate":
		return has("--token-stdin")
	case len(words) >= 4 && words[1] == "account" && words[2] == "diagnostics" && words[3] == "enable":
		return has("--system-token-stdin")
	}
	return true
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
