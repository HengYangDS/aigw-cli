package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const validPolicy = `owner = "product-toolchain"
source = "repository architecture layout"
go_roots = ["cmd", "internal", "tools"]
composition_root_files = { "internal/cli" = ["app.go"] }
peer_package_roots = { "internal/cli" = ["invocation"] }
flat_directory_limit = 8
max_file_eloc = 700
max_directory_eloc = 3600
max_file_complexity = 180
max_directory_complexity = 900
max_test_file_eloc = 800
max_test_file_complexity = 200
max_function_eloc = 200
max_function_complexity = 100
max_nesting_depth = 20
max_test_function_eloc = 220
max_test_function_complexity = 120
max_test_nesting_depth = 20
suffix_flat_group_min = 3
platform_build_suffixes = ["unix", "windows", "darwin", "linux", "posix"]
ignore_roots = ["vendor", ".git", "records", "build"]
ignore_directory_names = ["vendor", ".git", "records", "runtime", "node_modules"]
check_exported_type_alias = true
check_function_var_alias = true
check_package_documentation = false
check_trivial_wrappers = true
`

type rejectingWriter struct{}

func (rejectingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write rejected")
}

func writePolicy(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "policy.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func decodeReport(t *testing.T, raw string) Report {
	t.Helper()
	var report Report
	if err := json.Unmarshal([]byte(raw), &report); err != nil {
		t.Fatalf("decode report: %v\nraw=%s", err, raw)
	}
	return report
}

func findingRules(report Report) []string {
	rules := make([]string, 0, len(report.Findings))
	for _, finding := range report.Findings {
		rules = append(rules, finding.Rule)
	}
	return rules
}

func hasRule(report Report, rule string) bool {
	for _, finding := range report.Findings {
		if finding.Rule == rule {
			return true
		}
	}
	return false
}

func TestRunCleanFixture(t *testing.T) {
	root := t.TempDir()
	policyPath := writePolicy(t, root, validPolicy)
	writeFile(t, filepath.Join(root, "scripts", "check", "ok.sh"), "#!/bin/sh\n")
	writeFile(t, filepath.Join(root, "internal", "pkg", "core.go"), "// Package pkg owns the fixture behavior.\npackage pkg\n\nfunc Hello() string { return \"ok\" }\n")
	writeFile(t, filepath.Join(root, "cmd", "tool", "main.go"), "// Command tool runs the fixture.\npackage main\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(root, "tools", "helper", "main.go"), "// Command helper runs the fixture.\npackage main\n\nfunc main() {}\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"-root", root, "-policy", policyPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q stdout=%s", code, stderr.String(), stdout.String())
	}
	report := decodeReport(t, stdout.String())
	if !report.OK || report.Summary["total"] != 0 {
		t.Fatalf("report=%+v", report)
	}
}

func TestRunDetectsELOCAndComplexityDebt(t *testing.T) {
	root := t.TempDir()
	body := strings.Replace(validPolicy, "max_file_eloc = 700", "max_file_eloc = 4", 1)
	body = strings.Replace(body, "max_directory_eloc = 3600", "max_directory_eloc = 9", 1)
	body = strings.Replace(body, "max_file_complexity = 180", "max_file_complexity = 1", 1)
	body = strings.Replace(body, "max_directory_complexity = 900", "max_directory_complexity = 2", 1)
	policyPath := writePolicy(t, root, body)
	writeFile(t, filepath.Join(root, "scripts", "check", "ok.sh"), "#!/bin/sh\n")
	writeFile(t, filepath.Join(root, "internal", "mixed", "large.go"), `package mixed

func classify(value int) string {
	if value < 0 {
		return "negative"
	}
	if value == 0 {
		return "zero"
	}
	return "positive"
}
`)
	writeFile(t, filepath.Join(root, "internal", "mixed", "peer.go"), `package mixed

func enabled(value bool) bool {
	if value {
		return true
	}
	return false
}
`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"-root", root, "-policy", policyPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("code=%d want 1 stderr=%q stdout=%s", code, stderr.String(), stdout.String())
	}
	report := decodeReport(t, stdout.String())
	for _, rule := range []string{"file_eloc", "directory_eloc", "file_complexity", "directory_complexity"} {
		if !hasRule(report, rule) {
			t.Fatalf("missing rule %s in %v\nstdout=%s", rule, findingRules(report), stdout.String())
		}
	}
	var foundStats bool
	for _, stat := range report.DirectoryStats {
		if stat.Path == "internal/mixed" {
			foundStats = stat.ProductionELOC > 0 && stat.ProductionComplexity > 0
		}
	}
	if !foundStats {
		t.Fatalf("missing ELOC/complexity directory stats: %+v", report.DirectoryStats)
	}
}

