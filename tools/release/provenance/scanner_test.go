package provenance

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const schemaOneReport = "{\"schema_version\":1,\"sources\":[]}\n"

func scannerLock(version, platform, checksum, url string) string {
	return fmt.Sprintf("[[tools.%q]]\nversion = %q\n\n[tools.%q.%q]\nchecksum = %q\nurl = %q\n",
		ScannerTool, version, ScannerTool, "platforms."+platform, checksum, url)
}

func scannerReport(t *testing.T, scanner *Scanner) string {
	t.Helper()
	data, err := json.Marshal(struct {
		SchemaVersion int      `json:"schema_version"`
		Scanner       *Scanner `json:"scanner,omitempty"`
		Sources       []string `json:"sources"`
	}{2, scanner, []string{}})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func writeDependencyReports(t *testing.T, candidate, version, content string) {
	t.Helper()
	for _, report := range []string{"vulnerabilities", "licenses"} {
		if err := os.WriteFile(filepath.Join(candidate, "aigw_"+version+"."+report+".json"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPlatformNamesFollowMiseLockPlatforms(t *testing.T) {
	for platform, want := range map[[2]string]string{
		{"darwin", "arm64"}: "macos-arm64", {"darwin", "amd64"}: "macos-x64",
		{"linux", "arm64"}: "linux-arm64", {"linux", "amd64"}: "linux-x64",
		{"windows", "arm64"}: "windows-arm64", {"windows", "amd64"}: "windows-x64",
	} {
		if got := platformName(platform[0], platform[1]); got != want {
			t.Fatalf("platform %v = %q, want %q", platform, got, want)
		}
	}
	if HostPlatform() != platformName(runtime.GOOS, runtime.GOARCH) {
		t.Fatalf("host platform = %q", HostPlatform())
	}
}

// The repository's own lock is the input that scanner admission reads.
func TestLockedScannerReadsEveryPlatformOfTheRepositoryLock(t *testing.T) {
	lock, err := os.ReadFile(filepath.Join("..", "..", "..", ".config", "mise", "mise.lock"))
	if err != nil {
		t.Fatal(err)
	}
	for _, platform := range []string{"linux-arm64", "linux-x64", "macos-arm64", "macos-x64", "windows-arm64", "windows-x64"} {
		image, err := LockedScanner(lock, platform)
		if err != nil {
			t.Fatalf("%s: %v", platform, err)
		}
		release := "https://github.com/google/osv-scanner/releases/download/v" + image.Version + "/osv-scanner_"
		if image.Version == "" || image.Platform != platform || !isSHA256(image.SHA256) || !strings.HasPrefix(image.URL, release) {
			t.Fatalf("%s locked image = %+v", platform, image)
		}
	}
}

func TestLockedScannerRefusesAnIncompleteSelection(t *testing.T) {
	digest := strings.Repeat("a", 64)
	complete := scannerLock("2.7.0", "linux-x64", "sha256:"+digest, "https://example.invalid/osv-scanner")
	want := LockedImage{Version: "2.7.0", Platform: "linux-x64", URL: "https://example.invalid/osv-scanner", SHA256: digest}
	if image, err := LockedScanner([]byte(complete), "linux-x64"); err != nil || image != want {
		t.Fatalf("complete lock = %+v, %v", image, err)
	}
	for name, lock := range map[string]string{
		"malformed":         "[[tools",
		"absent tool":       "[[tools.other]]\nversion = \"1.0.0\"\n",
		"two versions":      complete + "\n[[tools.\"" + ScannerTool + "\"]]\nversion = \"2.8.0\"\n",
		"absent platform":   strings.Replace(complete, "linux-x64", "linux-arm64", 1),
		"other algorithm":   strings.Replace(complete, "sha256:", "blake3:", 1),
		"short digest":      strings.Replace(complete, digest, "aa", 1),
		"uppercase digest":  strings.Replace(complete, digest, strings.ToUpper(digest), 1),
		"absent URL":        strings.Replace(complete, "url = ", "mirror = ", 1),
		"absent version":    strings.Replace(complete, "version = ", "release = ", 1),
		"non-string checks": strings.Replace(complete, `"sha256:`+digest+`"`, "1", 1),
	} {
		if image, err := LockedScanner([]byte(lock), "linux-x64"); err == nil {
			t.Fatalf("%s lock selected %+v", name, image)
		}
	}
}

func TestProvenanceListsTheLockedScannerItsReportsName(t *testing.T) {
	digest := strings.Repeat("c", 64)
	url := "https://example.invalid/osv-scanner_linux_amd64"
	locked := Scanner{Tool: ScannerTool, Version: "2.7.0", Platform: "linux-x64", SHA256: digest, Arguments: []string{"scan", "--lockfile", "{source}/go.mod"}}
	write := func(t *testing.T, reports string, lock string) (string, error) {
		t.Helper()
		root, candidate := provenanceFixture(t)
		if err := os.WriteFile(filepath.Join(root, ".config", "mise", "mise.lock"), []byte(lock), 0o600); err != nil {
			t.Fatal(err)
		}
		writeDependencyReports(t, candidate, "1.2.3", reports)
		target := filepath.Join(candidate, "aigw_1.2.3.provenance.json")
		err := WriteProvenance(root, candidate, target, "1.2.3", strings.Repeat("a", 40), strings.Repeat("b", 40))
		data, _ := os.ReadFile(target)
		return string(data), err
	}
	lock := scannerLock("2.7.0", "linux-x64", "sha256:"+digest, url)
	text, err := write(t, scannerReport(t, &locked), lock)
	if err != nil {
		t.Fatal(err)
	}
	var statement provenanceStatement
	if err := json.Unmarshal([]byte(text), &statement); err != nil {
		t.Fatal(err)
	}
	dependencies := statement.Predicate.BuildDefinition.ResolvedDependencies
	if len(dependencies) != 6 || dependencies[5].URI != url || len(dependencies[5].Digest) != 1 || dependencies[5].Digest["sha256"] != digest {
		t.Fatalf("provenance does not list the locked scanner: %+v", dependencies)
	}
	text, err = write(t, schemaOneReport, lock)
	var historical provenanceStatement
	if err != nil || json.Unmarshal([]byte(text), &historical) != nil || len(historical.Predicate.BuildDefinition.ResolvedDependencies) != 5 {
		t.Fatalf("schema 1 reports gained a scanner dependency: %v %s", err, text)
	}
	other := locked
	other.Arguments = []string{"scan"}
	unlocked := locked
	unlocked.SHA256 = strings.Repeat("d", 64)
	foreign := locked
	foreign.Tool = "github:example/scanner"
	for name, test := range map[string]struct{ reports, lock, diagnostic string }{
		"malformed report":          {"{", lock, "decode dependency report"},
		"schema 2 without scanner":  {scannerReport(t, nil), lock, "schema 2"},
		"schema 1 with scanner":     {strings.Replace(scannerReport(t, &locked), `"schema_version":2`, `"schema_version":1`, 1), lock, "schema 1"},
		"unknown schema":            {strings.Replace(schemaOneReport, "1", "3", 1), lock, "schema 3"},
		"unlocked image":            {scannerReport(t, &unlocked), lock, "not the image"},
		"foreign tool":              {scannerReport(t, &foreign), lock, "not the image"},
		"other version":             {scannerReport(t, &locked), strings.Replace(lock, "2.7.0", "2.8.0", 1), "not the image"},
		"platform absent from lock": {scannerReport(t, &locked), strings.Replace(lock, "linux-x64", "linux-arm64", 1), "no complete"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := write(t, test.reports, test.lock); err == nil || !strings.Contains(err.Error(), test.diagnostic) {
				t.Fatalf("provenance accepted %s: %v", name, err)
			}
		})
	}
	t.Run("reports disagree", func(t *testing.T) {
		root, candidate := provenanceFixture(t)
		if err := os.WriteFile(filepath.Join(root, ".config", "mise", "mise.lock"), []byte(lock), 0o600); err != nil {
			t.Fatal(err)
		}
		writeDependencyReports(t, candidate, "1.2.3", scannerReport(t, &locked))
		if err := os.WriteFile(filepath.Join(candidate, "aigw_1.2.3.licenses.json"), []byte(scannerReport(t, &other)), 0o600); err != nil {
			t.Fatal(err)
		}
		err := WriteProvenance(root, candidate, filepath.Join(candidate, "aigw_1.2.3.provenance.json"), "1.2.3", strings.Repeat("a", 40), strings.Repeat("b", 40))
		if err == nil || !strings.Contains(err.Error(), "different scanners") {
			t.Fatalf("disagreeing reports accepted: %v", err)
		}
	})
	if _, err := scannerDependency([]byte(lock), t.TempDir(), "1.2.3"); err == nil || !strings.Contains(err.Error(), "read dependency report") {
		t.Fatalf("absent dependency reports accepted: %v", err)
	}
}
