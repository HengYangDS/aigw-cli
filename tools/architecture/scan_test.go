package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestIgnoreHelpers(t *testing.T) {
	p := mustPolicy(t)
	if shouldIgnoreDirName("vendor", p) != true {
		t.Fatal("vendor")
	}
	if shouldIgnoreDirName(".", p) {
		t.Fatal("dot")
	}
	if shouldIgnoreDirName("..", p) {
		t.Fatal("dotdot")
	}
	if shouldIgnoreDirName(".hidden", p) {
		t.Fatal("hidden not ignored unless listed")
	}
	if !shouldIgnoreRelPath("vendor/pkg/a.go", p) {
		t.Fatal("ignore root")
	}
	if shouldIgnoreRelPath("internal/runtime/x.go", p) {
		t.Fatal("runtime must not receive a historical exemption")
	}
	if shouldIgnoreRelPath("records/internal/x.go", p) {
		t.Fatal("records must not receive a historical exemption")
	}
	if shouldIgnoreRelPath("internal/pkg/a.go", p) {
		t.Fatal("normal path")
	}
	if shouldIgnoreRelPath("", p) {
		t.Fatal("empty")
	}
}

func TestIgnorePolicyScopeAndAllocation(t *testing.T) {
	p := policy{IgnoreRoots: []string{"generated"}, IgnoreDirectoryNames: []string{"vendor"}}
	for _, test := range []struct {
		path string
		want bool
	}{
		{"generated/a.go", true},
		{"internal/generated/a.go", false},
		{"internal/vendor/a.go", true},
		{"vendor/a.go", true},
		{"internal/vendored/a.go", false},
		{"internal/a.go", false},
		{"", false},
	} {
		t.Run(test.path, func(t *testing.T) {
			var got bool
			allocations := testing.AllocsPerRun(100, func() {
				got = shouldIgnoreRelPath(test.path, p)
			})
			if got != test.want {
				t.Fatalf("ignored = %t, want %t", got, test.want)
			}
			if allocations != 0 {
				t.Fatalf("policy lookup allocated %.0f objects per path", allocations)
			}
		})
	}
}

func TestRelativePolicyFromRoot(t *testing.T) {
	root := t.TempDir()
	writeRepository(t, root, validPolicy)
	writeFile(t, filepath.Join(root, "scripts", "check", "a.sh"), "ok\n")
	writeFile(t, filepath.Join(root, "internal", "pkg", "core.go"), "package pkg\n")
	trackFixture(t, root)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"-root", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q stdout=%s", code, stderr.String(), stdout.String())
	}
	report := decodeReport(t, stdout.String())
	if report.Policy != policyPath {
		t.Fatalf("policy=%q", report.Policy)
	}
}

func TestCollectIgnoresNonGoFiles(t *testing.T) {
	root := t.TempDir()
	writeRepository(t, root, validPolicy)
	writeFile(t, filepath.Join(root, "scripts", "check", "a.sh"), "ok\n")
	writeFile(t, filepath.Join(root, "internal", "pkg", "note.txt"), "x")
	writeFile(t, filepath.Join(root, "internal", "pkg", "core.go"), "package pkg\n")
	trackFixture(t, root)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"-root", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stdout=%s", code, stdout.String())
	}
	report := decodeReport(t, stdout.String())
	if !report.OK {
		t.Fatalf("ignored paths produced findings: %+v", report.Findings)
	}
}

func mustPolicy(t *testing.T) policy {
	t.Helper()
	path := writeRepository(t, t.TempDir(), validPolicy)
	p, err := loadPolicy(path)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAnalyzeRepositoryDirect(t *testing.T) {
	root := t.TempDir()
	writeRepository(t, root, validPolicy)
	writeFile(t, filepath.Join(root, "scripts", "check", "a.sh"), "ok\n")
	writeFile(t, filepath.Join(root, "internal", "pkg", "core.go"), "package pkg\n")
	trackFixture(t, root)
	report, err := analyzeRepository(root, mustPolicy(t))
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("%+v", report.Findings)
	}
}

func TestAnalyzeRepositoryPropagatesSemanticStageFailures(t *testing.T) {
	for name, test := range map[string]struct {
		fixture func(t *testing.T, root string)
		policy  policy
		want    string
	}{
		"decision records": {
			fixture: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, "docs", "decisions", decisionNavigationFile, "entry.md"), "")
			},
			want: "read Decision Record navigation",
		},
		"package children": {policy: policy{PackageChildren: map[string][]string{"tools": {"release"}}}, want: "stat package root tools"},
		"Go files":         {policy: policy{GoRoots: []string{"internal"}}, want: "stat go root internal"},
		"layers": {
			fixture: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, "internal", "pkg", "core.go"), "package pkg\n")
			},
			policy: policy{GoRoots: []string{"internal"}, Layers: [][]string{{"internal"}}},
			want:   "read go.mod",
		},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if test.fixture != nil {
				test.fixture(t, root)
			}
			trackFixture(t, root)
			if _, err := analyzeRepository(root, test.policy); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestAnalyzeRepositoryRejectsBrokenRepositoryMetadata(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := analyzeRepository(root, policy{}); err == nil || !strings.Contains(err.Error(), "list tracked files") {
		t.Fatalf("error = %v", err)
	}
}

func TestCollectGoFilesRejectsInvalidRoot(t *testing.T) {
	if _, err := collectGoFiles(t.TempDir(), policy{GoRoots: []string{"invalid\x00root"}}); err == nil {
		t.Fatal("invalid Go root was accepted")
	}
}

func TestCollectGoFilesPropagatesWalkFailures(t *testing.T) {
	if runtime.GOOS == "windows" {
		return
	}
	if os.Geteuid() == 0 {
		t.Skip("root lists a directory without read permission")
	}
	root := t.TempDir()
	closed := filepath.Join(root, "internal", "closed")
	writeFile(t, filepath.Join(closed, "owner.go"), "package closed\n")
	if err := os.Chmod(closed, 0o300); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(closed, 0o700) })
	if _, err := collectGoFiles(root, policy{GoRoots: []string{"internal"}}); err == nil || !strings.Contains(err.Error(), "walk go root internal") {
		t.Fatalf("walk error = %v", err)
	}
}

func TestCollectGoFilesIgnoresConfiguredFilePath(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "internal", "ignored.go"), "package internal\n")
	files, err := collectGoFiles(root, policy{GoRoots: []string{"internal"}, IgnoreRoots: []string{"internal"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("ignored path collected: files=%+v", files)
	}
}

func TestRunReportsAnalysisFailure(t *testing.T) {
	root := t.TempDir()
	writeRepository(t, root, validPolicy+"layers = [[\"internal\"]]\n")
	writeFile(t, filepath.Join(root, "internal", "pkg", "core.go"), "package pkg\n")
	trackFixture(t, root)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-root", root}, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "analyze repository: read go.mod") || stdout.Len() != 0 {
		t.Fatalf("code=%d stderr=%q stdout=%s", code, stderr.String(), stdout.String())
	}
}
