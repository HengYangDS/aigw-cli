package main

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// glossaryPath is the product glossary. Its table gives each concept one
// canonical term and the synonyms that the spelling gate refuses.
const glossaryPath = "docs/concepts/product-concepts.md"

var glossaryHeader = []string{"Term", "Meaning", "Rejected synonyms"}

var (
	synonymWord    = regexp.MustCompile(`^[a-z]+$`)
	tableDelimiter = regexp.MustCompile(`^:?-+:?$`)
	nameWords      = regexp.MustCompile(`[A-Z]?[a-z]+|[A-Z]+`)
)

// namingPassage is text that must spell a rejected synonym, in the files that
// a typos file-type glob selects.
type namingPassage struct {
	fileType, glob, pattern string
}

// namingPassages are CHANGELOG's released sections, the glossary's table and
// the requirements that a delta removes under their earlier names.
func namingPassages() []namingPassage {
	columns := make([]string, len(glossaryHeader))
	for index, column := range glossaryHeader {
		columns[index] = " " + regexp.QuoteMeta(column) + " +"
	}
	return []namingPassage{
		{"released-changelog", "CHANGELOG.md", `(?ms)^## \[[0-9][^\n]*\n.*`},
		{"glossary-table", path.Base(glossaryPath), `(?m)^\|` + strings.Join(columns, `\|`) + `\|\n(?:\|[^\n]*\n)*`},
		{"removed-requirements", "spec.md", `(?ms)^## REMOVED Requirements\n.*?(?:^## |\z)`},
	}
}

// rejectedSynonyms maps each rejected synonym in the glossary's table to its
// canonical term. typos matches whole words, so each synonym is one lowercase
// word and the glossary lists each inflected form.
func rejectedSynonyms(root string) (map[string]string, error) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(glossaryPath)))
	if err != nil {
		return nil, fmt.Errorf("read glossary: %w", err)
	}
	_, section, found := strings.Cut(string(data), "\n## Glossary\n")
	if !found {
		return nil, fmt.Errorf("glossary: %s has no Glossary section", glossaryPath)
	}
	section, _, _ = strings.Cut(section, "\n## ")
	var rows [][]string
	for line := range strings.Lines(section) {
		if line = strings.TrimSpace(line); strings.HasPrefix(line, "|") {
			cells := strings.Split(strings.Trim(line, "|"), "|")
			for index := range cells {
				cells[index] = strings.TrimSpace(cells[index])
			}
			rows = append(rows, cells)
		}
	}
	if len(rows) < 3 || !slices.Equal(rows[0], glossaryHeader) || len(rows[1]) != len(glossaryHeader) ||
		slices.ContainsFunc(rows[1], func(cell string) bool { return !tableDelimiter.MatchString(cell) }) {
		return nil, fmt.Errorf("glossary: the Glossary section needs a table with the columns %s and one row per concept", strings.Join(glossaryHeader, ", "))
	}
	synonyms := map[string]string{}
	for _, row := range rows[2:] {
		if len(row) != len(glossaryHeader) || row[0] == "" {
			return nil, fmt.Errorf("glossary: row %q needs a term, a meaning and its rejected synonyms", strings.Join(row, " | "))
		}
		for word := range strings.SplitSeq(row[2], ",") {
			word = strings.Trim(strings.TrimSpace(word), "`")
			if word == "" {
				continue
			}
			if !synonymWord.MatchString(word) {
				return nil, fmt.Errorf("glossary: rejected synonym %q of %q must be one lowercase word", word, row[0])
			}
			if previous, ok := synonyms[word]; ok {
				return nil, fmt.Errorf("glossary: %q is rejected for both %q and %q", word, previous, strings.ToLower(row[0]))
			}
			synonyms[word] = strings.ToLower(row[0])
		}
	}
	return synonyms, nil
}

// synonymCorrection is the canonical term for a rejected synonym, or the term's
// plural when the synonym is the plural of another of its synonyms.
func synonymCorrection(synonyms map[string]string, word string) string {
	term := synonyms[word]
	if singular, ok := strings.CutSuffix(word, "s"); ok && synonyms[singular] == term {
		return term + "s"
	}
	return term
}