func TestRunDetectsFunctionBudgets(t *testing.T) {
	root := t.TempDir()
	body := strings.Replace(validPolicy, "max_test_function_eloc = 220", "max_test_function_eloc = 4", 1)
	body = strings.Replace(body, "max_test_function_complexity = 120", "max_test_function_complexity = 1", 1)
	body = strings.Replace(body, "max_test_nesting_depth = 20", "max_test_nesting_depth = 1", 1)
	policyPath := writePolicy(t, root, body)
	writeFile(t, filepath.Join(root, "internal", "domain", "behavior_test.go"), `package domain
func TestBehavior(value int) bool {
	if value > 0 {
		if value > 1 {
			return true
		}
	}
	return false
}
func TestPeer(value int) bool {
	if value < 0 { return true }
	return false
}
`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := run([]string{"-root", root, "-policy", policyPath}, &stdout, &stderr); code != 1 {
		t.Fatalf("code=%d want 1 stderr=%q stdout=%s", code, stderr.String(), stdout.String())
	}
	report := decodeReport(t, stdout.String())
	for _, rule := range []string{"function_eloc", "function_complexity", "function_nesting"} {
		if !hasRule(report, rule) {
			t.Fatalf("missing rule %s in %v\nstdout=%s", rule, findingRules(report), stdout.String())
		}
	}
}

func TestRunDetectsCoreViolations(t *testing.T) {
	root := t.TempDir()
	policyPath := writePolicy(t, root, validPolicy)

	// flat directory over limit (9 production files)
	for i := 1; i <= 9; i++ {
		writeFile(t, filepath.Join(root, "internal", "flat", "f"+itoa(i)+".go"), "package flat\n")
	}
	// many tests must not hide production flatness
	for i := 1; i <= 5; i++ {
		writeFile(t, filepath.Join(root, "internal", "flat", "f"+itoa(i)+"_test.go"), "package flat\n")
	}

	// suffix-flat group
	writeFile(t, filepath.Join(root, "internal", "sfx", "foo_a.go"), "package sfx\n")
	writeFile(t, filepath.Join(root, "internal", "sfx", "foo_b.go"), "package sfx\n")
	writeFile(t, filepath.Join(root, "internal", "sfx", "foo_c.go"), "package sfx\n")

	// forbidden directory name
	writeFile(t, filepath.Join(root, "internal", "shims", "launcher.go"), "package shims\n\nfunc X() {}\n")

	// composition root behavior outside its declared assembler
	writeFile(t, filepath.Join(root, "internal", "cli", "app.go"), "package cli\n")
	writeFile(t, filepath.Join(root, "internal", "cli", "setup.go"), "package cli\n")
	writeFile(t, filepath.Join(root, "internal", "cli", "account", "account.go"), "package account\n\nimport _ \"fixture/internal/cli/profile\"\n")
	writeFile(t, filepath.Join(root, "internal", "cli", "profile", "profile.go"), "package profile\n\nimport _ \"fixture/internal/cli/invocation\"\n")

	// AST violations
	writeFile(t, filepath.Join(root, "internal", "alias", "alias.go"), `package alias

import "fmt"

type Config = fmt.Stringer

var Println func(a ...any) (n int, err error) = fmt.Println

var Sprint = fmt.Sprint

func Print(a ...any) (n int, err error) {
	return fmt.Print(a...)
}
`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"-root", root, "-policy", policyPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("code=%d want 1 stderr=%q", code, stderr.String())
	}
	report := decodeReport(t, stdout.String())
	if report.OK {
		t.Fatal("expected ok=false")
	}
	for _, rule := range []string{
		"flat_directory",
		"suffix_flat",
		"composition_root_file",
		"peer_package_import",
		"exported_type_alias",
		"function_var_alias",
		"trivial_wrapper",
	} {
		if !hasRule(report, rule) {
			t.Fatalf("missing rule %s in %v\nstdout=%s", rule, findingRules(report), stdout.String())
		}
	}
	// flat finding should include production count and not use tests to pass
	foundFlat := false
	for _, finding := range report.Findings {
		if finding.Rule == "flat_directory" {
			foundFlat = true
			if finding.Count != 9 || finding.Limit != 8 {
				t.Fatalf("flat finding=%+v", finding)
			}
			if finding.Line != 0 && finding.Path == "" {
				t.Fatal("flat path missing")
			}
		}
		if finding.Rule == "exported_type_alias" && finding.Line < 1 {
			t.Fatalf("type alias missing line: %+v", finding)
		}
		if finding.Rule == "trivial_wrapper" && finding.Line < 1 {
			t.Fatalf("wrapper missing line: %+v", finding)
		}
	}
	if !foundFlat {
		t.Fatal("missing flat_directory finding details")
	}
}

func TestNegativeAndEdgeCases(t *testing.T) {
	root := t.TempDir()
	policyPath := writePolicy(t, root, validPolicy)

	// scripts only semantic subdir — OK
	writeFile(t, filepath.Join(root, "scripts", "check", "a.sh"), "#!/bin/sh\n")

	// platform variants collapse — not suffix-flat
	writeFile(t, filepath.Join(root, "internal", "plat", "files_unix.go"), "package plat\n")
	writeFile(t, filepath.Join(root, "internal", "plat", "files_windows.go"), "package plat\n")
	writeFile(t, filepath.Join(root, "internal", "plat", "files_darwin.go"), "package plat\n")
	writeFile(t, filepath.Join(root, "internal", "plat", "files_linux.go"), "package plat\n")
	writeFile(t, filepath.Join(root, "internal", "plat", "files_posix.go"), "package plat\n")

	// two suffix modules only — under threshold
	writeFile(t, filepath.Join(root, "internal", "pair", "foo_a.go"), "package pair\n")
	writeFile(t, filepath.Join(root, "internal", "pair", "foo_b.go"), "package pair\n")

	// exactly 8 production files — OK
	for i := 1; i <= 8; i++ {
		writeFile(t, filepath.Join(root, "internal", "limit", "f"+itoa(i)+".go"), "package limit\n")
	}

	// ignored roots must not be scanned
	writeFile(t, filepath.Join(root, "vendor", "internal", "shims", "x.go"), "package shims\n")
	writeFile(t, filepath.Join(root, "build", "runtime", "shims", "x.go"), "package shims\n")
	writeFile(t, filepath.Join(root, "internal", "vendor", "x.go"), "package vendor\n")
	writeFile(t, filepath.Join(root, "internal", "pkg", "runtime", "x.go"), "package runtime\n")

	// non-wrapper: has logic
	writeFile(t, filepath.Join(root, "internal", "wrap", "ok.go"), `package wrap

import "fmt"

func Print(a ...any) (n int, err error) {
	if len(a) == 0 {
		return 0, nil
	}
	return fmt.Print(a...)
}

// unexported alias/type are allowed
type config = fmt.Stringer

var println func(a ...any) (n int, err error) = fmt.Println

// value var is not a function alias
var DefaultName = "x"

// constructor call is not an alias
var Buf = bytesBuffer()

func bytesBuffer() string { return "" }

// method not checked
type T struct{}

func (T) Print(a ...any) (n int, err error) {
	return fmt.Print(a...)
}

// defined type (not alias) OK
type Named fmt.Stringer
`)

	// parse error file still produces finding
	writeFile(t, filepath.Join(root, "internal", "bad", "bad.go"), "package bad\nfunc (\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"-root", root, "-policy", policyPath}, &stdout, &stderr)
	report := decodeReport(t, stdout.String())
	if hasRule(report, "suffix_flat") {
		t.Fatalf("platform/pair should not suffix-flat: %v", findingRules(report))
	}
	if hasRule(report, "flat_directory") {
		t.Fatalf("limit-8 should pass: %v", findingRules(report))
	}
	if hasRule(report, "trivial_wrapper") {
		t.Fatalf("logic wrapper false positive: %v\n%s", findingRules(report), stdout.String())
	}
	if hasRule(report, "exported_type_alias") {
		t.Fatalf("unexported alias false positive: %v", findingRules(report))
	}
	if hasRule(report, "function_var_alias") {
		t.Fatalf("value/constructor false positive: %v", findingRules(report))
	}
	if !hasRule(report, "go_parse_error") {
		t.Fatalf("expected parse error finding, got %v", findingRules(report))
	}
	if code != 1 {
		t.Fatalf("code=%d want 1 for parse error", code)
	}
}

func TestPolicyValidationAndCLI(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name string
		body string
		args []string
		want string
		code int
	}{
		{name: "unknown flag", args: []string{"-unknown"}, want: "flag provided but not defined", code: 2},
		{name: "positional", args: []string{"extra"}, want: "does not accept positional", code: 2},
		{name: "missing policy", args: []string{"-root", root, "-policy", filepath.Join(root, "missing.toml")}, want: "load architecture policy", code: 1},
		{name: "unknown field", body: validPolicy + "extra = 1\n", want: "load architecture policy", code: 1},
		{name: "empty owner", body: strings.Replace(validPolicy, "product-toolchain", "", 1), want: "owner and source", code: 1},
		{name: "empty go roots", body: strings.Replace(validPolicy, `go_roots = ["cmd", "internal", "tools"]`, `go_roots = []`, 1), want: "go_roots", code: 1},
		{name: "bad flat limit", body: strings.Replace(validPolicy, "flat_directory_limit = 8", "flat_directory_limit = 0", 1), want: "flat_directory_limit", code: 1},
		{name: "bad file ELOC", body: strings.Replace(validPolicy, "max_file_eloc = 700", "max_file_eloc = 0", 1), want: "ELOC limits", code: 1},
		{name: "bad directory ELOC", body: strings.Replace(validPolicy, "max_directory_eloc = 3600", "max_directory_eloc = 699", 1), want: "ELOC limits", code: 1},
		{name: "bad file complexity", body: strings.Replace(validPolicy, "max_file_complexity = 180", "max_file_complexity = 0", 1), want: "complexity limits", code: 1},
		{name: "bad directory complexity", body: strings.Replace(validPolicy, "max_directory_complexity = 900", "max_directory_complexity = 179", 1), want: "complexity limits", code: 1},
		{name: "bad suffix min", body: strings.Replace(validPolicy, "suffix_flat_group_min = 3", "suffix_flat_group_min = 1", 1), want: "suffix_flat_group_min", code: 1},
		{name: "abs go root", body: strings.Replace(validPolicy, `go_roots = ["cmd", "internal", "tools"]`, `go_roots = ["/tmp/x"]`, 1), want: "go_roots", code: 1},
		{name: "windows drive go root", body: strings.Replace(validPolicy, `go_roots = ["cmd", "internal", "tools"]`, `go_roots = ["C:/tmp/x"]`, 1), want: "go_roots", code: 1},
		{name: "windows relative drive go root", body: strings.Replace(validPolicy, `go_roots = ["cmd", "internal", "tools"]`, `go_roots = ["C:tmp/x"]`, 1), want: "go_roots", code: 1},
		{name: "unc go root", body: strings.Replace(validPolicy, `go_roots = ["cmd", "internal", "tools"]`, `go_roots = ["//server/share"]`, 1), want: "go_roots", code: 1},
		{name: "parent traversal go root", body: strings.Replace(validPolicy, `go_roots = ["cmd", "internal", "tools"]`, `go_roots = ["internal/../cmd"]`, 1), want: "go_roots", code: 1},
		{name: "windows composition root", body: strings.Replace(validPolicy, `composition_root_files = { "internal/cli" = ["app.go"] }`, `composition_root_files = { "C:/internal/cli" = ["app.go"] }`, 1), want: "composition_root_files", code: 1},
		{name: "duplicate composition file", body: strings.Replace(validPolicy, `composition_root_files = { "internal/cli" = ["app.go"] }`, `composition_root_files = { "internal/cli" = ["app.go", "app.go"] }`, 1), want: "composition_root_files", code: 1},
		{name: "parent peer root", body: strings.Replace(validPolicy, `peer_package_roots = { "internal/cli" = ["invocation"] }`, `peer_package_roots = { "internal/../cli" = ["invocation"] }`, 1), want: "peer_package_roots", code: 1},
		{name: "empty platform", body: strings.Replace(validPolicy, `platform_build_suffixes = ["unix", "windows", "darwin", "linux", "posix"]`, `platform_build_suffixes = []`, 1), want: "platform_build_suffixes", code: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := test.args
			if test.body != "" {
				args = []string{"-root", root, "-policy", writePolicy(t, t.TempDir(), test.body)}
			}
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := run(args, &stdout, &stderr)
			if code != test.code {
				t.Fatalf("code=%d want %d stderr=%q", code, test.code, stderr.String())
			}
			if !strings.Contains(stderr.String(), test.want) {
				t.Fatalf("stderr=%q want %q", stderr.String(), test.want)
			}
		})
	}
}

