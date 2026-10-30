package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

// rejectedWord is spelled from bytes so that the spelling gate does not read
// this file as using it.
var rejectedWord = string([]byte{'c', 'a', 't', 'a', 'l', 'o', 'g', 'u', 'e'})

func glossaryFixture(rows string) string {
	return "# Product Concepts\n\nIntro.\n\n## Glossary\n\n| Term | Meaning | Rejected synonyms |\n| --- | --- | --- |\n" + rows + "\n## Core entities\n\nText.\n"
}

func writeRepositoryFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryGlossaryRejectsTheMeasuredDrift(t *testing.T) {
	synonyms, err := rejectedSynonyms(repositoryRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	if synonyms[rejectedWord] != "catalog" || synonyms[rejectedWord+"s"] != "catalog" {
		t.Fatalf("synonyms = %v", synonyms)
	}
}

func TestRejectedSynonymsReadTheGlossaryTable(t *testing.T) {
	root := t.TempDir()
	writeRepositoryFile(t, root, glossaryPath, glossaryFixture(
		"| Account | A provider's endpoints | |\n| Catalog | A list of models | `"+rejectedWord+"`, `"+rejectedWord+"s` |\n"))
	synonyms, err := rejectedSynonyms(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(synonyms) != 2 || synonyms[rejectedWord] != "catalog" || synonyms[rejectedWord+"s"] != "catalog" {
		t.Fatalf("synonyms = %v", synonyms)
	}
	for name, content := range map[string]string{
		"no section":     "# Product Concepts\n\n## Core entities\n",
		"no table":       "# Product Concepts\n\n## Glossary\n\nText.\n\n## Core entities\n",
		"other columns":  "# Product Concepts\n\n## Glossary\n\n| Term | Meaning |\n| --- | --- |\n| Catalog | A list |\n",
		"no delimiter":   "# Product Concepts\n\n## Glossary\n\n| Term | Meaning | Rejected synonyms |\n| Catalog | A list | |\n| Route | A path | |\n",
		"short row":      glossaryFixture("| Catalog | A list |\n"),
		"several words":  glossaryFixture("| Catalog | A list | `model list` |\n"),
		"capitals":       glossaryFixture("| Catalog | A list | `Listing` |\n"),
		"rejected twice": glossaryFixture("| Catalog | A list | `listing` |\n| Index | A list | `listing` |\n"),
		"no glossary":    "",
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if content != "" {
				writeRepositoryFile(t, root, glossaryPath, content)
			}
			if _, err := rejectedSynonyms(root); err == nil {
				t.Fatal("an invalid glossary was accepted")
			}
		})
	}
}

func TestSynonymConfigurationKeepsThePolicy(t *testing.T) {
	policy := "[files]\nextend-exclude = [\"history/**\"]\n\n[default]\nextend-ignore-re = [\"kept\"]\n\n[default.extend-identifiers]\nkept = \"kept\"\n"
	data, err := synonymConfiguration([]byte(policy), map[string]string{rejectedWord: "catalog", rejectedWord + "s": "catalog"})
	if err != nil {
		t.Fatal(err)
	}
	type fileType struct {
		Globs   []string `toml:"extend-glob"`
		Ignored []string `toml:"extend-ignore-re"`
	}
	var configuration struct {
		Files struct {
			Exclude []string `toml:"extend-exclude"`
		} `toml:"files"`
		Default struct {
			Ignored     []string          `toml:"extend-ignore-re"`
			Identifiers map[string]string `toml:"extend-identifiers"`
			Words       map[string]string `toml:"extend-words"`
		} `toml:"default"`
		Types map[string]fileType `toml:"type"`
	}
	if err := toml.Unmarshal(data, &configuration); err != nil {
		t.Fatalf("configuration is not TOML: %v\n%s", err, data)
	}
	if !slices.Equal(configuration.Files.Exclude, []string{"history/**"}) || configuration.Default.Identifiers["kept"] != "kept" ||
		!slices.Equal(configuration.Default.Ignored, []string{"kept"}) ||
		len(configuration.Default.Words) != 2 || configuration.Default.Words[rejectedWord] != "catalog" || configuration.Default.Words[rejectedWord+"s"] != "catalogs" {
		t.Fatalf("configuration = %+v", configuration)
	}
	for _, passage := range namingPassages() {
		if got := configuration.Types[passage.fileType]; !slices.Equal(got.Globs, []string{passage.glob}) || !slices.Equal(got.Ignored, []string{passage.pattern}) {
			t.Fatalf("file type %s = %+v", passage.fileType, got)
		}
	}
	if _, err := synonymConfiguration([]byte("invalid = ["), nil); err == nil {
		t.Fatal("an invalid policy was accepted")
	}
	if _, err := synonymConfiguration([]byte("[type.glossary-table]\nextend-glob = [\"x\"]\n"), nil); err == nil {
		t.Fatal("a policy file type was replaced")
	}
}

