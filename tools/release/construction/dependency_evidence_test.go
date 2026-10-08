package construction

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

type osvFixtureReport struct {
	ExperimentalConfig map[string]string  `json:"experimental_config,omitempty"`
	Results            []osvFixtureResult `json:"results"`
}

type osvFixtureResult struct {
	Source   osvFixtureSource    `json:"source"`
	Packages []osvFixturePackage `json:"packages"`
}

type osvFixtureSource struct {
	Path string `json:"path"`
	Type string `json:"type"`
}

type osvFixturePackage struct {
	Package         dependencyIdentity        `json:"package"`
	Licenses        []string                  `json:"licenses"`
	Vulnerabilities []osvFixtureVulnerability `json:"vulnerabilities,omitempty"`
}

type osvFixtureVulnerability struct {
	ID       string          `json:"id"`
	Modified string          `json:"modified,omitempty"`
	Aliases  []string        `json:"aliases,omitempty"`
	Summary  string          `json:"summary,omitempty"`
	Affected json.RawMessage `json:"affected,omitempty"`
}

func TestDependencyEvidenceBindsFixesToPackageAndVersionScheme(t *testing.T) {
	for _, test := range []struct {
		name     string
		affected string
		fixed    []string
	}{
		{"package releases", `[{"package":{"ecosystem":"Go","name":"example.invalid/a"},"ranges":[{"type":"SEMVER","events":[{"introduced":"0"},{"fixed":" 2.0.0 "},{"fixed":"1.2.0"}]},{"type":"ECOSYSTEM","events":[{"fixed":"1.2.0"}]}]}]`, []string{"1.2.0", "2.0.0"}},
		{"other package", `[{"package":{"ecosystem":"Go","name":"example.invalid/b"},"ranges":[{"type":"SEMVER","events":[{"fixed":"9.0.0"}]}]}]`, nil},
		{"other ecosystem", `[{"package":{"ecosystem":"npm","name":"example.invalid/a"},"ranges":[{"type":"SEMVER","events":[{"fixed":"9.0.0"}]}]}]`, nil},
		{"ecosystem wildcard", `[{"package":{"ecosystem":"Go","name":"*"},"ranges":[{"type":"ECOSYSTEM","events":[{"fixed":"2.0.0"}]}]}]`, []string{"2.0.0"}},
		{"other ecosystem wildcard", `[{"package":{"ecosystem":"npm","name":"*"},"ranges":[{"type":"ECOSYSTEM","events":[{"fixed":"9.0.0"}]}]}]`, nil},
		{"mixed affected entries", `[{"package":{"ecosystem":"npm","name":"example.invalid/a"},"ranges":[{"type":"SEMVER","events":[{"fixed":"9.0.0"}]}]},{"package":{"ecosystem":"Go","name":"example.invalid/a"},"ranges":[{"type":"GIT","events":[{"fixed":"abcdef"}]},{"type":"SEMVER","events":[{"fixed":"2.0.0"}]}]},{"package":{"ecosystem":"Go","name":"example.invalid/a"},"ranges":[{"type":"ECOSYSTEM","events":[{"fixed":"1.2.0"},{"fixed":"2.0.0"}]}]}]`, []string{"1.2.0", "2.0.0"}},
		{"git commit", `[{"package":{"ecosystem":"Go","name":"example.invalid/a"},"ranges":[{"type":"GIT","repo":"https://example.invalid/a","events":[{"fixed":"0123456789abcdef0123456789abcdef01234567"}]}]}]`, nil},
		{"unattributed range", `[{"ranges":[{"type":"SEMVER","events":[{"fixed":"9.0.0"}]}]}]`, nil},
		{"unknown range type", `[{"package":{"ecosystem":"Go","name":"example.invalid/a"},"ranges":[{"type":"OTHER","events":[{"fixed":"9.0.0"}]}]}]`, nil},
		{"no fixed release", `[{"package":{"ecosystem":"Go","name":"example.invalid/a"},"ranges":[{"type":"SEMVER","events":[{"introduced":"0"},{"fixed":" "},{"last_affected":"1.1.0"}]}]}]`, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			identity := dependencyIdentity{Ecosystem: "Go", Name: "example.invalid/a", Version: "1.0.0"}
			report := osvFixtureReport{Results: []osvFixtureResult{{
				Source: osvFixtureSource{Path: "go.mod", Type: "lockfile"},
				Packages: []osvFixturePackage{{Package: identity, Licenses: []string{"MIT"}, Vulnerabilities: []osvFixtureVulnerability{{
					ID: "OSV-1", Aliases: []string{"CVE-2", "CVE-1", "CVE-1"}, Summary: "Package advisory", Affected: json.RawMessage(test.affected),
				}}}},
			}}}
			root := t.TempDir()
			source := filepath.Join(root, "osv.json")
			if err := writeJSON(source, report); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(root, "vulnerabilities.json")
			if err := normalizeDependencyEvidence(source, target, filepath.Join(root, "licenses.json"), []string{"go.mod"}); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(target)
			if err != nil {
				t.Fatal(err)
			}
			var got vulnerabilityEvidence
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			want := vulnerabilityEvidence{SchemaVersion: 1, Sources: []vulnerabilitySource{{
				Lockfile: "go.mod", Packages: []vulnerableDependency{{dependencyIdentity: identity, Vulnerabilities: []vulnerability{{ID: "OSV-1", Aliases: []string{"CVE-1", "CVE-2"}, Summary: "Package advisory", FixedVersions: test.fixed}}}},
			}}}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("vulnerability evidence = %#v; want %#v", got, want)
			}
		})
	}
}