func TestRunRejectsInvalidRootPath(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := run([]string{"-root", "invalid\x00root"}, &stdout, &stderr); code != 1 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "invalid argument") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestWriteReportFailure(t *testing.T) {
	root := t.TempDir()
	policyPath := writePolicy(t, root, validPolicy)
	writeFile(t, filepath.Join(root, "scripts", "check", "a.sh"), "ok\n")
	writeFile(t, filepath.Join(root, "internal", "pkg", "core.go"), "package pkg\n")
	var stderr bytes.Buffer
	code := run([]string{"-root", root, "-policy", policyPath}, rejectingWriter{}, &stderr)
	if code != 1 {
		t.Fatalf("code=%d want 1", code)
	}
	if !strings.Contains(stderr.String(), "write report") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestAbsolutePolicyPath(t *testing.T) {
	root := t.TempDir()
	policyPath := writePolicy(t, root, validPolicy)
	writeFile(t, filepath.Join(root, "scripts", "check", "a.sh"), "ok\n")
	writeFile(t, filepath.Join(root, "internal", "pkg", "core.go"), "package pkg\n")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"-root", root, "-policy", policyPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	report := decodeReport(t, stdout.String())
	if report.Policy == "" {
		t.Fatal("empty policy path in report")
	}
}

func TestDecisionRecordDuplicateSequence(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "docs", "decisions")
	writeFile(t, filepath.Join(directory, "README.md"), "[A](dr-0001-a.md)\n[B](dr-0001-b.md)\n")
	body := "# DR-0001: Fixture\n\n- Status: Accepted\n- Date: 2026-08-08\n\n## Context\nX\n\n## Decision\nX\n\n## Consequences\nX\n\n## Revisit Trigger\nX\n"
	writeFile(t, filepath.Join(directory, "dr-0001-a.md"), body)
	writeFile(t, filepath.Join(directory, "dr-0001-b.md"), body)
	report := newReport("policy", root)
	if err := checkDecisionRecords(root, &report); err != nil {
		t.Fatal(err)
	}
	if !hasRule(report, "decision_record_sequence_duplicate") {
		t.Fatalf("duplicate sequence was not reported: %+v", report.Findings)
	}
}