func TestSynonymFilesLeaveHistoryOut(t *testing.T) {
	root := t.TempDir()
	writeRepositoryFile(t, root, "openspec/changes/active/specs/rewritten/spec.md", "## MODIFIED Requirements\n\n### Requirement: A\n")
	writeRepositoryFile(t, root, "openspec/changes/active/specs/added/spec.md", "## ADDED Requirements\n\n### Requirement: B\n")
	writeRepositoryFile(t, root, "openspec/changes/archive/2026-01-01-done/specs/kept/spec.md", "## REMOVED Requirements\n")
	files := []string{
		"README.md",
		"openspec/specs/added/spec.md",
		"openspec/specs/kept/spec.md",
		"openspec/specs/rewritten/spec.md",
		"openspec/changes/active/design.md",
		"openspec/changes/active/specs/added/spec.md",
		"openspec/changes/active/specs/rewritten/spec.md",
		"openspec/changes/archive/2026-01-01-done/design.md",
		"openspec/changes/archive/2026-01-01-done/specs/kept/spec.md",
	}
	got, err := synonymFiles(root, files)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"README.md",
		"openspec/specs/added/spec.md",
		"openspec/specs/kept/spec.md",
		"openspec/changes/active/specs/added/spec.md",
		"openspec/changes/active/specs/rewritten/spec.md",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("files = %v, want %v", got, want)
	}
}

func TestRejectedDirectoryNamesSplitNames(t *testing.T) {
	synonyms := map[string]string{rejectedWord: "catalog", rejectedWord + "s": "catalog"}
	capitalized := strings.ToUpper(rejectedWord[:1]) + rejectedWord[1:]
	err := rejectedDirectoryNames([]string{
		"docs/tool-search-" + rejectedWord + "/note.md",
		"internal/model" + capitalized + "/document.go",
		"internal/model" + capitalized + "/document_test.go",
		"internal/modelcatalog/document.go",
		"README.md",
	}, synonyms)
	want := fmt.Sprintf("docs/tool-search-%s: directory name uses the rejected synonym %q; use \"catalog\"\ninternal/model%s: directory name uses the rejected synonym %q; use \"catalog\"", rejectedWord, rejectedWord, capitalized, rejectedWord)
	if err == nil || err.Error() != want {
		t.Fatalf("directory findings = %v, want %s", err, want)
	}
	if err := rejectedDirectoryNames([]string{"internal/modelcatalog/document.go", "README.md"}, synonyms); err != nil {
		t.Fatalf("canonical directory names were refused: %v", err)
	}
}

