package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// desktopTarget finds a Claude Desktop target that a script sets, the library
// that follows --target in a command that names claude-desktop.
var desktopTarget = regexp.MustCompile(`(?m)^.*\bclaude-desktop\b.*--target (\S+)`)

// Claude Desktop keeps its standard configuration two levels above its
// library, so a script whose library lies less deep below $WORK writes a file
// that every script shares, and concurrent scripts break each other. Every
// script nests the library at least two levels below $WORK.
func TestScriptsKeepClaudeDesktopFilesInTheirWorkDirectory(t *testing.T) {
	scripts, err := filepath.Glob(filepath.Join("testdata", "script", "*.txtar"))
	if err != nil || len(scripts) == 0 {
		t.Fatalf("scripts = %v, %v", scripts, err)
	}
	for _, script := range scripts {
		data, err := os.ReadFile(script)
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range desktopTarget.FindAllStringSubmatch(string(data), -1) {
			if !nestedBelowWork(match[1], 2) {
				t.Errorf("%s sets Claude Desktop's library to %s, which puts its standard configuration outside $WORK", script, match[1])
			}
		}
	}
	for target, nested := range map[string]bool{
		"$WORK/configLibrary": false, "$WORK/library": false, "$WORK/a/library": false,
		"$WORK/desktop/Claude-3p/configLibrary": true, "relative/library": false,
	} {
		if got := nestedBelowWork(target, 2); got != nested {
			t.Errorf("nestedBelowWork(%q) = %t, want %t", target, got, nested)
		}
	}
}

// nestedBelowWork reports whether target lies more than levels directories
// below $WORK, so that its levels-th parent is still inside $WORK.
func nestedBelowWork(target string, levels int) bool {
	rest, below := strings.CutPrefix(target, "$WORK/")
	return below && strings.Count(rest, "/") >= levels
}