func TestTrackedFilesRejectsBrokenRepositoryMetadata(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := trackedFiles(root); err == nil || !strings.Contains(err.Error(), "list tracked files") {
		t.Fatalf("error=%v", err)
	}
}

func TestWorkspaceFilesSkipsGitMetadataDirectories(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".git-shadow", "private"), "ignored")
	writeFile(t, filepath.Join(root, "docs", "kept.md"), "kept")
	files, err := workspaceFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != "docs/kept.md" {
		t.Fatalf("files=%v", files)
	}
}

func TestTextLayoutSkipsPythonSources(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "fixture.py"), "print('fixture')\r\n")
	report := newReport("policy", root)
	if err := checkTextLayout(root, &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("Python source entered the text-layout plane: %+v", report.Findings)
	}
}

func TestSuffixFlatPlatformPartialStillGroupsSemantic(t *testing.T) {
	root := t.TempDir()
	policyPath := writePolicy(t, root, validPolicy)
	writeFile(t, filepath.Join(root, "scripts", "check", "a.sh"), "ok\n")
	// foo_report platform variants collapse to one semantic key; plus foo_native and foo_index => 3
	writeFile(t, filepath.Join(root, "internal", "sfx", "foo_report_unix.go"), "package sfx\n")
	writeFile(t, filepath.Join(root, "internal", "sfx", "foo_report_windows.go"), "package sfx\n")
	writeFile(t, filepath.Join(root, "internal", "sfx", "foo_native.go"), "package sfx\n")
	writeFile(t, filepath.Join(root, "internal", "sfx", "foo_index.go"), "package sfx\n")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"-root", root, "-policy", policyPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("code=%d", code)
	}
	report := decodeReport(t, stdout.String())
	if !hasRule(report, "suffix_flat") {
		t.Fatalf("expected suffix_flat, got %v", findingRules(report))
	}
	for _, finding := range report.Findings {
		if finding.Rule == "suffix_flat" && finding.Count != 3 {
			t.Fatalf("count=%d want 3 files=%v", finding.Count, finding.Files)
		}
	}
}