// synonymConfiguration adds each rejected synonym to the spelling policy, and
// one file type per naming passage that ignores the passage in its files.
func synonymConfiguration(policy []byte, synonyms map[string]string) ([]byte, error) {
	configuration := map[string]any{}
	if err := toml.Unmarshal(policy, &configuration); err != nil {
		return nil, fmt.Errorf("read spelling policy: %w", err)
	}
	defaults, _ := configuration["default"].(map[string]any)
	if defaults == nil {
		defaults = map[string]any{}
	}
	words, _ := defaults["extend-words"].(map[string]any)
	if words == nil {
		words = map[string]any{}
	}
	for word := range synonyms {
		words[word] = synonymCorrection(synonyms, word)
	}
	defaults["extend-words"] = words
	configuration["default"] = defaults
	types, _ := configuration["type"].(map[string]any)
	if types == nil {
		types = map[string]any{}
	}
	for _, passage := range namingPassages() {
		if _, ok := types[passage.fileType]; ok {
			return nil, fmt.Errorf("spelling policy already declares the file type %q of the synonym check", passage.fileType)
		}
		types[passage.fileType] = map[string]any{"extend-glob": []string{passage.glob}, "extend-ignore-re": []string{passage.pattern}}
	}
	configuration["type"] = types
	return toml.Marshal(configuration)
}

// synonymFiles leaves out history and the records that name what they reject:
// archived Changes, an active Change's own records, and each main
// specification that an active Change rewrites, whose delta stays in.
func synonymFiles(root string, files []string) ([]string, error) {
	transition, err := readOpenSpecTransition(root, files)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(slices.Clone(files), func(file string) bool {
		return strings.HasPrefix(file, "openspec/changes/archive/") || transition.rewritten[file] || transition.changeRecord(file)
	}), nil
}

// rejectedDirectoryNames refuses each directory of files whose name has a
// rejected synonym as one of its words. typos reads file names, not the
// directories that hold them.
func rejectedDirectoryNames(files []string, synonyms map[string]string) error {
	seen := map[string]bool{}
	var findings []string
	for _, file := range files {
		for directory := path.Dir(file); directory != "." && !seen[directory]; directory = path.Dir(directory) {
			seen[directory] = true
			for _, word := range nameWords.FindAllString(path.Base(directory), -1) {
				if word = strings.ToLower(word); synonyms[word] != "" {
					findings = append(findings, fmt.Sprintf("%s: directory name uses the rejected synonym %q; use %q", directory, word, synonymCorrection(synonyms, word)))
				}
			}
		}
	}
	if len(findings) == 0 {
		return nil
	}
	slices.Sort(findings)
	return errors.New(strings.Join(findings, "\n"))
}

// checkRejectedSynonyms runs typos over the current files with the glossary's
// rejected synonyms, so that a finding names the path, the line and the
// canonical term, and refuses a directory whose name uses one.
func checkRejectedSynonyms(root string, files []string, runner commandRunner) (result error) {
	synonyms, err := rejectedSynonyms(root)
	if err != nil || len(synonyms) == 0 {
		return err
	}
	policy, err := os.ReadFile(filepath.Join(root, ".config", "checks", "spelling", "policy.toml"))
	if err != nil {
		return fmt.Errorf("read spelling policy: %w", err)
	}
	configuration, err := synonymConfiguration(policy, synonyms)
	if err != nil {
		return err
	}
	files, err = synonymFiles(root, files)
	if err != nil {
		return err
	}
	scratch := filepath.Join(root, "build", "tmp")
	if err := os.MkdirAll(scratch, 0o700); err != nil {
		return fmt.Errorf("prepare synonym check: %w", err)
	}
	directory, err := os.MkdirTemp(scratch, ".synonyms-")
	if err != nil {
		return fmt.Errorf("prepare synonym check: %w", err)
	}
	defer func() { result = errors.Join(result, os.RemoveAll(directory)) }()
	configurationPath := filepath.Join(directory, "typos.toml")
	if err := os.WriteFile(configurationPath, configuration, 0o600); err != nil {
		return fmt.Errorf("write synonym policy: %w", err)
	}
	return errors.Join(runner(command{
		Name:  "typos",
		Dir:   root,
		Args:  []string{"--config", configurationPath, "--force-exclude", "--file-list", "-"},
		Input: strings.Join(files, "\n") + "\n",
	}), rejectedDirectoryNames(files, synonyms))
}
