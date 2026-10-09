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

func spdxFixture(t *testing.T, root, version string) []byte {
	t.Helper()
	files := make([]string, 0, len(artifact.Archives(version)))
	for _, archive := range artifact.Archives(version) {
		name := strings.TrimSuffix(strings.TrimSuffix(archive, ".tar.gz"), ".zip")
		relative := name + "/aigw"
		target := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(relative), 0o600); err != nil {
			t.Fatal(err)
		}
		files = append(files, fmt.Sprintf(`{"fileName":%q,"checksums":[{"algorithm":"SHA1","checksumValue":"0000000000000000000000000000000000000000"}]}`, relative))
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

func TestRenderGoReleaserConfigRejectsUnwritableDestination(t *testing.T) {
	root := releaseRoot(t)
	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := renderGoReleaserConfig(root, blocked, t.TempDir()); err == nil || !strings.Contains(err.Error(), "write GoReleaser config") {
		t.Fatalf("unwritable config error = %v", err)
	}
}

func TestNormalizedSPDXIsDeterministicAndPortable(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "raw.json")
	if err := os.WriteFile(source, spdxFixture(t, root, "1.2.3"), 0o600); err != nil {
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
	var normalized struct {
		Files []struct {
			Name      string `json:"fileName"`
			Checksums []struct {
				Algorithm string `json:"algorithm"`
				Value     string `json:"checksumValue"`
			} `json:"checksums"`
		} `json:"files"`
	}
	if err := json.Unmarshal(leftData, &normalized); err != nil {
		t.Fatal(err)
	}
	for _, file := range normalized.Files {
		want := fmt.Sprintf("%x", sha256.Sum256([]byte(file.Name)))
		found := false
		for _, sum := range file.Checksums {
			if sum.Algorithm == "SHA256" && sum.Value == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("normalized SBOM lacks measured SHA256 for %s", file.Name)
		}
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
	raw := bytes.Replace(spdxFixture(t, spdxRoot, "1.2.3"), []byte(`"creationInfo":{"created":"2099-01-01T00:00:00Z"},`), nil, 1)
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

func TestSPDXBindingRequiresOwnedBytes(t *testing.T) {
	root := t.TempDir()
	data := []byte("native executable")
	if err := os.WriteFile(filepath.Join(root, "aigw"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	matching := fmt.Sprintf(`[{"algorithm":"SHA256","checksumValue":"%x"}]`, sha256.Sum256(data))
	for _, test := range []struct {
		name, encoded string
		valid         bool
	}{
		{"missing checksum", `[{"fileName":"aigw"}]`, true},
		{"scan root path", `[{"fileName":"/aigw"}]`, true},
		{"Windows scan root path", `[{"fileName":"\\aigw"}]`, true},
		{"Windows traversal", `[{"fileName":"\\..\\aigw"}]`, false},
		{"network path", `[{"fileName":"\\\\host\\aigw"}]`, false},
		{"root-relative duplicate", `[{"fileName":"aigw"},{"fileName":"\\aigw"}]`, false},
		{"measured checksum", `[{"fileName":"aigw","checksums":` + matching + `}]`, true},
		{"mismatch", `[{"fileName":"aigw","checksums":[{"algorithm":"SHA256","checksumValue":"wrong"}]}]`, false},
		{"foreign path", `[{"fileName":"../aigw"}]`, false},
		{"duplicate", `[{"fileName":"aigw"},{"fileName":"./aigw"}]`, false},
		{"missing binary", `[{"fileName":"absent"}]`, false},
		{"missing path", `[{}]`, false},
		{"invalid entry", `[null]`, false},
		{"invalid checksum", `[{"fileName":"aigw","checksums":[null]}]`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			var files []any
			if err := json.Unmarshal([]byte(test.encoded), &files); err != nil {
				t.Fatal(err)
			}
			if err := bindSPDXFiles(root, files); (err == nil) != test.valid {
				t.Fatalf("valid=%t binding error=%v", test.valid, err)
			}
			if test.valid {
				entry, ok := files[0].(map[string]any)
				if !ok || entry["fileName"] != "aigw" {
					t.Fatalf("scan-relative path was not normalized: %v", files)
				}
			}
		})
	}
	if err := bindSPDXFiles(filepath.Join(root, "absent"), nil); err == nil {
		t.Fatal("absent build root was accepted")
	}
	if actual, err := os.ReadFile(filepath.Join(root, "aigw")); err != nil || !bytes.Equal(actual, data) {
		t.Fatalf("SBOM binding changed native bytes: %q, %v", actual, err)
	}
}

func TestReleaseSBOMCatalogsEveryNativeBinary(t *testing.T) {
	root := releaseRoot(t)
	for name, content := range map[string]string{
		"go.mod": "module fixture\n", "main.go": "package main\nfunc main() {}\n",
		".syft.yaml": "exclude: ['**/*']\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	version := "1.2.3"
	expected := make(map[string]string)
	var sbom []byte
	var sbomPath string
	observed := errors.New("native SBOM observed before other release evidence")
	err := buildRelease(t.Context(), buildRequest{Root: root, Output: filepath.Join(root, "dist"), Version: version, Epoch: "1784246400", SigningKey: "unused"}, func(call toolCall) error {
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
			stage := filepath.Dir(strings.TrimPrefix(call.Args[len(call.Args)-1], "spdx-json="))
			sbomPath = filepath.Join(filepath.Dir(stage), "artifacts", "aigw_"+version+".spdx.json")
			command := exec.Command(call.Name, call.Args...)
			command.Dir = call.Directory
			if output, err := command.CombinedOutput(); err != nil || len(output) != 0 {
				t.Fatalf("native Syft failed or emitted diagnostics: %v\n%s", err, output)
			}
			return nil
		case "osv-scanner":
			var err error
			sbom, err = os.ReadFile(sbomPath)
			if err != nil {
				t.Fatal(err)
			}
			return observed
		default:
			return fmt.Errorf("unexpected release tool %s", call.Name)
		}
	})
	if !errors.Is(err, observed) {
		t.Fatalf("release did not reach native SBOM acceptance: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(root, ".syft.yaml")); err != nil || string(data) != "exclude: ['**/*']\n" {
		t.Fatalf("release changed caller configuration: %q, %v", data, err)
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

func TestReleaseBuildEnvironment(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("1.2.3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte("# Changelog\n\nThis project follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and [Semantic Versioning](https://semver.org/).\n\n## [Unreleased]\n\n## [1.2.3] - 2026-08-09\n\n### Fixed\n\n- Fix.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	for name, value := range map[string]string{
		"AIGW_GITLAB_RELEASE_ORIGIN":     "https://gitlab.example",
		"AIGW_GITLAB_RELEASE_REPOSITORY": "group/aigw-cli",
		"AIGW_GITHUB_RELEASE_ORIGIN":     "https://github.example",
		"AIGW_GITHUB_RELEASE_REPOSITORY": "org/aigw-cli",
	} {
		t.Setenv(name, value)
	}
	request, err := buildRequestFromEnvironment(t.Context(), "dist")
	if err != nil {
		t.Fatal(err)
	}
	requestRoot, requestRootErr := os.Stat(request.Root)
	wantRoot, wantRootErr := os.Stat(root)
	if request.Version != "1.2.3" || request.Epoch != "1786233600" || request.Output != "dist" || requestRootErr != nil || wantRootErr != nil || !os.SameFile(requestRoot, wantRoot) {
		t.Fatalf("request = %#v", request)
	}
	missingVersion := t.TempDir()
	if err := os.Chdir(missingVersion); err != nil {
		t.Fatal(err)
	}
	if _, err := buildRequestFromEnvironment(t.Context(), "dist"); err == nil || !strings.Contains(err.Error(), "read VERSION") {
		t.Fatalf("missing VERSION error = %v", err)
	}
	missingChronology := t.TempDir()
	if err := os.WriteFile(filepath.Join(missingChronology, "VERSION"), []byte("1.2.3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(missingChronology); err != nil {
		t.Fatal(err)
	}
	if _, err := buildRequestFromEnvironment(t.Context(), "dist"); err == nil || !strings.Contains(err.Error(), "open CHANGELOG") {
		t.Fatalf("missing release chronology error = %v", err)
	}
}

func TestMacOSDistributionRequiresExplicitIdentity(t *testing.T) {
	if err := VerifyMacOSDistribution(t.Context(), t.TempDir(), "1.2.3", "", Notarization{}); err == nil {
		t.Fatal("distribution accepted without explicit publisher")
	}
	if err := VerifyMacOSDistribution(t.Context(), t.TempDir(), "invalid", strings.Repeat("a", 40), Notarization{Archive: "upload.zip", SubmissionID: "submission", KeychainProfile: "profile"}); err == nil {
		t.Fatal("invalid version accepted")
	}
}

func TestReleaseEpochRejectsInvalidDateAndOversizedChangelogLine(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CI_COMMIT_TAG", "v1.2.3")
	changelog := filepath.Join(root, "CHANGELOG.md")
	if err := os.WriteFile(changelog, []byte("## [1.2.3] - 2026-99-99\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveReleaseEpoch(t.Context(), root, "1.2.3"); err == nil {
		t.Fatal("invalid release date was accepted")
	}
	if err := os.WriteFile(changelog, []byte(strings.Repeat("x", 70*1024)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveReleaseEpoch(t.Context(), root, "1.2.3"); err == nil || !strings.Contains(err.Error(), "token too long") {
		t.Fatalf("oversized changelog error = %v", err)
	}
}