func dependencyReportFixture(root string) osvFixtureReport {
	return osvFixtureReport{
		ExperimentalConfig: map[string]string{"credential": "must-not-survive"},
		Results: []osvFixtureResult{
			{
				Source: osvFixtureSource{Path: filepath.Join(root, "go.mod"), Type: "lockfile"},
				Packages: []osvFixturePackage{{
					Package:  dependencyIdentity{Ecosystem: "Go", Name: "example.invalid/z", Version: "1.0.0"},
					Licenses: []string{"MIT"},
					Vulnerabilities: []osvFixtureVulnerability{
						{ID: "OSV-2", Modified: "2026-09-07T00:00:00Z"},
						{ID: "OSV-1"},
					},
				}},
			},
			{
				Source: osvFixtureSource{Path: filepath.Join(root, "package-lock.json"), Type: "lockfile"},
				Packages: []osvFixturePackage{{
					Package:  dependencyIdentity{Ecosystem: "npm", Name: "a", Version: "2.0.0"},
					Licenses: []string{"ISC"},
				}},
			},
		},
	}
}

func TestNormalizeDependencyEvidenceRemovesHostAndVolatileMetadata(t *testing.T) {
	root := filepath.Join(t.TempDir(), "checkout")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	raw := filepath.Join(t.TempDir(), "osv.json")
	encoded, err := json.Marshal(dependencyReportFixture(root))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(raw, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	vulnerabilities := filepath.Join(t.TempDir(), "vulnerabilities.json")
	licenses := filepath.Join(t.TempDir(), "licenses.json")
	if err := normalizeDependencyEvidence(raw, vulnerabilities, licenses, []string{filepath.Join(root, "go.mod"), filepath.Join(root, "package-lock.json")}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{vulnerabilities, licenses} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		for _, forbidden := range []string{root, "must-not-survive", "2026-09-07T00:00:00Z"} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s contains forbidden metadata %q: %s", path, forbidden, text)
			}
		}
	}
	vulnerabilityData, _ := os.ReadFile(vulnerabilities)
	for _, required := range []string{`"sources": [`, `"go.mod"`, `"package-lock.json"`, `"OSV-1"`, `"OSV-2"`} {
		if !strings.Contains(string(vulnerabilityData), required) {
			t.Fatalf("vulnerability report missing %q: %s", required, vulnerabilityData)
		}
	}
	licenseData, _ := os.ReadFile(licenses)
	for _, required := range []string{`"ecosystem": "Go"`, `"ecosystem": "npm"`, `"MIT"`, `"ISC"`} {
		if !strings.Contains(string(licenseData), required) {
			t.Fatalf("license report missing %q: %s", required, licenseData)
		}
	}
}

