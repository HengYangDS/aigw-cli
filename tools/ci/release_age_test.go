package main

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// releaseAgeRenovateFixture is a policy that holds its root age at zero and
// skips the age for a security fix.
const releaseAgeRenovateFixture = `{
  minimumReleaseAge: "0 hours",
  minimumReleaseAgeBehaviour: "timestamp-required",
  internalChecksFilter: "strict",
  vulnerabilityAlerts: {
    minimumReleaseAge: "0 hours",
  },
}
`

// releaseAgeWithoutRoot is a policy whose root age is missing.
const releaseAgeWithoutRoot = `{
  vulnerabilityAlerts: {
    minimumReleaseAge: "0 hours",
  },
}
`

const releaseAgeNPMFixture = "registry=https://registry.npmjs.org/\n"

// declaredADay is a declaration of one day, as a TOML value.
var declaredADay = strconv.Quote("24h")

// releaseAgeCheckout writes a checkout whose Mise configuration holds declaration,
// a TOML value, and returns its root.
func releaseAgeCheckout(t *testing.T, declaration, renovate, npm string) string {
	t.Helper()
	root := t.TempDir()
	for path, content := range map[string]string{
		releaseAgeDeclaration: "[settings]\nminimum_release_age = " + declaration + "\n",
		releaseAgeRenovate:    renovate,
		releaseAgeNPM:         npm,
	} {
		target := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func releaseAgeFile(t *testing.T, root, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func acceptingRunner(command) error { return nil }

func TestReleaseAgeFillGivesEachSettingItsLineForTheDeclaredHours(t *testing.T) {
	for _, test := range []struct {
		hours, days string
	}{{"0", ""}, {"1", "1"}, {"24", "1"}, {"25", "2"}, {"72", "3"}} {
		t.Run(test.hours+"h", func(t *testing.T) {
			root := releaseAgeCheckout(t, strconv.Quote(test.hours+"h"), releaseAgeRenovateFixture, releaseAgeNPMFixture+"min-release-age=9\n")
			var calls []command
			if err := fillReleaseAge(root, func(call command) error { calls = append(calls, call); return nil }); err != nil {
				t.Fatal(err)
			}
			renovate := releaseAgeFile(t, root, releaseAgeRenovate)
			if !strings.Contains(renovate, "\n  minimumReleaseAge: "+strconv.Quote(test.hours+" hours")+",\n") || !strings.Contains(renovate, "vulnerabilityAlerts: {\n    minimumReleaseAge: "+strconv.Quote("0 hours")+",\n") {
				t.Fatalf("Renovate's policy after the fill:\n%s", renovate)
			}
			want := releaseAgeNPMFixture
			if test.days != "" {
				want += "min-release-age=" + test.days + "\n"
			}
			if got := releaseAgeFile(t, root, releaseAgeNPM); got != want {
				t.Fatalf("npm's configuration = %q, want %q", got, want)
			}
			if len(calls) != 1 || calls[0].Name != "node" || calls[0].Input != renovate || !strings.HasSuffix(strings.Join(calls[0].Args, " "), " "+test.hours+" hours") {
				t.Fatalf("the reader's call = %+v", calls)
			}
		})
	}
}

func TestReleaseAgeCheckNamesTheFilesThatDifferAndFillRepairsThem(t *testing.T) {
	root := releaseAgeCheckout(t, declaredADay, releaseAgeRenovateFixture, releaseAgeNPMFixture)
	err := checkReleaseAge(root, acceptingRunner)
	for _, want := range []string{"release_age_differs:", releaseAgeRenovate, releaseAgeNPM, "mise run fix"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("check error = %v, want it to hold %q", err, want)
		}
	}
	if releaseAgeFile(t, root, releaseAgeRenovate) != releaseAgeRenovateFixture || releaseAgeFile(t, root, releaseAgeNPM) != releaseAgeNPMFixture {
		t.Fatal("the check wrote a file")
	}
	if err := fillReleaseAge(root, acceptingRunner); err != nil {
		t.Fatal(err)
	}
	if err := checkReleaseAge(root, acceptingRunner); err != nil {
		t.Fatalf("the check after the fill: %v", err)
	}
}

func TestReleaseAgeRefusesBeforeItWritesAnything(t *testing.T) {
	for _, test := range []struct {
		name, declaration, renovate, npm, want string
	}{
		{"a unit of days", strconv.Quote("1d"), releaseAgeRenovateFixture, releaseAgeNPMFixture, "release_age_undeclared"},
		{"a number in place of hours", "24", releaseAgeRenovateFixture, releaseAgeNPMFixture, "release_age_undeclared"},
		{"a cutoff date of npm's own", declaredADay, releaseAgeRenovateFixture, releaseAgeNPMFixture + "before=2026-01-01\n", "release_age_other_rule:" + releaseAgeNPM + ":before"},
		{"an exemption of npm's own", declaredADay, releaseAgeRenovateFixture, releaseAgeNPMFixture + "min-release-age-exclude=prettier\n", "release_age_other_rule:" + releaseAgeNPM + ":min-release-age-exclude"},
		{"two ages in npm", declaredADay, releaseAgeRenovateFixture, releaseAgeNPMFixture + "min-release-age=1\nmin-release-age=2\n", "release_age_setting_unreadable:" + releaseAgeNPM},
		{"a policy without a root age", declaredADay, releaseAgeWithoutRoot, releaseAgeNPMFixture, "release_age_setting_unreadable:" + releaseAgeRenovate},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := releaseAgeCheckout(t, test.declaration, test.renovate, test.npm)
			err := fillReleaseAge(root, acceptingRunner)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("fill error = %v, want it to hold %q", err, test.want)
			}
			if releaseAgeFile(t, root, releaseAgeRenovate) != test.renovate || releaseAgeFile(t, root, releaseAgeNPM) != test.npm {
				t.Fatal("a refused fill wrote a file")
			}
		})
	}
}

