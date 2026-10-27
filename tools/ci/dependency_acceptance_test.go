//go:build dependency_acceptance

package main

import (
	"aigw-cli/tools/release/artifact"
	"aigw-cli/tools/release/construction"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The scan queries the public OSV and deps.dev services, so this test runs
// only in the network-declared dependency-acceptance quality gate.
func TestNativeDependencyEvidenceRetainsFindings(t *testing.T) {
	root, locked := nativeDependencyFixture(t)
	output := filepath.Join(root, "evidence")
	if err := construction.ScanDependencies(t.Context(), root, output); err != nil {
		t.Fatal(err)
	}
	scans, err := filepath.Glob(filepath.Join(output, "scan-*"))
	if err != nil || len(scans) != 1 {
		t.Fatalf("expected one native scan: %v %v", scans, err)
	}
	files, err := filepath.Glob(filepath.Join(scans[0], "dependencies.*.json"))
	if err != nil || len(files) != 3 {
		t.Fatalf("native raw, exit and invocation evidence incomplete: %v %v", files, err)
	}
	for _, name := range []string{"dependencies.raw.json", "vulnerabilities.json", "licenses.json"} {
		content, err := os.ReadFile(filepath.Join(scans[0], name))
		if err != nil || !json.Valid(content) {
			t.Fatalf("native evidence %s is unavailable or invalid: %v", name, err)
		}
		if name != "licenses.json" {
			if !bytes.Contains(content, []byte("GHSA-grv7-fg5c-xmjg")) {
				t.Fatalf("native evidence %s lost the selected advisory", name)
			}
		} else {
			for _, value := range []string{"go.mod", "package-lock.json", "example.invalid/dependency", "braces", "MIT"} {
				encoded, err := json.Marshal(value)
				if err != nil || !bytes.Contains(content, encoded) {
					t.Fatalf("native license evidence lost %s: %v", value, err)
				}
			}
		}
		if name != "dependencies.raw.json" {
			requireLockedScanner(t, name, content, locked)
		}
	}
	requireScanInvocation(t, scans[0], root, locked)
	requireNonblockingFindings(t, scans[0])
}

// nativeDependencyFixture writes a source with a known advisory and the
// repository's lock, so the scan admits only the scanner image that the
// repository locks for this host.
func nativeDependencyFixture(t *testing.T) (string, artifact.LockedImage) {
	t.Helper()
	root := t.TempDir()
	lock, err := os.ReadFile(filepath.Join("..", "..", ".config", "mise", "mise.lock"))
	if err != nil {
		t.Fatal(err)
	}
	locked, err := artifact.LockedScanner(lock, artifact.HostPlatform())
	if err != nil {
		t.Fatal(err)
	}
	// npm records a registry package's resolved URL; OSV Scanner 2.7 treats an
	// entry without one as a local package and queries no advisory for it.
	// braces 3.0.2 carries GHSA-grv7-fg5c-xmjg (CVE-2024-4068), fixed in 3.0.3.
	for path, value := range map[string]string{
		".config/checks/dependencies/policy.toml": "[[PackageOverrides]]\nname='example.invalid/dependency'\nversion='1.0.0'\necosystem='Go'\nlicense.override=['MIT']\n[[PackageOverrides]]\nname='braces'\nversion='3.0.2'\necosystem='npm'\nlicense.override=['MIT']\n",
		".config/mise/mise.lock":                  string(lock),
		"go.mod":                                  "module example.invalid/fixture\ngo 1.27.1\nrequire example.invalid/dependency v1.0.0\n",
		"package-lock.json":                       `{"name":"fixture","lockfileVersion":3,"packages":{"node_modules/braces":{"version":"3.0.2","resolved":"https://registry.npmjs.org/braces/-/braces-3.0.2.tgz","dev":true}}}`,
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, path), []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root, locked
}

func requireLockedScanner(t *testing.T, name string, content []byte, locked artifact.LockedImage) {
	t.Helper()
	var report struct {
		SchemaVersion int              `json:"schema_version"`
		Scanner       artifact.Scanner `json:"scanner"`
	}
	if err := json.Unmarshal(content, &report); err != nil || report.SchemaVersion != 2 || report.Scanner.Tool != artifact.ScannerTool ||
		report.Scanner.Version != locked.Version || report.Scanner.Platform != locked.Platform || report.Scanner.SHA256 != locked.SHA256 ||
		!slices.Contains(report.Scanner.Arguments, "{source}/go.mod") || !slices.Contains(report.Scanner.Arguments, "--all-vulns") {
		t.Fatalf("native %s does not name the locked scanner %+v: %+v %v", name, locked, report, err)
	}
}

func requireScanInvocation(t *testing.T, scan, root string, locked artifact.LockedImage) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(scan, "dependencies.raw.json"))
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(scan, "dependencies.raw.json.invocation.json"))
	if err != nil {
		t.Fatal(err)
	}
	var invocation struct {
		Executable string   `json:"executable"`
		SHA256     string   `json:"sha256"`
		Version    string   `json:"version"`
		Arguments  []string `json:"arguments"`
		Report     string   `json:"report_sha256"`
	}
	if err := json.Unmarshal(content, &invocation); err != nil || invocation.SHA256 != locked.SHA256 || !filepath.IsAbs(invocation.Executable) ||
		!strings.Contains(invocation.Version, "osv-scanner version: "+locked.Version+"\n") || !slices.Contains(invocation.Arguments, filepath.Join(root, "go.mod")) ||
		invocation.Report != fmt.Sprintf("%x", sha256.Sum256(raw)) {
		t.Fatalf("native scan invocation does not bind the locked scanner and its raw report: %+v %v", invocation, err)
	}
}

func requireNonblockingFindings(t *testing.T, scan string) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(scan, "dependencies.raw.json.exit.json"))
	if err != nil {
		t.Fatal(err)
	}
	var exit struct {
		Exit     int  `json:"exit"`
		Failed   bool `json:"failed"`
		Findings bool `json:"findings"`
	}
	if err := json.Unmarshal(content, &exit); err != nil || exit.Exit != 1 || exit.Failed || !exit.Findings {
		t.Fatalf("native findings must be retained and nonblocking: %+v %v", exit, err)
	}
}
