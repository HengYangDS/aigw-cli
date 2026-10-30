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

func confinementPolicy() clientConfinement {
	return clientConfinement{
		Clients:          []string{"claude", "claude-desktop", "codex", "hermes"},
		ProductRoots:     []string{"cmd", "internal"},
		AdapterRoot:      "internal/client",
		CompositionRoots: []string{"cmd/aigw"},
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

// An adapter's package name joins its client ID's words, so a command that
// imports the Claude Desktop adapter names that client.
func TestClientConfinementReadsTheAdapterPackageNameOfAMultiWordID(t *testing.T) {
	if got := clientsInWords([]string{"internal", "client", "claudedesktop"}, clientWords(confinementPolicy().Clients)); !reflect.DeepEqual(got, [][]string{{"claude", "desktop"}}) {
		t.Fatalf("clients = %q", got)
	}
	root, files := confinementFixture(t, map[string]string{
		"internal/cli/sync/desktop.go":           "package sync\n\nimport \"aigw-cli/internal/client/claudedesktop\"\n\nvar _ = claudedesktop.LibraryPathFor\n",
		"internal/client/claudedesktop/paths.go": "package claudedesktop\n\nfunc LibraryPathFor() {}\n",
	})
	report := newReport("policy.toml", root)
	if err := checkClientConfinement(root, files, confinementPolicy(), &report); err != nil {
		t.Fatal(err)
	}
	if want := map[string][]string{"claude-desktop": {"internal/cli/sync"}}; !reflect.DeepEqual(report.ClientPackages, want) {
		t.Fatalf("client packages = %v, want %v", report.ClientPackages, want)
	}
}

func TestClientConfinementReportsNamingPackagesPerClient(t *testing.T) {
	root, files := confinementFixture(t, confinementSources)
	report := newReport("policy.toml", root)
	if err := checkClientConfinement(root, files, confinementPolicy(), &report); err != nil {
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

// A client's native vocabulary can contain another client's ID, like Hermes'
// API mode "codex_responses". A declared native term is accepted only as that
// exact literal in its own client's adapter package.
func TestClientConfinementAcceptsDeclaredNativeTermsOnlyInTheirAdapter(t *testing.T) {
	policy := confinementPolicy()
	policy.NativeTerms = map[string][]string{"hermes": {"codex_responses"}}
	root, files := confinementFixture(t, map[string]string{
		"internal/client/hermes/document.go":   "package hermes\n\nvar mode = \"codex_responses\"\n",
		"internal/client/hermes/escaped.go":    "package hermes\n\nvar mode = \"\\x63odex_responses\"\n",
		"internal/client/hermes/variant.go":    "package hermes\n\nvar other = \"codex_responses_v2\"\n",
		"internal/client/claude/settings.go":   "package claude\n\nvar mode = \"codex_responses\"\n",
		"internal/shared/modes.go":             "package shared\n\nvar mode = \"codex_responses\"\n",
		"internal/client/hermes/identifier.go": "package hermes\n\nvar codexResponses = 1\n",
	})
	report := newReport("policy.toml", root)
	if err := checkClientConfinement(root, files, policy, &report); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, finding := range report.Findings {
		got = append(got, finding.Rule+" "+finding.Package+" "+finding.Name)
	}
	want := []string{
		"client_confinement internal/client/claude codex",
		"client_confinement internal/client/hermes codex",
		"client_confinement internal/shared codex",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findings = %q, want %q", got, want)
	}
	for _, finding := range report.Findings {
		if finding.Package == "internal/client/hermes" && finding.Path != "internal/client/hermes/identifier.go" && finding.Path != "internal/client/hermes/variant.go" {
			t.Fatalf("the declared native term itself was refused: %+v", finding)
		}
	}
}

func TestClientConfinementRefusesAnUnusedNativeTerm(t *testing.T) {
	policy := confinementPolicy()
	policy.NativeTerms = map[string][]string{"hermes": {"codex_responses"}}
	root, files := confinementFixture(t, map[string]string{
		"internal/client/hermes/document.go": "package hermes\n\nvar mode = \"anthropic_messages\"\n",
	})
	report := newReport("policy.toml", root)
	if err := checkClientConfinement(root, files, policy, &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("findings = %+v, want only the unused native term", report.Findings)
	}
	stale := report.Findings[0]
	if stale.Rule != "client_confinement_native_term" || stale.Package != "internal/client/hermes" || stale.Name != "codex_responses" || !strings.Contains(stale.Message, "remove it from native_terms") {
		t.Fatalf("unused native term finding = %+v", stale)
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
	if err := validateClientConfinement(confinementPolicy()); err != nil {
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
		"duplicate composition": func(c *clientConfinement) { c.CompositionRoots = []string{"cmd/aigw", "cmd/aigw"} },
		"native term owner":     func(c *clientConfinement) { c.NativeTerms = map[string][]string{"gemini": {"codex_responses"}} },
		"empty native term":     func(c *clientConfinement) { c.NativeTerms = map[string][]string{"hermes": {""}} },
		"duplicate native term": func(c *clientConfinement) {
			c.NativeTerms = map[string][]string{"hermes": {"codex_responses", "codex_responses"}}
		},
		"needless native term": func(c *clientConfinement) { c.NativeTerms = map[string][]string{"hermes": {"hermes_mode"}} },
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