func TestCollapseAndHelpers(t *testing.T) {
	platform := map[string]struct{}{"unix": {}, "windows": {}}
	if got := collapsePlatformSuffixes("files_unix", platform); got != "files" {
		t.Fatalf("got %q", got)
	}
	if got := collapsePlatformSuffixes("files_unix_windows", platform); got != "files" {
		t.Fatalf("got %q", got)
	}
	if got := collapsePlatformSuffixes("foo_report", platform); got != "foo_report" {
		t.Fatalf("got %q", got)
	}
	if !isTestGoFile("a_test.go") || isTestGoFile("a.go") {
		t.Fatal("isTestGoFile")
	}
	if !isIdentPrefix("foo") || isIdentPrefix("") || isIdentPrefix("1x") {
		t.Fatal("isIdentPrefix")
	}
	if !isExportedIdent("Foo") || isExportedIdent("foo") || isExportedIdent("") {
		t.Fatal("isExportedIdent")
	}
	if startsWithDotDot("..") != true || startsWithDotDot("pkg") {
		t.Fatal("startsWithDotDot")
	}
	if runtime.GOOS == "windows" {
		if !startsWithDotDot("..\\x") {
			t.Fatal("windows dotdot")
		}
	} else if !startsWithDotDot("../x") {
		t.Fatal("unix dotdot")
	}
}

