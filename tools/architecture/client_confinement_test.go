package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func confinementFixture(t *testing.T, sources map[string]string) (string, []goFileInfo) {
	t.Helper()
	root := t.TempDir()
	var files []goFileInfo
	for relative, source := range sources {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
		files = append(files, goFileInfo{
			relPath: relative,
			name:    filepath.Base(relative),
			dir:     filepath.ToSlash(filepath.Dir(relative)),
			isTest:  isTestGoFile(filepath.Base(relative)),
		})
	}
	return root, files
}

func confinementPolicy(exceptions ...string) clientConfinement {
	return clientConfinement{
		Clients:             []string{"claude", "claude-desktop", "codex", "hermes"},
		ProductRoots:        []string{"cmd", "internal"},
		AdapterRoot:         "internal/client",
		CompositionRoots:    []string{"cmd/aigw"},
		TemporaryExceptions: exceptions,
	}
}

var confinementSources = map[string]string{
	"internal/shared/paths.go":             "package shared\n\n// Codex in a comment is not knowledge.\nconst home = \"codex\"\n",
	"internal/shared/paths_test.go":        "package shared\n\nconst fixture = \"hermes\"\n",
	"internal/neutral/neutral.go":          "package neutral\n\nconst label = \"client\"\n",
	"internal/client/registry.go":          "package client\n\nfunc NewHermesSelection() {}\n",
	"internal/client/codex/adapter.go":     "package codex\n\nvar codexHome = \"codex\"\n",
	"internal/client/codex/leak.go":        "package codex\n\nvar other = \"aigw verify --for hermes\"\n",
	"internal/client/claudedesktop/app.go": "package claudedesktop\n\nconst app = \"Claude.app\"\n\nvar ClaudeDesktopLibrary = \"x\"\n",
	"internal/client/claude/settings.go":   "package claude\n\nconst settings = \"claude\"\n\nvar desktop = \"Claude Desktop\"\n",
	"cmd/aigw/main.go":                     "package main\n\nimport \"aigw-cli/internal/client/codex\"\n\nvar _ = codex.New\n",
	"tools/release/release.go":             "package release\n\nconst client = \"codex\"\n",
}

func TestClientConfinementRefusesPackagesThatNameAnotherClient(t *testing.T) {
	root, files := confinementFixture(t, confinementSources)
	report := newReport("policy.toml", root)
	if err := checkClientConfinement(root, files, confinementPolicy(), &report); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, finding := range report.Findings {
		got = append(got, finding.Rule+" "+finding.Package+" "+finding.Name+" "+finding.Path)
	}
	want := []string{
		"client_confinement internal/client hermes internal/client/registry.go",
		"client_confinement internal/client/claude claude-desktop internal/client/claude/settings.go",
		"client_confinement internal/client/codex hermes internal/client/codex/leak.go",
		"client_confinement internal/shared codex internal/shared/paths.go",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findings = %q, want %q", got, want)
	}
	if report.Findings[3].Line != 4 {
		t.Fatalf("finding line = %d, want the literal's line 4", report.Findings[3].Line)
	}
}

func TestClientConfinementExceptionsOnlyShrink(t *testing.T) {
	root, files := confinementFixture(t, confinementSources)
	report := newReport("policy.toml", root)
	policy := confinementPolicy("internal/client", "internal/client/claude", "internal/client/codex", "internal/shared", "internal/neutral")
	if err := checkClientConfinement(root, files, policy, &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("findings = %+v, want only the stale exception", report.Findings)
	}
	stale := report.Findings[0]
	if stale.Rule != "client_confinement_exception" || stale.Package != "internal/neutral" {
		t.Fatalf("stale exception finding = %+v", stale)
	}
	if !strings.Contains(stale.Message, "remove it from temporary_exceptions") {
		t.Fatalf("stale exception message = %q", stale.Message)
	}
}

