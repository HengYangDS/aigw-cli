package main

import (
	"bytes"
	"encoding/json"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"

	"github.com/rogpeppe/go-internal/txtar"
	"github.com/spf13/cobra"
)

// retiredWord matches the implementation's nouns that provider, model and
// client replaced, which help, human output and JSON keys no longer use, nor
// any environment-variable name that they show.
var retiredWord = regexp.MustCompile(`(?i)\b(accounts?|routes?|projections?|bindings?|adapters?|admitted|candidates?)\b`)

// keychainLabel is the label of the macOS Keychain item that holds a Token,
// which keeps its spelling because credential storage does not change.
var keychainLabel = regexp.MustCompile(`AIGW Account Token`)

// retiredIn returns the first retired word in human text. An underscore
// separates the words of an environment-variable name, which, unlike a
// Keychain item label, follows the product's vocabulary.
func retiredIn(text string) string {
	return retiredWord.FindString(strings.ReplaceAll(keychainLabel.ReplaceAllString(text, ""), "_", " "))
}

// TestHelpSpeaksTheProductVocabulary renders the help of every command,
// hidden ones included, and fails on a retired word.
func TestHelpSpeaksTheProductVocabulary(t *testing.T) {
	setAIGWTestEnvironment(t)
	var visit func(*cobra.Command)
	visit = func(command *cobra.Command) {
		args := append(strings.Fields(command.CommandPath())[1:], "--help")
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 0 {
			t.Fatalf("aigw %s exited %d: %s", strings.Join(args, " "), code, &stderr)
		}
		if word := retiredIn(stdout.String()); word != "" {
			t.Errorf("aigw %s names %q:\n%s", strings.Join(args, " "), word, &stdout)
		}
		for _, child := range command.Commands() {
			visit(child)
		}
	}
	visit(cli.NewRoot(&cli.App{Clients: clients.Registry()}))
}

// TestScriptGoldensSpeakTheProductVocabulary reads every file that a script
// compares with what aigw wrote to standard output or error. Human output
// must not name a retired word, nor a JSON document have a key that does. A
// team file is the configuration's format, which changes with the
// configuration file, so a script's expected TOML is not read.
func TestScriptGoldensSpeakTheProductVocabulary(t *testing.T) {
	scripts, err := filepath.Glob(filepath.Join("testdata", "script", "*.txtar"))
	if err != nil || len(scripts) == 0 {
		t.Fatalf("scripts: %v, %v", scripts, err)
	}
	read := 0
	for _, script := range scripts {
		archive, err := txtar.ParseFile(script)
		if err != nil {
			t.Fatal(err)
		}
		files := map[string][]byte{}
		for _, file := range archive.Files {
			files[file.Name] = file.Data
		}
		for _, name := range outputGoldens(string(archive.Comment)) {
			data, found := files[name]
			if !found || strings.HasSuffix(name, ".toml") {
				continue
			}
			read++
			var document any
			if json.Unmarshal(data, &document) == nil {
				if key := retiredKey(document); key != "" {
					t.Errorf("%s: %s has the JSON key %q", script, name, key)
				}
			} else if word := retiredIn(string(data)); word != "" {
				t.Errorf("%s: %s names %q", script, name, word)
			}
		}
	}
	if read == 0 {
		t.Fatal("no script compares its output with an expected file")
	}
}

// outputGoldens returns the files that a script compares with aigw's output:
// with stdout or stderr, or with a file that it wrote from one of them.
func outputGoldens(script string) []string {
	written := map[string]bool{"stdout": true, "stderr": true}
	var goldens []string
	for line := range strings.SplitSeq(script, "\n") {
		words := strings.Fields(line)
		for len(words) > 0 && (strings.HasPrefix(words[0], "[") || words[0] == "!") {
			words = words[1:]
		}
		if len(words) < 3 {
			continue
		}
		switch words[0] {
		case "normalize", "cp":
			if words[1] == "stdout" || words[1] == "stderr" {
				written[words[2]] = true
			}
		case "cmp", "cmpenv":
			if written[words[1]] {
				goldens = append(goldens, words[2])
			}
		}
	}
	return goldens
}

// retiredKey returns the first key of a JSON value, at any depth, that names
// a retired word; an underscore separates a key's words.
func retiredKey(value any) string {
	switch value := value.(type) {
	case map[string]any:
		for _, key := range slices.Sorted(maps.Keys(value)) {
			if retiredWord.MatchString(strings.ReplaceAll(key, "_", " ")) {
				return key
			}
			if found := retiredKey(value[key]); found != "" {
				return found
			}
		}
	case []any:
		for _, item := range value {
			if found := retiredKey(item); found != "" {
				return found
			}
		}
	}
	return ""
}

func TestOutputGoldensAreTheFilesComparedWithOutput(t *testing.T) {
	script := "exec aigw status\ncmp stdout status.txt\n[windows] cmp stdout status-windows.txt\n" +
		"! exec aigw check --json\nnormalize stdout check.out\ncmp check.out check.json\n" +
		"cmp $HOME/.claude/settings.json settings.json\nsnapshot $HOME after.txt\ncmp after.txt before.txt\n"
	if got := outputGoldens(script); !slices.Equal(got, []string{"status.txt", "status-windows.txt", "check.json"}) {
		t.Fatalf("goldens = %q", got)
	}
	if key := retiredKey(map[string]any{"clients": []any{map[string]any{"projection_deferred": true}}}); key != "projection_deferred" {
		t.Fatalf("retired key = %q", key)
	}
	if word := retiredIn("AIGW Account Token: team"); word != "" {
		t.Fatalf("an exempt spelling matched %q", word)
	}
	if word := retiredIn("Set AIGW_ACCOUNT_TOKEN_TEAM"); word != "ACCOUNT" {
		t.Fatalf("a retired word in an environment-variable name matched %q", word)
	}
}
