package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// The release age is declared once, in whole hours, as Mise's own
// minimum_release_age. `ci release-age` fills Renovate's root age and npm's
// min-release-age from it; `ci check-release-age` fails while either file holds
// another age, or another rule of its own. npm counts days, so it holds a
// release for the days that cover the declared hours, and an age of zero leaves
// its line out.
const (
	releaseAgeDeclaration = ".config/mise/config.toml"
	releaseAgeRenovate    = ".config/checks/renovate/renovate.json5"
	releaseAgeNPM         = ".config/npm/npmrc"
	releaseAgeReader      = "release-age.mjs"
	releaseAgeDayHours    = 24
)

var (
	releaseAgeDeclared = regexp.MustCompile(`^(0|[1-9][0-9]*)h$`)
	releaseAgeRoot     = regexp.MustCompile(`(?m)^  minimumReleaseAge: "[^"]*",$`)
)

func checkReleaseAge(root string, runner commandRunner) error {
	return releaseAge(root, runner, false)
}

func fillReleaseAge(root string, runner commandRunner) error {
	return releaseAge(root, runner, true)
}

// releaseAge reads the declared age, gives each file its line, has the Node
// reader prove that Renovate reads the filled policy as the declared age, and
// then either names the files that differ or writes them. A refusal writes
// nothing.
func releaseAge(root string, runner commandRunner, write bool) error {
	hours, err := declaredReleaseAge(root)
	if err != nil {
		return err
	}
	current := make(map[string]string, 2)
	for _, path := range []string{releaseAgeRenovate, releaseAgeNPM} {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return fmt.Errorf("release_age_setting_unreadable:%s: %w", path, err)
		}
		current[path] = string(data)
	}
	renovate, err := fillRenovateAge(current[releaseAgeRenovate], hours)
	if err != nil {
		return err
	}
	npm, others := fillNPMAge(current[releaseAgeNPM], hours)
	if len(others) > 0 {
		return errors.New(strings.Join(others, "\n"))
	}
	filled := map[string]string{releaseAgeRenovate: renovate, releaseAgeNPM: npm}
	reader := filepath.Join(root, "tools", "ci", releaseAgeReader)
	if err := runner(command{Name: "node", Dir: root, Args: []string{reader, renovateAgeText(hours)}, Input: renovate}); err != nil {
		return err
	}
	var differing []string
	for _, path := range []string{releaseAgeRenovate, releaseAgeNPM} {
		if filled[path] != current[path] {
			differing = append(differing, path)
		}
	}
	if len(differing) == 0 {
		return nil
	}
	if !write {
		return fmt.Errorf("release_age_differs:%s: run `mise run fix`", strings.Join(differing, ","))
	}
	// Open every target for writing before the first write, so that a file that cannot be
	// written leaves the other one as it was.
	for _, path := range differing {
		file, err := os.OpenFile(filepath.Join(root, filepath.FromSlash(path)), os.O_WRONLY, 0)
		if err != nil {
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
	}
	for _, path := range differing {
		target := filepath.Join(root, filepath.FromSlash(path))
		info, err := os.Stat(target)
		if err != nil {
			return err
		}
		if err := os.WriteFile(target, []byte(filled[path]), info.Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}

// declaredReleaseAge returns the hours that Mise's minimum_release_age holds in
// the repository's own configuration.
func declaredReleaseAge(root string) (int, error) {
	refusal := fmt.Errorf("release_age_undeclared:%s: [settings] minimum_release_age takes whole hours, such as \"24h\"", releaseAgeDeclaration)
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(releaseAgeDeclaration)))
	if err != nil {
		return 0, errors.Join(refusal, err)
	}
	var document struct {
		Settings struct {
			MinimumReleaseAge string `toml:"minimum_release_age"`
		} `toml:"settings"`
	}
	if err := toml.Unmarshal(data, &document); err != nil {
		return 0, errors.Join(refusal, err)
	}
	match := releaseAgeDeclared.FindStringSubmatch(document.Settings.MinimumReleaseAge)
	if match == nil {
		return 0, refusal
	}
	hours, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, errors.Join(refusal, err)
	}
	return hours, nil
}

func renovateAgeText(hours int) string {
	return strconv.Itoa(hours) + " hours"
}

// fillRenovateAge gives the policy's root age its line; the security-fix
// rule's own age stands deeper and is not touched.
func fillRenovateAge(text string, hours int) (string, error) {
	if len(releaseAgeRoot.FindAllStringIndex(text, -1)) != 1 {
		return "", fmt.Errorf("release_age_setting_unreadable:%s: the root minimumReleaseAge must stand once on one line at the root's indentation", releaseAgeRenovate)
	}
	return releaseAgeRoot.ReplaceAllLiteralString(text, `  minimumReleaseAge: "`+renovateAgeText(hours)+`",`), nil
}

func npmAgeLine(hours int) string {
	if hours == 0 {
		return ""
	}
	return "min-release-age=" + strconv.Itoa(hours/releaseAgeDayHours+min(hours%releaseAgeDayHours, 1))
}

// fillNPMAge puts npm's age line in place of the one that the file holds, or
// at its end, and leaves every other line as it was. It names each other rule
// that holds or admits releases, and a second age line, which npm's readers
// treat differently.
func fillNPMAge(text string, hours int) (string, []string) {
	var kept, problems []string
	line, placed, ages := npmAgeLine(hours), false, 0
	for entry := range strings.SplitSeq(strings.TrimRight(text, "\n"), "\n") {
		key, _, _ := strings.Cut(entry, "=")
		key = strings.TrimSuffix(strings.Trim(strings.TrimSpace(key), `"'`), "[]")
		switch {
		case key == "min-release-age":
			ages++
			if line != "" && !placed {
				kept, placed = append(kept, line), true
			}
		case key == "before" || strings.HasPrefix(key, "min-release-age-"):
			problems = append(problems, "release_age_other_rule:"+releaseAgeNPM+":"+key)
			kept = append(kept, entry)
		default:
			kept = append(kept, entry)
		}
	}
	if ages > 1 {
		problems = append(problems, "release_age_setting_unreadable:"+releaseAgeNPM)
	}
	if line != "" && !placed {
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n") + "\n", problems
}
