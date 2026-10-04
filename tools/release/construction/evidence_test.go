package construction

import (
	"aigw-cli/tools/release/artifact"
	"aigw-cli/tools/release/readiness"
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"
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

func TestGoReleaserArchiveMetadataIsHostIndependent(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", ".config", "release", "goreleaser.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var configuration struct {
		Archives []struct {
			BuildsInfo struct {
				Owner string `yaml:"owner"`
				Group string `yaml:"group"`
			} `yaml:"builds_info"`
			Files []struct {
				Source string `yaml:"src"`
				Info   struct {
					Owner string `yaml:"owner"`
					Group string `yaml:"group"`
				} `yaml:"info"`
			} `yaml:"files"`
		} `yaml:"archives"`
	}
	if err := yaml.Unmarshal(data, &configuration); err != nil {
		t.Fatal(err)
	}
	if len(configuration.Archives) != 1 {
		t.Fatalf("archives = %d, want 1", len(configuration.Archives))
	}
	archive := configuration.Archives[0]
	if archive.BuildsInfo.Owner != "root" || archive.BuildsInfo.Group != "root" {
		t.Fatalf("build archive identity = %q:%q, want root:root", archive.BuildsInfo.Owner, archive.BuildsInfo.Group)
	}
	for _, file := range archive.Files {
		if file.Info.Owner != "root" || file.Info.Group != "root" {
			t.Fatalf("archive identity for %s = %q:%q, want root:root", file.Source, file.Info.Owner, file.Info.Group)
		}
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
	var finalSBOM string
	observed := errors.New("native SBOM observed before other release evidence")
	err := buildRelease(t.Context(), buildRequest{Root: root, Output: filepath.Join(root, "dist"), Version: version, Epoch: "1784246400", SigningKey: "unused"}, func(call toolCall) error {
		switch call.Name {
		case "git":
			if slices.Contains(call.Args, "HEAD^{commit}") {
				var readErr error
				sbom, readErr = os.ReadFile(finalSBOM)
				return cmp.Or(readErr, observed)
			}
			return nil
		case "goreleaser":
			stage := goReleaserStage(t, call.Args)
			finalSBOM = filepath.Join(filepath.Dir(stage), "artifacts", "aigw_"+version+".spdx.json")
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
			return nil
		case "osv-scanner":
			return writeJSON(call.Args[len(call.Args)-1], dependencyReportFixture(root))
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

func TestReleaseBuildPropagatesPostBuildValidationFailures(t *testing.T) {
	for _, boundary := range []string{"decode Syft", "selected lockfiles"} {
		t.Run(boundary, func(t *testing.T) {
			root := releaseRoot(t)
			output := filepath.Join(root, "dist")
			if err := os.Mkdir(output, 0o700); err != nil {
				t.Fatal(err)
			}
			accepted := filepath.Join(output, "accepted")
			if err := os.WriteFile(accepted, []byte("previous release"), 0o600); err != nil {
				t.Fatal(err)
			}
			valid := buildRequest{Root: root, Output: output, Version: "1.2.3", Epoch: "1784246400", SigningKey: "unused"}
			err := buildRelease(t.Context(), valid, func(call toolCall) error {
				switch call.Name {
				case "git":
					return nil
				case "goreleaser":
					return populatePortableStage(t, call, valid.Version, "portable_linux_amd64/aigw")
				case "syft":
					data := spdxFixture(t, filepath.Dir(strings.TrimPrefix(call.Args[len(call.Args)-1], "spdx-json=")), valid.Version)
					if boundary == "decode Syft" {
						data = []byte("{")
					}
					return os.WriteFile(strings.TrimPrefix(call.Args[len(call.Args)-1], "spdx-json="), data, 0o600)
				case "osv-scanner":
					content, err := json.Marshal(dependencyReportFixture(root))
					if err != nil {
						return err
					}
					if boundary == "selected lockfiles" {
						content = []byte(`{"results":[]}`)
					}
					return os.WriteFile(call.Args[len(call.Args)-1], content, 0o600)
				default:
					t.Fatalf("invalid evidence reached later release tool %s", call.Name)
					return nil
				}
			})
			if err == nil || !strings.Contains(err.Error(), boundary) {
				t.Fatalf("release validation error = %v, want %s", err, boundary)
			}
			if data, err := os.ReadFile(accepted); err != nil || string(data) != "previous release" {
				t.Fatalf("invalid evidence changed accepted output: %q, %v", data, err)
			}
			entries, err := os.ReadDir(output)
			if err != nil || len(entries) != 1 {
				t.Fatalf("partial evidence published: %v, %v", entries, err)
			}
			workspaces, err := filepath.Glob(filepath.Join(root, ".aigw-release-*"))
			if err != nil || len(workspaces) != 0 {
				t.Fatalf("failed release left workspace residue: %v, %v", workspaces, err)
			}
		})
	}
}

func TestReleaseSBOMRequiresPortableBinaries(t *testing.T) {
	valid := buildRequest{Root: releaseRoot(t), Output: filepath.Join(t.TempDir(), "dist"), Version: "1.2.3", Epoch: "1784246400", SigningKey: "unused"}
	err := buildRelease(t.Context(), valid, func(call toolCall) error {
		if call.Name == "osv-scanner" {
			return writeJSON(call.Args[len(call.Args)-1], dependencyReportFixture(valid.Root))
		}
		if call.Name == "syft" {
			path := strings.TrimPrefix(call.Args[len(call.Args)-1], "spdx-json=")
			return os.WriteFile(path, []byte(`{"spdxVersion":"SPDX-2.3"}`), 0o600)
		}
		if call.Name != "goreleaser" {
			return nil
		}
		stage := goReleaserStage(t, call.Args)
		if err := os.MkdirAll(stage, 0o700); err != nil {
			return err
		}
		for _, name := range artifact.Archives(valid.Version) {
			if err := os.WriteFile(filepath.Join(stage, name), []byte(name), 0o600); err != nil {
				return err
			}
		}
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "binary matrix") {
		t.Fatalf("missing binary error = %v", err)
	}
}

func TestNativeDependencyEvidenceTransportRefusesBrokenResources(t *testing.T) {
	root := t.TempDir()
	if _, err := readDependencyPolicy(root); err == nil {
		t.Fatal("absent dependency policy accepted")
	}
	policy := filepath.Join(root, ".config", "checks", "dependencies", "policy.toml")
	if err := os.MkdirAll(filepath.Dir(policy), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policy, []byte("unexpected = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readDependencyPolicy(root); err == nil {
		t.Fatal("unknown native policy field accepted")
	}
	approved := "[[IgnoredVulns]]\nid='GHSA-vfj7-8cjw-p6xm'\nreason='Reviewed fixture'\nignoreUntil=2026-10-18\n"
	if err := os.WriteFile(policy, []byte(approved), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readDependencyPolicy(root); err == nil {
		t.Fatal("exception accepted without its lockfile")
	}
	lock := filepath.Join(root, "package-lock.json")
	for _, invalid := range []string{"{", `{"lockfileVersion":3,"packages":{"node_modules/braces":{"version":"3.0.4","dev":true}}}`, `{"lockfileVersion":3,"packages":{}}`} {
		if err := os.WriteFile(lock, []byte(invalid), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := readDependencyPolicy(root); err == nil {
			t.Fatal("exception accepted outside its exact locked scope")
		}
	}
	blocked := filepath.Join(root, "blocked")
	if err := os.WriteFile(blocked, []byte("retained"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := scanDependencies(root, blocked, dependencyPolicy{}, func(toolCall) error {
		t.Fatal("scanner started before its evidence parent was admitted")
		return nil
	}); err == nil {
		t.Fatal("blocked evidence parent accepted")
	}
	for _, raw := range []string{"missing", "malformed"} {
		path := filepath.Join(root, raw)
		if raw == "malformed" {
			if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if err := validateDependencyExceptions(dependencyPolicy{}, path); err == nil {
			t.Fatal("unavailable raw evidence accepted")
		}
	}
	problem := errors.New("native scanner unavailable")
	output := filepath.Join(root, "unavailable", "raw.json")
	if err := runDependencyScan(toolCall{Args: []string{output}}, func(toolCall) error { return problem }); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing capture parent accepted: %v", err)
	}
	output = filepath.Join(root, "raw.json")
	if err := os.Mkdir(output+".stderr", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := runDependencyScan(toolCall{Args: []string{output}}, func(toolCall) error { return problem }); err == nil {
		t.Fatal("invalid stderr target accepted")
	}
	if err := os.Remove(output + ".stderr"); err != nil {
		t.Fatal(err)
	}
	if err := runDependencyScan(toolCall{Args: []string{output}}, func(toolCall) error { return problem }); !errors.Is(err, problem) {
		t.Fatalf("native failure lost: %v", err)
	}
	if err := os.Remove(output + ".exit.json"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(output+".exit.json", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := runDependencyScan(toolCall{Args: []string{output}}, func(toolCall) error { return nil }); err == nil {
		t.Fatal("unwritable exit evidence accepted")
	}
}
