package construction

import (
	"aigw-cli/tools/release/readiness"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
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
			if err := normalizeDependencyEvidence(source, target, filepath.Join(root, "licenses.json")); err != nil {
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

func dependencyReportFixture(root string) ([]byte, error) {
	return json.Marshal(osvFixtureReport{
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
	})
}

func TestNormalizeDependencyEvidenceRemovesHostAndVolatileMetadata(t *testing.T) {
	root := filepath.Join(t.TempDir(), "checkout")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	raw := filepath.Join(t.TempDir(), "osv.json")
	encoded, err := dependencyReportFixture(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(raw, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	vulnerabilities := filepath.Join(t.TempDir(), "vulnerabilities.json")
	licenses := filepath.Join(t.TempDir(), "licenses.json")
	if err := normalizeDependencyEvidence(raw, vulnerabilities, licenses); err != nil {
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

func TestNormalizedSPDXIsDeterministicAndPortable(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "raw.json")
	raw := `{"spdxVersion":"SPDX-2.3","documentNamespace":"https://volatile.invalid/random","creationInfo":{"created":"2099-01-01T00:00:00Z"},"files":[{"fileName":"aigw","sourceInfo":"acquired from /aigw"}]}`
	if err := os.WriteFile(source, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	instant, err := readiness.ParseEpoch("1784246400")
	if err != nil {
		t.Fatal(err)
	}
	left, right := filepath.Join(root, "left.json"), filepath.Join(root, "right.json")
	if err := normalizeSPDX(source, left, "1.2.3", instant); err != nil {
		t.Fatal(err)
	}
	if err := normalizeSPDX(source, right, "1.2.3", instant); err != nil {
		t.Fatal(err)
	}
	leftData, _ := os.ReadFile(left)
	rightData, _ := os.ReadFile(right)
	if !bytes.Equal(leftData, rightData) {
		t.Fatal("normalized SPDX differs across equivalent runs")
	}
	for _, forbidden := range []string{"/Users/", "/private/tmp/"} {
		if bytes.Contains(leftData, []byte(forbidden)) {
			t.Fatalf("normalized SPDX leaks host path %q", forbidden)
		}
	}
	hostLocal := filepath.Join(root, "host-local.json")
	if err := os.WriteFile(hostLocal, []byte(`{"sourceInfo":"/Users/alice/project"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := normalizeSPDX(hostLocal, filepath.Join(root, "rejected.json"), "1.2.3", instant); err == nil {
		t.Fatal("host-local SPDX path was accepted")
	}
}

func TestSPDXInputFailures(t *testing.T) {
	spdxRoot := t.TempDir()
	instant := time.Unix(1784246400, 0).UTC()
	missingCreation := filepath.Join(spdxRoot, "missing-creation.json")
	if err := os.WriteFile(missingCreation, []byte(`{"spdxVersion":"SPDX-2.3"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(spdxRoot, "normalized.json")
	if err := normalizeSPDX(missingCreation, target, "1.2.3", instant); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil || !bytes.Contains(data, []byte(`"creationInfo"`)) {
		t.Fatalf("normalized SPDX = %s error=%v", data, err)
	}
	malformed := filepath.Join(spdxRoot, "malformed.json")
	if err := os.WriteFile(malformed, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := normalizeSPDX(malformed, target, "1.2.3", instant); err == nil || !strings.Contains(err.Error(), "decode") {
		t.Fatalf("malformed SPDX error = %v", err)
	}
	if err := normalizeSPDX(filepath.Join(spdxRoot, "absent.json"), target, "1.2.3", instant); err == nil || !strings.Contains(err.Error(), "read Syft") {
		t.Fatalf("missing SPDX error = %v", err)
	}
}