func TestSpellingRefusesARejectedSynonymOutsideHistory(t *testing.T) {
	repository := repositoryRoot(t)
	root := newGitRepository(t)
	write := func(relative, content string) { writeRepositoryFile(t, root, relative, content) }
	// checked holds the files that the synonym check reads, with word where
	// it reads text; the history around word must keep the rejected spelling.
	checked := func(word string) map[string]string {
		return map[string]string{
			glossaryPath:   "# Product Concepts\n\n## Glossary\n\nThe intro names the " + word + ".\n\n| Term | Meaning | Rejected synonyms |\n| --- | --- | --- |\n| Catalog | A list of models | `" + rejectedWord + "` |\n\n## Core entities\n\nAfter the " + word + ".\n",
			"README.md":    "Read the " + word + " first.\n",
			"notes.md":     "# Notes\n\n## [1] Background\n\nThe " + word + " here.\n",
			"source.go":    "package fixture\n\nconst fixture = `\n## [1.0.0]\n`\n\nvar hermes" + strings.ToUpper(word[:1]) + word[1:] + " = 1\n",
			"CHANGELOG.md": "# Changelog\n\n## [Unreleased]\n\n- The " + word + " moved.\n\n## [1.0.0] - 2026-01-01\n\n- The " + rejectedWord + " grew.\n",
			"openspec/changes/active/specs/models/spec.md":   "## ADDED Requirements\n\n### Requirement: Models are listed\n\nThe " + word + " SHALL list models.\n\n## REMOVED Requirements\n\n### Requirement: The " + rejectedWord + " lists models\n\n**Reason**: Renamed.\n\n**Migration**: Read the new name.\n",
			"openspec/changes/active/specs/accounts/spec.md": "## ADDED Requirements\n\n### Requirement: Accounts are listed\n\nThe catalog SHALL list Accounts.\n",
			"openspec/specs/accounts/spec.md":                "# accounts\n\nThe " + word + " lists Accounts.\n",
			"docs/" + word + "/note.md":                      "Text.\n",
		}
	}
	write(".config/checks/spelling/policy.toml", string(readFile(t, filepath.Join(repository, ".config", "checks", "spelling", "policy.toml"))))
	write("openspec/changes/active/design.md", "The word "+rejectedWord+" is rejected.\n")
	write("openspec/changes/active/specs/routes/spec.md", "## MODIFIED Requirements\n\n### Requirement: Routes are listed\n\nThe catalog SHALL list Routes.\n")
	write("openspec/specs/models/spec.md", "# models\n\nThe "+rejectedWord+" lists models.\n")
	write("openspec/specs/routes/spec.md", "# routes\n\nThe "+rejectedWord+" lists Routes.\n")
	for path, content := range checked(rejectedWord) {
		write(path, content)
	}
	if output, err := exec.Command("git", "-C", root, "add", ".").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, output)
	}
	var diagnostics []byte
	err := checkSpelling(root, func(call command) error {
		output, err := systemOutputRunner(call)
		diagnostics = append(diagnostics, output...)
		return err
	})
	refused := string(diagnostics)
	if err != nil {
		refused += "\n" + err.Error()
	}
	for _, finding := range []string{
		"README.md:1:", "notes.md:5:", "source.go:7:", "CHANGELOG.md:5:", glossaryPath + ":5:", glossaryPath + ":13:",
		"openspec/changes/active/specs/models/spec.md:5:", "openspec/specs/accounts/spec.md:3:",
		"should be `catalog`", "should be `Catalog`", "docs/" + rejectedWord + ": directory name",
	} {
		if err == nil || !strings.Contains(refused, finding) {
			t.Fatalf("spelling error = %v, want %q\n%s", err, finding, refused)
		}
	}
	for _, history := range []string{"CHANGELOG.md:9:", glossaryPath + ":9:", "design.md", "active/specs/models/spec.md:9:", "openspec/specs/models", "openspec/specs/routes"} {
		if strings.Contains(refused, history) {
			t.Fatalf("the synonym check read history or a rewritten specification (%s):\n%s", history, refused)
		}
	}
	if err := os.RemoveAll(filepath.Join(root, "docs", rejectedWord)); err != nil {
		t.Fatal(err)
	}
	for path, content := range checked("catalog") {
		write(path, content)
	}
	if err := checkSpelling(root, systemRunner); err != nil {
		t.Fatalf("spelling refused canonical text: %v", err)
	}
	if entries, err := os.ReadDir(filepath.Join(root, "build", "tmp")); err != nil || len(entries) != 0 {
		t.Fatalf("synonym scratch left behind: %v, %v", entries, err)
	}
}
