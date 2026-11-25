package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// releaseAgeRenovateFixture is a policy that holds its root age at zero and
// skips the age for a security fix.
const releaseAgeRenovateFixture = `{
  minimumReleaseAge: "0 hours",
  minimumReleaseAgeBehaviour: "timestamp-required",
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

func TestReleaseAgeReaderNamesWhatRenovateReadsAsItsParserDoes(t *testing.T) {
	root := repositoryRoot(t)
	reader := filepath.Join(root, "tools", "ci", releaseAgeReader)
	securityHeld := strings.Replace(releaseAgeRenovateFixture, `    minimumReleaseAge: "0 hours"`, `    minimumReleaseAge: "24 hours"`, 1)
	ownRule := strings.Replace(releaseAgeRenovateFixture, `  vulnerabilityAlerts: {`, `  packageRules: [{ minimumReleaseAge: "72 hours" }],`+"\n"+`  vulnerabilityAlerts: {`, 1)
	for _, test := range []struct {
		name, policy, declared, want string
	}{
		{"the declared root age", releaseAgeRenovateFixture, "0 hours", ""},
		{"another root age", releaseAgeRenovateFixture, "24 hours", "release_age_differs:/minimumReleaseAge"},
		{"a security fix held by the age", securityHeld, "0 hours", "release_age_security_fix_not_skipped"},
		{"an age of a rule's own", ownRule, "0 hours", "release_age_other_rule:/packageRules[]/minimumReleaseAge"},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			call := exec.CommandContext(ctx, "node", reader, test.declared)
			call.Dir = root
			call.Stdin = strings.NewReader(test.policy)
			output, err := call.CombinedOutput()
			if test.want == "" {
				if err != nil || len(output) != 0 {
					t.Fatalf("the reader refused a policy that holds the declared age: %v\n%s", err, output)
				}
				return
			}
			if err == nil || !strings.Contains(string(output), test.want) {
				t.Fatalf("the reader's verdict = %v\n%s\nwant it to hold %q", err, output, test.want)
			}
		})
	}
}