func TestDependencyEvidenceOrdersPackagesByCompleteIdentity(t *testing.T) {
	root := t.TempDir()
	identities := []dependencyIdentity{
		{Ecosystem: "npm", Name: "alpha", Version: "1.0.0"},
		{Ecosystem: "Go", Name: "zeta", Version: "2.0.0"},
		{Ecosystem: "Go", Name: "alpha", Version: "2.0.0"},
		{Ecosystem: "Go", Name: "alpha", Version: "1.0.0"},
	}
	packages := make([]osvFixturePackage, 0, len(identities))
	for _, identity := range identities {
		packages = append(packages, osvFixturePackage{
			Package: identity, Licenses: []string{"MIT"},
			Vulnerabilities: []osvFixtureVulnerability{{ID: "OSV-1"}},
		})
	}
	source := filepath.Join(root, "osv.json")
	if err := writeJSON(source, osvFixtureReport{Results: []osvFixtureResult{{
		Source:   osvFixtureSource{Path: filepath.Join(root, "go.mod"), Type: "lockfile"},
		Packages: packages,
	}}}); err != nil {
		t.Fatal(err)
	}
	vulnerabilitiesPath := filepath.Join(root, "vulnerabilities.json")
	licensesPath := filepath.Join(root, "licenses.json")
	if err := normalizeDependencyEvidence(source, vulnerabilitiesPath, licensesPath, []string{filepath.Join(root, "go.mod")}); err != nil {
		t.Fatal(err)
	}
	want := []dependencyIdentity{identities[3], identities[2], identities[1], identities[0]}
	var vulnerabilities vulnerabilityEvidence
	data, err := os.ReadFile(vulnerabilitiesPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &vulnerabilities); err != nil {
		t.Fatal(err)
	}
	got := make([]dependencyIdentity, 0, len(want))
	for _, dependency := range vulnerabilities.Sources[0].Packages {
		got = append(got, dependency.dependencyIdentity)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("vulnerability package order = %v, want %v", got, want)
	}
	var licenses licenseEvidence
	data, err = os.ReadFile(licensesPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &licenses); err != nil {
		t.Fatal(err)
	}
	got = got[:0]
	for _, dependency := range licenses.Sources[0].Packages {
		got = append(got, dependency.dependencyIdentity)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("license package order = %v, want %v", got, want)
	}
}

func TestDependencyEvidenceRejectsMissingOrMalformedScan(t *testing.T) {
	for _, test := range []struct {
		name, content, expected string
	}{
		{"missing", "", "read OSV dependency report"},
		{"malformed", "{", "decode OSV dependency report"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "osv.json")
			if test.content != "" {
				if err := os.WriteFile(source, []byte(test.content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			vulnerabilities := filepath.Join(root, "vulnerabilities.json")
			licenses := filepath.Join(root, "licenses.json")
			err := normalizeDependencyEvidence(source, vulnerabilities, licenses, []string{"go.mod"})
			if err == nil || !strings.Contains(err.Error(), test.expected) {
				t.Fatalf("%s scan error = %v", test.name, err)
			}
			for _, target := range []string{vulnerabilities, licenses} {
				if _, err := os.Stat(target); !os.IsNotExist(err) {
					t.Fatalf("invalid scan produced %s: %v", target, err)
				}
			}
		})
	}
}

func TestDependencyEvidenceRequiresCompleteSelectedSources(t *testing.T) {
	for _, scenario := range []string{
		"complete", "missing results", "partial", "foreign checkout", "foreign lockfile", "duplicate source", "non-lockfile source", "empty packages", "incomplete package identity", "missing package license", "blank package license", "unknown package license", "unasserted package license",
	} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			encoded, err := json.Marshal(dependencyReportFixture(root))
			if err != nil {
				t.Fatal(err)
			}
			var report osvFixtureReport
			if err := json.Unmarshal(encoded, &report); err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "missing results":
				report.Results = nil
			case "partial":
				report.Results = report.Results[:1]
			case "foreign checkout":
				report.Results[0].Source.Path = filepath.Join(root, "sibling", "go.mod")
			case "foreign lockfile":
				report.Results[0].Source.Path = filepath.Join(root, "other.lock")
			case "duplicate source":
				report.Results = append(report.Results, report.Results[0])
			case "non-lockfile source":
				report.Results[0].Source.Type = "directory"
			case "empty packages":
				report.Results[0].Packages = nil
			case "incomplete package identity":
				report.Results[0].Packages[0].Package.Name = ""
			case "missing package license":
				report.Results[0].Packages[0].Licenses = nil
			case "blank package license":
				report.Results[0].Packages[0].Licenses = []string{" "}
			case "unknown package license":
				report.Results[0].Packages[0].Licenses = []string{"UNKNOWN"}
			case "unasserted package license":
				report.Results[0].Packages[0].Licenses = []string{"NOASSERTION"}
			}
			source := filepath.Join(root, "osv.json")
			if err := writeJSON(source, report); err != nil {
				t.Fatal(err)
			}
			vulnerabilities, licenses := filepath.Join(root, "vulnerabilities.json"), filepath.Join(root, "licenses.json")
			err = normalizeDependencyEvidence(source, vulnerabilities, licenses, []string{filepath.Join(root, "go.mod"), filepath.Join(root, "package-lock.json")})
			if scenario == "complete" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil {
				t.Fatalf("%s dependency report was accepted", scenario)
			}
			for _, target := range []string{vulnerabilities, licenses} {
				if _, err := os.Stat(target); !os.IsNotExist(err) {
					t.Fatalf("incomplete scan wrote accepted evidence: %s, %v", target, err)
				}
			}
		})
	}
}