func TestReleaseAgeFillWritesNothingWhenTheReaderRefuses(t *testing.T) {
	root := releaseAgeCheckout(t, declaredADay, releaseAgeRenovateFixture, releaseAgeNPMFixture)
	refusal := errors.New("the reader refused the filled policy")
	if err := fillReleaseAge(root, func(command) error { return refusal }); !errors.Is(err, refusal) {
		t.Fatalf("fill error = %v, want the reader's refusal", err)
	}
	if releaseAgeFile(t, root, releaseAgeRenovate) != releaseAgeRenovateFixture || releaseAgeFile(t, root, releaseAgeNPM) != releaseAgeNPMFixture {
		t.Fatal("a fill that the reader refused wrote a file")
	}
}

func TestReleaseAgeNPMFillHandlesABlankFileAndAnUnderscoredSpelling(t *testing.T) {
	if got, problems := fillNPMAge("", 24); got != "min-release-age=1\n" || len(problems) != 0 {
		t.Fatalf("a blank file at 24h = %q, %v", got, problems)
	}
	if got, problems := fillNPMAge("", 0); got != "" || len(problems) != 0 {
		t.Fatalf("a blank file at 0h = %q, %v", got, problems)
	}
	if got, problems := fillNPMAge("min_release_age=3\n", 24); got != "min-release-age=1\n" || len(problems) != 0 {
		t.Fatalf("an underscored age line = %q, %v", got, problems)
	}
	if _, problems := fillNPMAge("min_release_age_exclude=prettier\n", 24); len(problems) != 1 || !strings.Contains(problems[0], "min-release-age-exclude") {
		t.Fatalf("an underscored exemption = %v", problems)
	}
}

func TestReleaseAgeFillWritesNothingWhenAFileCannotBeWritten(t *testing.T) {
	root := releaseAgeCheckout(t, strconv.Quote("48h"), releaseAgeRenovateFixture, releaseAgeNPMFixture)
	npm := filepath.Join(root, filepath.FromSlash(releaseAgeNPM))
	if err := os.Chmod(npm, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(npm, 0o600) })
	if file, err := os.OpenFile(npm, os.O_WRONLY, 0); err == nil {
		_ = file.Close()
		t.Skip("this process can write a read-only file, so the case cannot be made")
	}
	if err := fillReleaseAge(root, acceptingRunner); err == nil {
		t.Fatal("the fill reported success although a target cannot be written")
	}
	if releaseAgeFile(t, root, releaseAgeRenovate) != releaseAgeRenovateFixture || releaseAgeFile(t, root, releaseAgeNPM) != releaseAgeNPMFixture {
		t.Fatal("a fill that could not write one file wrote the other")
	}
}

func TestReleaseAgeNPMDaysDoNotWrapForALargeDeclaration(t *testing.T) {
	const want = math.MaxInt/releaseAgeDayHours + 1
	if got := npmAgeLine(math.MaxInt); got != "min-release-age="+strconv.Itoa(want) {
		t.Fatalf("npmAgeLine(MaxInt) = %q, want the %d days that cover it", got, want)
	}
}
