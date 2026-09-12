package construction

import (
	"aigw-cli/tools/release/artifact"
	"aigw-cli/tools/release/readiness"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func spdxFixture(version string) []byte {
	files := make([]string, 0, len(artifact.Archives(version)))
	for _, archive := range artifact.Archives(version) {
		name := strings.TrimSuffix(strings.TrimSuffix(archive, ".tar.gz"), ".zip")
		files = append(files, fmt.Sprintf(`{"fileName":%q}`, name+"/aigw"))
	}
	return []byte(`{"spdxVersion":"SPDX-2.3","documentNamespace":"https://volatile.invalid/random","creationInfo":{"created":"2099-01-01T00:00:00Z"},"files":[` + strings.Join(files, ",") + `]}`)
}

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

func TestDependencyEvidenceRequiresCompleteSelectedSources(t *testing.T) {
	for _, scenario := range []string{
		"complete", "missing results", "partial", "foreign checkout", "foreign lockfile", "duplicate source", "non-lockfile source", "empty packages",
	} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			encoded, err := dependencyReportFixture(root)
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

func TestNormalizedSPDXIsDeterministicAndPortable(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "raw.json")
	if err := os.WriteFile(source, spdxFixture("1.2.3"), 0o600); err != nil {
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
	raw := bytes.Replace(spdxFixture("1.2.3"), []byte(`"creationInfo":{"created":"2099-01-01T00:00:00Z"},`), nil, 1)
	if err := os.WriteFile(missingCreation, raw, 0o600); err != nil {
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

func TestSPDXRequiresTheCompletePlatformMatrix(t *testing.T) {
	root := t.TempDir()
	source, target := filepath.Join(root, "raw.json"), filepath.Join(root, "normalized.json")
	for _, count := range []int{0, 1, len(artifact.Archives("1.2.3")) - 1, len(artifact.Archives("1.2.3")) + 1} {
		files := make([]map[string]string, count)
		for index := range files {
			files[index] = map[string]string{"fileName": fmt.Sprintf("platform-%d/aigw", index)}
		}
		encoded, err := json.Marshal(map[string]any{"spdxVersion": "SPDX-2.3", "files": files})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(source, encoded, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := normalizeSPDX(source, target, "1.2.3", time.Unix(1784246400, 0).UTC()); err == nil || !strings.Contains(err.Error(), "binary matrix") {
			t.Fatalf("incomplete or extra platform inventory (%d): %v", count, err)
		}
		if _, err := os.Stat(target); !os.IsNotExist(err) {
			t.Fatalf("invalid matrix created an accepted SBOM: %v", err)
		}
	}
}

func TestReleaseSBOMCatalogsEveryNativeBinary(t *testing.T) {
	root := releaseRoot(t)
	for name, content := range map[string]string{
		"go.mod": "module fixture\n", "main.go": "package main\nfunc main() {}\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	version := "1.2.3"
	expected := make(map[string]string)
	var sbom []byte
	observed := errors.New("native SBOM observed before other release evidence")
	err := buildRelease(buildRequest{Root: root, Output: filepath.Join(root, "dist"), Version: version, Epoch: "1784246400", SigningKey: "unused"}, func(call toolCall) error {
		switch call.Name {
		case "git":
			return nil
		case "goreleaser":
			stage := goReleaserStage(t, call.Args)
			for _, archive := range artifact.Archives(version) {
				platform := strings.Split(strings.TrimSuffix(strings.TrimSuffix(archive, ".tar.gz"), ".zip"), "_")
				name := "aigw"
				if platform[2] == "windows" {
					name += ".exe"
				}
				relative := filepath.Join(platform[2]+"-"+platform[3], name)
				target := filepath.Join(stage, relative)
				command := exec.Command("go", "build", "-trimpath", "-o", target, ".")
				command.Dir = root
				command.Env = append(os.Environ(), "GOOS="+platform[2], "GOARCH="+platform[3], "CGO_ENABLED=0")
				if output, err := command.CombinedOutput(); err != nil {
					t.Fatalf("build native fixture %s: %v\n%s", relative, err, output)
				}
				data, err := os.ReadFile(target)
				if err != nil {
					t.Fatal(err)
				}
				expected[filepath.ToSlash(relative)] = fmt.Sprintf("%x", sha256.Sum256(data))
				if err := os.WriteFile(filepath.Join(stage, archive), []byte(archive), 0o600); err != nil {
					return err
				}
			}
			return nil
		case "syft":
			command := exec.Command(call.Name, call.Args...)
			command.Dir = call.Directory
			if output, err := command.CombinedOutput(); err != nil || len(output) != 0 {
				t.Fatalf("native Syft failed or emitted diagnostics: %v\n%s", err, output)
			}
			source := strings.TrimPrefix(call.Args[len(call.Args)-1], "spdx-json=")
			data, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			sbom = data
			return nil
		case "osv-scanner":
			return observed
		default:
			return fmt.Errorf("unexpected release tool %s", call.Name)
		}
	})
	if !errors.Is(err, observed) {
		t.Fatalf("release did not reach native SBOM acceptance: %v", err)
	}
	var document struct {
		Files []struct {
			Name      string `json:"fileName"`
			Checksums []struct {
				Algorithm string `json:"algorithm"`
				Value     string `json:"checksumValue"`
			} `json:"checksums"`
		} `json:"files"`
	}
	if err := json.Unmarshal(sbom, &document); err != nil {
		t.Fatal(err)
	}
	for _, file := range document.Files {
		for _, sum := range file.Checksums {
			if sum.Algorithm != "SHA256" {
				continue
			}
			if expected[file.Name] != sum.Value {
				t.Fatalf("SBOM file not bound to native artifact: %+v", file)
			}
			delete(expected, file.Name)
		}
	}
	if len(expected) != 0 {
		t.Fatalf("SBOM omitted platform artifacts: %v", expected)
	}
}
