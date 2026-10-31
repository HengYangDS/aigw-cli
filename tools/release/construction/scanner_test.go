package construction

import (
	"aigw-cli/tools/release/provenance"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// lockedScannerFixture is the scanner image that construction tests lock for
// this host.
var lockedScannerFixture = provenance.LockedImage{
	Version: "2.7.0", Platform: provenance.HostPlatform(),
	URL: "https://example.invalid/osv-scanner", SHA256: strings.Repeat("5", 64),
}

// scannedInputs selects lockedScannerFixture with the default dependency policy.
var scannedInputs = dependencyInputs{Scanner: lockedScannerFixture}

// reportScannerFixture is the scanner that normalization tests name.
var reportScannerFixture = provenance.Scanner{
	Tool: provenance.ScannerTool, Version: "2.7.0", Platform: "linux-x64", SHA256: strings.Repeat("5", 64), Arguments: []string{"scan", "source"},
}

// matrixFixtureContent is the content of a fixture release file: its name, or
// a schema 1 report for the dependency reports, which provenance reads.
func matrixFixtureContent(name string) []byte {
	if strings.HasSuffix(name, ".vulnerabilities.json") || strings.HasSuffix(name, ".licenses.json") {
		return []byte(`{"schema_version":1,"sources":[]}`)
	}
	return []byte(name)
}

// scannerLockFixture is a Mise lock that selects lockedScannerFixture.
var scannerLockFixture = fmt.Sprintf("[[tools.%q]]\nversion = %q\n\n[tools.%q.%q]\nchecksum = \"sha256:%s\"\nurl = %q\n",
	provenance.ScannerTool, lockedScannerFixture.Version, provenance.ScannerTool, "platforms."+lockedScannerFixture.Platform,
	lockedScannerFixture.SHA256, lockedScannerFixture.URL)

// withLockedScanner runs a fake behind the locked scanner fixture: it answers
// the version report and observes the locked image for every scanner call, so
// the fake handles only the scan itself.
func withLockedScanner(run toolRunner) toolRunner {
	return func(call toolCall) error {
		if call.Name != scannerName {
			return run(call)
		}
		if call.Image != nil {
			*call.Image = toolImage{Path: "/locked/osv-scanner", SHA256: lockedScannerFixture.SHA256}
		}
		if slices.Equal(call.Args, []string{"--version"}) {
			_, err := fmt.Fprintf(call.Stdout, "osv-scanner version: %s\ncommit: fixture\n", lockedScannerFixture.Version)
			return err
		}
		return run(call)
	}
}

func writeScannerLock(t *testing.T, root string) {
	t.Helper()
	lock := filepath.Join(root, ".config", "mise", "mise.lock")
	if err := os.MkdirAll(filepath.Dir(lock), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lock, []byte(scannerLockFixture), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestDependencyInputsSelectTheHostScannerOfTheLock(t *testing.T) {
	root := releaseRoot(t)
	inputs, err := readDependencyInputs(root)
	if err != nil || inputs.Scanner != lockedScannerFixture {
		t.Fatalf("dependency inputs = %+v, %v", inputs, err)
	}
	lock := filepath.Join(root, ".config", "mise", "mise.lock")
	if err := os.WriteFile(lock, []byte(strings.ReplaceAll(scannerLockFixture, lockedScannerFixture.Platform, "plan9-mips")), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readDependencyInputs(root); err == nil || !strings.Contains(err.Error(), "no complete") {
		t.Fatalf("a lock without the host scanner was accepted: %v", err)
	}
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}
	if _, err := readDependencyInputs(root); err == nil || !strings.Contains(err.Error(), "read Mise lock") {
		t.Fatalf("an absent lock was accepted: %v", err)
	}
}

// A scan admits only the locked image; every refusal comes before either
// normalized report is written.
func TestDependencyScanAdmitsOnlyTheLockedScannerImage(t *testing.T) {
	for name, test := range map[string]struct {
		probe      toolImage
		version    string
		probeErr   error
		scan       toolImage
		diagnostic string
	}{
		"unlocked image":  {toolImage{"/other/osv-scanner", strings.Repeat("6", 64)}, "2.7.0", nil, toolImage{}, "not the 2.7.0 image"},
		"other version":   {toolImage{"/locked/osv-scanner", lockedScannerFixture.SHA256}, "2.8.0", nil, toolImage{}, `reports version "2.8.0"`},
		"no version":      {toolImage{"/locked/osv-scanner", lockedScannerFixture.SHA256}, "", nil, toolImage{}, `reports version ""`},
		"version failure": {toolImage{}, "", errors.New("scanner unavailable"), toolImage{}, "scanner unavailable"},
		"image changed":   {toolImage{"/locked/osv-scanner", lockedScannerFixture.SHA256}, "2.7.0", nil, toolImage{"/later/osv-scanner", lockedScannerFixture.SHA256}, "changed"},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			scans := 0
			_, err := scanDependencies(root, filepath.Join(root, "evidence"), scannedInputs, func(call toolCall) error {
				if slices.Equal(call.Args, []string{"--version"}) {
					*call.Image = test.probe
					if test.version != "" {
						_, _ = fmt.Fprintf(call.Stdout, "osv-scanner version: %s\n", test.version)
					}
					return test.probeErr
				}
				scans++
				*call.Image = test.scan
				return writeJSON(call.Args[len(call.Args)-1], dependencyReportFixture(root))
			})
			if err == nil || !strings.Contains(err.Error(), test.diagnostic) {
				t.Fatalf("scanner admission error = %v, want %q", err, test.diagnostic)
			}
			if test.scan == (toolImage{}) && scans != 0 {
				t.Fatalf("an unadmitted scanner ran %d scans", scans)
			}
			for _, report := range []string{"vulnerabilities.json", "licenses.json"} {
				if written, _ := filepath.Glob(filepath.Join(root, "evidence", "scan-*", report)); len(written) != 0 {
					t.Fatalf("a refused scanner wrote %v", written)
				}
			}
		})
	}
}

func TestDependencyReportsNameTheirScannerAndTheScanRetainsItsInvocation(t *testing.T) {
	root := t.TempDir()
	var scanned toolCall
	raw, err := scanDependencies(root, filepath.Join(root, "evidence"), scannedInputs, withLockedScanner(func(call toolCall) error {
		scanned = call
		return writeJSON(call.Args[len(call.Args)-1], dependencyReportFixture(root))
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := provenance.Scanner{
		Tool: provenance.ScannerTool, Version: "2.7.0", Platform: provenance.HostPlatform(), SHA256: lockedScannerFixture.SHA256,
		Arguments: []string{
			"scan", "source", "--config", "{evidence}/policy.toml", "--lockfile", "{source}/go.mod", "--lockfile", "{source}/package-lock.json",
			"--no-call-analysis=go", "--format", "json", "--all-packages", "--all-vulns", "--licenses=", "--output-file", "{evidence}/dependencies.raw.json",
		},
	}
	for _, report := range []string{"vulnerabilities.json", "licenses.json"} {
		data, err := os.ReadFile(filepath.Join(filepath.Dir(raw), report))
		if err != nil {
			t.Fatal(err)
		}
		var named struct {
			SchemaVersion int                `json:"schema_version"`
			Scanner       provenance.Scanner `json:"scanner"`
		}
		if err := json.Unmarshal(data, &named); err != nil || named.SchemaVersion != 2 || !slices.Equal(named.Scanner.Arguments, want.Arguments) ||
			named.Scanner.Tool != want.Tool || named.Scanner.Version != want.Version || named.Scanner.Platform != want.Platform || named.Scanner.SHA256 != want.SHA256 {
			t.Fatalf("%s names scanner %+v (schema %d), want %+v: %v", report, named.Scanner, named.SchemaVersion, want, err)
		}
		if strings.Contains(string(data), root) {
			t.Fatalf("%s contains the host path %s", report, root)
		}
	}
	data, err := os.ReadFile(raw + ".invocation.json")
	if err != nil {
		t.Fatal(err)
	}
	var invocation scanInvocation
	if err := json.Unmarshal(data, &invocation); err != nil {
		t.Fatal(err)
	}
	rawData, err := os.ReadFile(raw)
	if err != nil {
		t.Fatal(err)
	}
	if invocation.Executable != "/locked/osv-scanner" || invocation.SHA256 != lockedScannerFixture.SHA256 || invocation.Platform != provenance.HostPlatform() ||
		invocation.Version != "osv-scanner version: 2.7.0\ncommit: fixture\n" || !slices.Equal(invocation.Arguments, scanned.Args) ||
		invocation.Report != fmt.Sprintf("%x", sha256.Sum256(rawData)) {
		t.Fatalf("scan invocation = %+v", invocation)
	}
}

func TestPortableArgumentsNameTheirRoots(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source")
	inside := filepath.Join(source, "build", "scan-1")
	outside := filepath.Join(t.TempDir(), "scan-2")
	args := []string{"--config", filepath.Join(inside, "policy.toml"), "--lockfile", filepath.Join(source, "go.mod"), filepath.Join(source+"-sibling", "go.mod"), "--licenses="}
	if got := portableArguments(args, source, inside); !slices.Equal(got, []string{"--config", "{evidence}/policy.toml", "--lockfile", "{source}/go.mod", args[4], "--licenses="}) {
		t.Fatalf("evidence inside the source = %v", got)
	}
	args[1] = filepath.Join(outside, "nested", "policy.toml")
	if got := portableArguments(args, source, outside); got[1] != "{evidence}/nested/policy.toml" || got[3] != "{source}/go.mod" || args[1] == got[1] {
		t.Fatalf("evidence outside the source = %v", got)
	}
}

func TestDependencyScanBindsRelativeRootToNativeAbsoluteSources(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	calls := 0
	_, err := scanDependencies(".", filepath.Join(root, "evidence"), scannedInputs, withLockedScanner(func(call toolCall) error {
		calls++
		return writeJSON(call.Args[len(call.Args)-1], dependencyReportFixture(root))
	}))
	if err != nil || calls != 1 {
		t.Fatalf("relative root did not bind complete native sources: error=%v calls=%d", err, calls)
	}
}

func TestDependencyScanRejectsIncompleteNativeOutput(t *testing.T) {
	for _, failure := range []string{"missing", "malformed", "incomplete", "unreadable"} {
		t.Run(failure, func(t *testing.T) {
			root := t.TempDir()
			calls := 0
			_, err := scanDependencies(root, filepath.Join(root, "evidence"), scannedInputs, withLockedScanner(func(call toolCall) error {
				calls++
				report := dependencyReportFixture(root)
				switch failure {
				case "missing":
					return nil
				case "malformed":
					return os.WriteFile(call.Args[len(call.Args)-1], []byte("{"), 0o600)
				case "unreadable":
					return os.Mkdir(call.Args[len(call.Args)-1], 0o700)
				case "incomplete":
					report.Results = report.Results[:1]
				}
				return writeJSON(call.Args[len(call.Args)-1], report)
			}))
			if err == nil || calls != 1 || (failure == "unreadable") != strings.HasPrefix(err.Error(), "scan dependencies:") {
				t.Fatalf("incomplete native scan accepted or misattributed: error=%v calls=%d", err, calls)
			}
		})
	}
}

func TestDependencyScanRetainsButRejectsNativeWarning(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "native.json")
	_, err := runDependencyScan(toolCall{Args: []string{output}}, func(call toolCall) error {
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

// The live scan runs only in the network-declared dependency-acceptance gate;
// these refusals happen before the scanner is started.
func TestScanDependenciesRefusesUnusableInputsBeforeScanning(t *testing.T) {
	if err := ScanDependencies(t.Context(), t.TempDir(), t.TempDir()); err == nil || !strings.Contains(err.Error(), "read dependency policy") {
		t.Fatalf("missing dependency policy accepted: %v", err)
	}
	root := t.TempDir()
	policy := filepath.Join(root, ".config", "checks", "dependencies", "policy.toml")
	if err := os.MkdirAll(filepath.Dir(policy), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policy, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ScanDependencies(t.Context(), root, t.TempDir()); err == nil || !strings.Contains(err.Error(), "read Mise lock") {
		t.Fatalf("missing Mise lock accepted: %v", err)
	}
	writeScannerLock(t, root)
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, []byte("not a directory\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ScanDependencies(t.Context(), root, filepath.Join(blocked, "evidence")); err == nil || strings.Contains(err.Error(), "Mise lock") {
		t.Fatalf("unusable evidence output accepted: %v", err)
	}
}