func TestStartsWithDotDotAcceptsBothPortableSeparators(t *testing.T) {
	for _, value := range []string{"..", "../x", `..\x`} {
		if !startsWithDotDot(value) {
			t.Fatalf("%q was not recognized", value)
		}
	}
	for _, value := range []string{".", "pkg", ".../x"} {
		if startsWithDotDot(value) {
			t.Fatalf("%q was recognized", value)
		}
	}
}

func TestReportFinalizeStable(t *testing.T) {
	report := newReport("p.toml", "/tmp/root")
	report.addFinding(Finding{Rule: "b", Path: "z", Message: "m2"})
	report.addFinding(Finding{Rule: "a", Path: "y", Message: "m1", Line: 2})
	report.addFinding(Finding{Rule: "a", Path: "y", Message: "m0", Line: 1})
	var buf bytes.Buffer
	if err := writeReport(&buf, report); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `"ok": false`) {
		t.Fatalf("out=%s", out)
	}
	// rule a before b
	if idxA, idxB := strings.Index(out, `"rule": "a"`), strings.Index(out, `"rule": "b"`); idxA < 0 || idxB < 0 || idxA > idxB {
		t.Fatalf("unstable order: %s", out)
	}
}

func TestLoadPolicyRepoDefaultShape(t *testing.T) {
	// Ensure the checked-in policy path shape is loadable when present relative to module.
	// This test uses an embedded copy equivalent rather than depending on cwd.
	dir := t.TempDir()
	path := writePolicy(t, dir, validPolicy)
	p, err := loadPolicy(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.FlatDirectoryLimit != 8 || p.MaxFileELOC != 700 || p.MaxDirectoryELOC != 3600 || p.MaxFileComplexity != 180 || p.MaxDirectoryComplexity != 900 || p.SuffixFlatGroupMin != 3 {
		t.Fatalf("%+v", p)
	}
}

func TestDisabledASTChecks(t *testing.T) {
	root := t.TempDir()
	body := validPolicy
	body = strings.Replace(body, "check_exported_type_alias = true", "check_exported_type_alias = false", 1)
	body = strings.Replace(body, "check_function_var_alias = true", "check_function_var_alias = false", 1)
	body = strings.Replace(body, "check_trivial_wrappers = true", "check_trivial_wrappers = false", 1)
	policyPath := writePolicy(t, root, body)
	writeFile(t, filepath.Join(root, "scripts", "check", "a.sh"), "ok\n")
	writeFile(t, filepath.Join(root, "internal", "alias", "alias.go"), `package alias
import "fmt"
type Config = fmt.Stringer
var Println func(a ...any) (n int, err error) = fmt.Println
func Print(a ...any) (n int, err error) { return fmt.Print(a...) }
`)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"-root", root, "-policy", policyPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q stdout=%s", code, stderr.String(), stdout.String())
	}
}

func TestMissingGoRootsAreSkipped(t *testing.T) {
	root := t.TempDir()
	policyPath := writePolicy(t, root, validPolicy)
	// No managed Go roots is a valid empty fixture.
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"-root", root, "-policy", policyPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q stdout=%s", code, stderr.String(), stdout.String())
	}
}

func TestGoRootFileIgnored(t *testing.T) {
	root := t.TempDir()
	policyPath := writePolicy(t, root, validPolicy)
	writeFile(t, filepath.Join(root, "internal"), "not-a-dir")
	writeFile(t, filepath.Join(root, "scripts", "check", "a.sh"), "ok\n")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"-root", root, "-policy", policyPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q stdout=%s", code, stderr.String(), stdout.String())
	}
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [16]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
