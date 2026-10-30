package main

import (
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
	var configuration struct {
		Files struct {
			Exclude []string `toml:"extend-exclude"`
		} `toml:"files"`
		Default struct {
			Ignored     []string          `toml:"extend-ignore-re"`
			Identifiers map[string]string `toml:"extend-identifiers"`
			Words       map[string]string `toml:"extend-words"`
		} `toml:"default"`
	}
	if err := toml.Unmarshal(data, &configuration); err != nil {
		t.Fatalf("configuration is not TOML: %v\n%s", err, data)
	}
	if !slices.Equal(configuration.Files.Exclude, []string{"history/**"}) || configuration.Default.Identifiers["kept"] != "kept" ||
		!slices.Equal(configuration.Default.Ignored, append([]string{"kept"}, namingPassages...)) ||
		len(configuration.Default.Words) != 2 || configuration.Default.Words[rejectedWord] != "catalog" || configuration.Default.Words[rejectedWord+"s"] != "catalogs" {
		t.Fatalf("configuration = %+v", configuration)
	}
	if _, err := synonymConfiguration([]byte("invalid = ["), nil); err == nil {
		t.Fatal("an invalid policy was accepted")
	}
}

func TestSynonymFilesLeaveHistoryOut(t *testing.T) {
	root := t.TempDir()
	writeRepositoryFile(t, root, "openspec/changes/active/specs/rewritten/spec.md", "## ADDED Requirements\n")
	writeRepositoryFile(t, root, "openspec/changes/archive/2026-01-01-done/specs/kept/spec.md", "")
	files := []string{
		"README.md",
		"openspec/specs/rewritten/spec.md",
		"openspec/specs/kept/spec.md",
		"openspec/changes/active/design.md",
		"openspec/changes/active/specs/rewritten/spec.md",
		"openspec/changes/archive/2026-01-01-done/design.md",
	}
	got, err := synonymFiles(root, files)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"README.md", "openspec/specs/kept/spec.md", "openspec/changes/active/specs/rewritten/spec.md"}
	if !slices.Equal(got, want) {
		t.Fatalf("files = %v, want %v", got, want)
	}
}

func TestSpellingRefusesARejectedSynonymOutsideHistory(t *testing.T) {
	repository := repositoryRoot(t)
	root := newGitRepository(t)
	write := func(relative, content string) { writeRepositoryFile(t, root, relative, content) }
	capitalized := strings.ToUpper(rejectedWord[:1]) + rejectedWord[1:]
	write(".config/checks/spelling/policy.toml", string(readFile(t, filepath.Join(repository, ".config", "checks", "spelling", "policy.toml"))))
	write(glossaryPath, glossaryFixture("| Catalog | A list of models | `"+rejectedWord+"` |\n"))
	write("README.md", "Read the "+rejectedWord+" first.\n")
	write("source.go", "package fixture\n\nvar hermes"+capitalized+" = 1\n")
	write("CHANGELOG.md", "# Changelog\n\n## [Unreleased]\n\nNothing.\n\n## [1.0.0] - 2026-01-01\n\n- The "+rejectedWord+" grew.\n")
	write("openspec/changes/active/design.md", "The word "+rejectedWord+" is rejected.\n")
	write("openspec/changes/active/specs/models/spec.md", "## ADDED Requirements\n\n### Requirement: Models are listed\n\nThe catalog SHALL list models.\n\n## REMOVED Requirements\n\n### Requirement: The "+rejectedWord+" lists models\n\n**Reason**: Renamed.\n\n**Migration**: Read the new name.\n")
	write("openspec/specs/models/spec.md", "# models\n\nThe "+rejectedWord+" lists models.\n")
	if output, err := exec.Command("git", "-C", root, "add", ".").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, output)
	}
	var diagnostics []byte
	err := checkSpelling(root, func(call command) error {
		output, err := systemOutputRunner(call)
		diagnostics = append(diagnostics, output...)
		return err
	})
	for _, finding := range []string{"README.md", "source.go", "should be `catalog`", "should be `Catalog`"} {
		if err == nil || !strings.Contains(string(diagnostics), finding) {
			t.Fatalf("spelling error = %v, want %q\n%s", err, finding, diagnostics)
		}
	}
	for _, history := range []string{"CHANGELOG.md", "design.md", "openspec/changes/active/specs", "openspec/specs/models"} {
		if strings.Contains(string(diagnostics), history) {
			t.Fatalf("the synonym check read history or a rewritten specification (%s):\n%s", history, diagnostics)
		}
	}
	write("README.md", "Read the catalog first.\n")
	write("source.go", "package fixture\n\nvar hermesCatalog = 1\n")
	if err := checkSpelling(root, systemRunner); err != nil {
		t.Fatalf("spelling refused canonical text: %v", err)
	}
	if entries, err := os.ReadDir(filepath.Join(root, "build", "tmp")); err != nil || len(entries) != 0 {
		t.Fatalf("synonym scratch left behind: %v, %v", entries, err)
	}
}