func TestDependencyFindingsAreNonblockingAndRetained(t *testing.T) {
	if os.Getenv("AIGW_TEST_SCANNER_TERMINAL") == "findings" {
		os.Exit(1)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(t.Context(), executable, "-test.run=^TestDependencyFindingsAreNonblockingAndRetained$")
	command.Env = append(os.Environ(), "AIGW_TEST_SCANNER_TERMINAL=findings")
	nativeExit := command.Run()
	exit, ok := errors.AsType[*exec.ExitError](nativeExit)
	if !ok || exit.ExitCode() != 1 {
		t.Fatalf("native scanner finding fixture = %v", nativeExit)
	}
	root := t.TempDir()
	calls := 0
	raw, err := scanDependencies(root, filepath.Join(root, "evidence"), dependencyPolicy{}, func(call toolCall) error {
		calls++
		if err := writeJSON(call.Args[len(call.Args)-1], dependencyReportFixture(root)); err != nil {
			return err
		}
		return fmt.Errorf("native findings: %w", nativeExit)
	})
	if err != nil || calls != 1 {
		t.Fatalf("complete findings blocked delivery or repeated the scan: error=%v calls=%d", err, calls)
	}
	for _, name := range []string{"dependencies.raw.json", "vulnerabilities.json"} {
		data, err := os.ReadFile(filepath.Join(filepath.Dir(raw), name))
		if err != nil || !strings.Contains(string(data), "OSV-1") || !strings.Contains(string(data), "OSV-2") {
			t.Fatalf("%s lost native findings: %q %v", name, data, err)
		}
	}
	var terminal struct {
		Exit     int  `json:"exit"`
		Failed   bool `json:"failed"`
		Findings bool `json:"findings"`
	}
	data, err := os.ReadFile(raw + ".exit.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &terminal); err != nil || terminal.Exit != 1 || terminal.Failed || !terminal.Findings {
		t.Fatalf("finding terminal was misclassified: %s %v", data, err)
	}
	for name, failure := range map[string]error{"deadline": context.DeadlineExceeded, "cleanup": os.ErrPermission} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			raw, err := scanDependencies(root, filepath.Join(root, "evidence"), dependencyPolicy{}, func(call toolCall) error {
				if err := writeJSON(call.Args[len(call.Args)-1], dependencyReportFixture(root)); err != nil {
					return err
				}
				return fmt.Errorf("execution: %w", errors.Join(fmt.Errorf("findings: %w", nativeExit), failure))
			})
			if !errors.Is(err, failure) {
				t.Fatalf("advisory exit swallowed the independent %s failure: %v", name, err)
			}
			data, err := os.ReadFile(raw + ".exit.json")
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &terminal); err != nil || terminal.Exit != 1 || !terminal.Failed || terminal.Findings {
				t.Fatalf("independent %s failure was classified as findings: %s %v", name, data, err)
			}
		})
	}
}