func TestClientConfinementReportsNamingPackagesPerClient(t *testing.T) {
	root, files := confinementFixture(t, confinementSources)
	report := newReport("policy.toml", root)
	policy := confinementPolicy("internal/client", "internal/client/claude", "internal/client/codex", "internal/shared")
	if err := checkClientConfinement(root, files, policy, &report); err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{
		"claude-desktop": {"internal/client/claude"},
		"codex":          {"internal/shared"},
		"hermes":         {"internal/client", "internal/client/codex"},
	}
	if !reflect.DeepEqual(report.ClientPackages, want) {
		t.Fatalf("client packages = %v, want %v", report.ClientPackages, want)
	}
}

// A literal names a client by its value, not its spelling: every Go escape
// form of the same string is refused like the plain one (Codex finding
// 60b67634 showed that "\x63odex" escaped the source-text scan).
func TestClientConfinementDecodesLiteralValues(t *testing.T) {
	for name, literal := range map[string]string{
		"plain":           `"codex"`,
		"hex escape":      `"\x63odex"`,
		"octal escape":    `"\143odex"`,
		"unicode escape":  "\"\\" + "u0063odex\"",
		"raw string":      "`codex`",
		"inside sentence": `"aigw verify --for \x63odex"`,
	} {
		t.Run(name, func(t *testing.T) {
			root, files := confinementFixture(t, map[string]string{
				"internal/shared/paths.go": "package shared\n\nconst home = " + literal + "\n",
			})
			report := newReport("policy.toml", root)
			if err := checkClientConfinement(root, files, confinementPolicy(), &report); err != nil {
				t.Fatal(err)
			}
			if len(report.Findings) != 1 || report.Findings[0].Name != "codex" || report.Findings[0].Line != 3 {
				t.Fatalf("literal %s findings = %+v, want one codex finding on line 3", literal, report.Findings)
			}
		})
	}
}

func TestClientWordsSplitIdentifiersAndLiterals(t *testing.T) {
	for text, want := range map[string][]string{
		"ClaudeDesktopLibrary":     {"claude", "desktop", "library"},
		"codexHome":                {"codex", "home"},
		"NewHermesSelection":       {"new", "hermes", "selection"},
		"aigw verify --for claude": {"aigw", "verify", "for", "claude"},
		"claude-desktop":           {"claude", "desktop"},
		"HTTPServer2":              {"http", "server2"},
		"Claude.app":               {"claude", "app"},
	} {
		if got := splitWords(text); !reflect.DeepEqual(got, want) {
			t.Errorf("splitWords(%q) = %q, want %q", text, got, want)
		}
	}
}

func TestClientConfinementPolicyRejectsInvalidDeclarations(t *testing.T) {
	if err := validateClientConfinement(clientConfinement{}); err != nil {
		t.Fatalf("an absent confinement table must be valid: %v", err)
	}
	if err := validateClientConfinement(confinementPolicy("internal/shared")); err != nil {
		t.Fatalf("valid confinement rejected: %v", err)
	}
	for name, mutate := range map[string]func(*clientConfinement){
		"uppercase client":      func(c *clientConfinement) { c.Clients = append(c.Clients, "Codex") },
		"empty client segment":  func(c *clientConfinement) { c.Clients = append(c.Clients, "claude--desktop") },
		"duplicate client":      func(c *clientConfinement) { c.Clients = append(c.Clients, "codex") },
		"absolute product root": func(c *clientConfinement) { c.ProductRoots = []string{"/internal"} },
		"no product root":       func(c *clientConfinement) { c.ProductRoots = nil },
		"missing adapter root":  func(c *clientConfinement) { c.AdapterRoot = "" },
		"foreign composition":   func(c *clientConfinement) { c.CompositionRoots = []string{`cmd\aigw`} },
		"duplicate exception":   func(c *clientConfinement) { c.TemporaryExceptions = []string{"internal/a", "internal/a"} },
	} {
		t.Run(name, func(t *testing.T) {
			policy := confinementPolicy()
			mutate(&policy)
			if err := validateClientConfinement(policy); err == nil {
				t.Fatal("invalid confinement accepted")
			}
		})
	}
}
