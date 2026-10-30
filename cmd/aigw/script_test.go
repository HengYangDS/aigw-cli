package main

import (
	"encoding/json"
	"flag"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

var updateScripts = flag.Bool("update-scripts", false, "rewrite each acceptance script's expected output")

func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){"aigw": main, "claude": scriptedClaude})
}

// TestScripts runs every acceptance script against the program in a private
// home. PATH holds only the program and the scripted clients, and $UPSTREAM
// is a scripted provider.
func TestScripts(t *testing.T) {
	program, err := exec.LookPath("aigw")
	if err != nil {
		t.Fatal(err)
	}
	testscript.Run(t, testscript.Params{
		Dir:                 filepath.Join("testdata", "script"),
		RequireExplicitExec: true,
		RequireUniqueNames:  true,
		UpdateScripts:       *updateScripts,
		Setup: func(env *testscript.Env) error {
			home := filepath.Join(env.WorkDir, "home")
			for _, name := range []string{"HOME", "USERPROFILE", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "APPDATA", "LOCALAPPDATA"} {
				env.Setenv(name, home)
			}
			env.Setenv("BIN", filepath.Dir(program))
			env.Setenv("PATH", filepath.Dir(program))
			// Windows sockets need SystemRoot to load their providers.
			if root := os.Getenv("SystemRoot"); root != "" {
				env.Setenv("SystemRoot", root)
			}
			env.Setenv("AIGW_SECRET_BACKEND", "file")
			env.Setenv("NO_COLOR", "1")
			server := httptest.NewServer(upstream())
			env.Defer(server.Close)
			env.Setenv("UPSTREAM", server.URL)
			return os.MkdirAll(home, 0o700)
		},
		Cmds: map[string]func(*testscript.TestScript, bool, []string){
			"apikeyhelper": apiKeyHelper,
			"envsubst":     envsubst,
			"normalize":    normalize,
		},
	})
}

// envsubst writes a script file with its $NAME references expanded, so that
// a manifest can name the script's upstream.
func envsubst(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 2 {
		ts.Fatalf("usage: envsubst <source> <target>")
	}
	text := os.Expand(ts.ReadFile(args[0]), ts.Getenv)
	if err := os.WriteFile(ts.MkAbs(args[1]), []byte(text), 0o600); err != nil {
		ts.Fatalf("%v", err)
	}
}

// normalize writes a command's output with the values that differ between
// runs spelled $HOME, $WORK, $BIN and $UPSTREAM, so that cmp can compare and
// update it. Each further argument is a pattern whose matches become
// <varies>. Path separators stay native; scripts match a path on Windows by
// pattern.
func normalize(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) < 2 {
		ts.Fatalf("usage: normalize stdout|stderr <target> [pattern...]")
	}
	text := ts.ReadFile(args[0])
	for _, name := range []string{"HOME", "WORK", "BIN", "UPSTREAM"} {
		for _, value := range spellings(ts.Getenv(name)) {
			text = strings.ReplaceAll(text, value, "$"+name)
		}
	}
	for _, pattern := range args[2:] {
		text = regexp.MustCompile(pattern).ReplaceAllString(text, "<varies>")
	}
	if err := os.WriteFile(ts.MkAbs(args[1]), []byte(text), 0o600); err != nil {
		ts.Fatalf("%v", err)
	}
}

// spellings returns a value as given and as its resolved path, longest
// first. On macOS a temporary path may also appear without /private.
func spellings(value string) []string {
	if value == "" {
		return nil
	}
	values := []string{value}
	if resolved, err := filepath.EvalSymlinks(value); err == nil {
		values = append(values, resolved)
	}
	if runtime.GOOS == "darwin" {
		for _, candidate := range slices.Clone(values) {
			if trimmed, ok := strings.CutPrefix(candidate, "/private"); ok && strings.HasPrefix(trimmed, "/") {
				values = append(values, trimmed)
			}
		}
	}
	slices.Sort(values)
	values = slices.Compact(values)
	slices.SortStableFunc(values, func(a, b string) int { return len(b) - len(a) })
	return values
}

// apiKeyHelper runs the credential helper that AIGW projected into a Claude
// Code settings file, as Claude Code does, and keeps its output for stdout.
func apiKeyHelper(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) != 1 {
		ts.Fatalf("usage: apikeyhelper <settings.json>")
	}
	var settings struct {
		APIKeyHelper string `json:"apiKeyHelper"`
	}
	if err := json.Unmarshal([]byte(ts.ReadFile(args[0])), &settings); err != nil {
		ts.Fatalf("read Claude Code settings: %v", err)
	}
	words := commandWords(settings.APIKeyHelper)
	if len(words) == 0 {
		ts.Fatalf("Claude Code settings have no credential helper")
	}
	err := ts.Exec(words[0], words[1:]...)
	switch {
	case err != nil && !neg:
		ts.Fatalf("credential helper: %v", err)
	case err == nil && neg:
		ts.Fatalf("unexpected credential helper success")
	}
}