func TestReleaseDependencyAdmissionPrecedesArtifactConstruction(t *testing.T) {
	for _, failure := range []string{"scanner failure", "malformed report", "incomplete scope"} {
		t.Run(failure, func(t *testing.T) {
			root := releaseRoot(t)
			output := filepath.Join(root, "dist")
			if err := os.Mkdir(output, 0o700); err != nil {
				t.Fatal(err)
			}
			accepted := filepath.Join(output, "accepted")
			if err := os.WriteFile(accepted, []byte("previous release"), 0o600); err != nil {
				t.Fatal(err)
			}
			var calls []string
			err := buildRelease(t.Context(), buildRequest{Root: root, Output: output, Version: "1.2.3", Epoch: "1784246400", SigningKey: signingKey(t)}, func(call toolCall) error {
				calls = append(calls, call.Name)
				switch call.Name {
				case "git":
					return nil
				case "goreleaser":
					return populatePortableStage(t, call, "1.2.3", "portable_linux_amd64/aigw")
				case "syft":
					path := strings.TrimPrefix(call.Args[len(call.Args)-1], "spdx-json=")
					return os.WriteFile(path, spdxFixture(t, filepath.Dir(path), "1.2.3"), 0o600)
				case "osv-scanner":
					if failure == "scanner failure" {
						return errors.New("scanner failure")
					}
					content := `{"results":[]}`
					if failure == "malformed report" {
						content = "{"
					}
					return os.WriteFile(call.Args[len(call.Args)-1], []byte(content), 0o600)
				default:
					t.Fatalf("unadmitted dependencies reached %s", call.Name)
					return nil
				}
			})
			if err == nil || !slices.Equal(calls, []string{"git", "osv-scanner"}) {
				t.Fatalf("dependency failure reached artifact construction: error=%v calls=%v", err, calls)
			}
			if content, err := os.ReadFile(accepted); err != nil || string(content) != "previous release" {
				t.Fatalf("dependency refusal changed accepted output: %q, %v", content, err)
			}
			workspaces, err := filepath.Glob(filepath.Join(root, ".aigw-release-*"))
			if err != nil || len(workspaces) != 0 {
				t.Fatalf("dependency refusal left workspace residue: %v, %v", workspaces, err)
			}
		})
	}
}

func TestDependencyScanBindsRelativeRootToNativeAbsoluteSources(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	calls := 0
	_, err := scanDependencies(".", filepath.Join(root, "evidence"), dependencyPolicy{}, func(call toolCall) error {
		calls++
		return writeJSON(call.Args[len(call.Args)-1], dependencyReportFixture(root))
	})
	if err != nil || calls != 1 {
		t.Fatalf("relative root did not bind complete native sources: error=%v calls=%d", err, calls)
	}
}

func TestDependencyScanRejectsIncompleteNativeOutput(t *testing.T) {
	for _, failure := range []string{"missing", "malformed", "incomplete"} {
		t.Run(failure, func(t *testing.T) {
			root := t.TempDir()
			calls := 0
			_, err := scanDependencies(root, filepath.Join(root, "evidence"), dependencyPolicy{}, func(call toolCall) error {
				calls++
				report := dependencyReportFixture(root)
				switch failure {
				case "missing":
					return nil
				case "malformed":
					return os.WriteFile(call.Args[len(call.Args)-1], []byte("{"), 0o600)
				case "incomplete":
					report.Results = report.Results[:1]
				}
				return writeJSON(call.Args[len(call.Args)-1], report)
			})
			if err == nil || calls != 1 {
				t.Fatalf("incomplete native scan accepted: error=%v calls=%d", err, calls)
			}
		})
	}
}

func TestDependencyScanRetainsButRejectsNativeWarning(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "native.json")
	err := runDependencyScan(toolCall{Args: []string{output}}, func(call toolCall) error {
		_, err := fmt.Fprintln(call.Stderr, "warning: incomplete native evidence")
		return err
	})
	if err == nil {
		t.Fatal("native warning was silently accepted")
	}
	data, err := os.ReadFile(output + ".stderr")
	if err != nil || !strings.Contains(string(data), "warning:") {
		t.Fatalf("native warning was not retained: %q %v", data, err)
	}
}
