package main

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
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

func TestRelativePolicyFromRoot(t *testing.T) {
	root := t.TempDir()
	cfgDir := filepath.Join(root, ".config", "checks", "architecture")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "policy.toml"), []byte(validPolicy), 0o600); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "scripts", "check", "a.sh"), "ok\n")
	writeFile(t, filepath.Join(root, "internal", "pkg", "core.go"), "package pkg\n")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"-root", root, "-policy", defaultPolicyPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q stdout=%s", code, stderr.String(), stdout.String())
	}
	report := decodeReport(t, stdout.String())
	if report.Policy != defaultPolicyPath {
		t.Fatalf("policy=%q", report.Policy)
	}
}

func TestCollectIgnoresNonGoFiles(t *testing.T) {
	root := t.TempDir()
	policyPath := writePolicy(t, root, validPolicy)
	writeFile(t, filepath.Join(root, "scripts", "check", "a.sh"), "ok\n")
	writeFile(t, filepath.Join(root, "internal", "pkg", "note.txt"), "x")
	writeFile(t, filepath.Join(root, "internal", "pkg", "core.go"), "package pkg\n")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"-root", root, "-policy", policyPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stdout=%s", code, stdout.String())
	}
	report := decodeReport(t, stdout.String())
	if !report.OK {
		t.Fatalf("ignored paths produced findings: %+v", report.Findings)
	}
}

func TestCheckGoASTReadError(t *testing.T) {
	report := newReport("p", ".")
	err := checkGoAST(t.TempDir(), []goFileInfo{{relPath: "missing.go", name: "missing.go", dir: ".", isTest: false}}, mustPolicy(t), &report)
	if err == nil {
		t.Fatal("expected read error")
	}
}

func TestCheckGoASTSkipsTestFiles(t *testing.T) {
	report := newReport("p", ".")
	err := checkGoAST(
		t.TempDir(),
		[]goFileInfo{{relPath: "missing_test.go", name: "missing_test.go", dir: ".", isTest: true}},
		mustPolicy(t),
		&report,
	)
	if err != nil || !report.OK {
		t.Fatalf("test-only source entered the production AST plane: report=%+v err=%v", report, err)
	}
}

func TestAbsolutePolicyOutsideRoot(t *testing.T) {
	root := t.TempDir()
	other := t.TempDir()
	policyPath := writePolicy(t, other, validPolicy)
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
		t.Fatal("empty policy")
	}
}

func mustPolicy(t *testing.T) policy {
	t.Helper()
	path := writePolicy(t, t.TempDir(), validPolicy)
	p, err := loadPolicy(path)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAnalyzeRepositoryDirect(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "scripts", "check", "a.sh"), "ok\n")
	writeFile(t, filepath.Join(root, "internal", "pkg", "core.go"), "package pkg\n")
	report, err := analyzeRepository(root, mustPolicy(t), "policy.toml")
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("%+v", report.Findings)
	}
}

func TestSemanticStageRejectsBrokenRepositoryMetadata(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	report := newReport("policy", root)
	if err := checkSemanticNames(root, &report); err == nil || !strings.Contains(err.Error(), "list tracked files") {
		t.Fatalf("error = %v", err)
	}
}

func TestAnalyzeRepositoryPropagatesSemanticStageFailures(t *testing.T) {
	original := repositoryAnalysis
	t.Cleanup(func() { repositoryAnalysis = original })
	want := errors.New("stage failed")
	for name, configure := range map[string]func(){
		"decision records": func() {
			repositoryAnalysis.decisionRecords = func(string, *Report) error { return want }
		},
		"semantic names": func() {
			repositoryAnalysis.semanticNames = func(string, *Report) error { return want }
		},
		"package children": func() {
			repositoryAnalysis.packageChildren = func(string, policy, *Report) error { return want }
		},
		"Go files": func() {
			repositoryAnalysis.goFiles = func(string, policy) ([]goFileInfo, error) { return nil, want }
		},
		"peer imports": func() {
			repositoryAnalysis.peerImports = func(string, []goFileInfo, policy, *Report) error { return want }
		},
		"import edges": func() {
			repositoryAnalysis.importEdges = func(string, []goFileInfo, policy, *Report) error { return want }
		},
		"Go AST": func() {
			repositoryAnalysis.goAST = func(string, []goFileInfo, policy, *Report) error { return want }
		},
	} {
		t.Run(name, func(t *testing.T) {
			repositoryAnalysis = original
			configure()
			p := policy{
				CheckDecisionRecords: name == "decision records",
				CheckSemanticNames:   name == "semantic names",
			}
			if _, err := analyzeRepository(t.TempDir(), p, "policy.toml"); !errors.Is(err, want) {
				t.Fatalf("error = %v, want %v", err, want)
			}
		})
	}
}

func TestAnalyzeRepositoryRejectsBrokenRepositoryMetadata(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := analyzeRepository(root, policy{}, "policy.toml"); err == nil || !strings.Contains(err.Error(), "list tracked files") {
		t.Fatalf("error = %v", err)
	}
}

func TestCollectGoFilesRejectsInvalidRoot(t *testing.T) {
	if _, err := collectGoFiles(t.TempDir(), policy{GoRoots: []string{"invalid\x00root"}}); err == nil {
		t.Fatal("invalid Go root was accepted")
	}
}

func TestCollectGoFilesPropagatesWalkAndRelativeFailures(t *testing.T) {
	root := t.TempDir()
	goRoot := filepath.Join(root, "internal")
	writeFile(t, filepath.Join(goRoot, "owner.go"), "package internal\n")
	walkFailure := errors.New("walk failed")
	if _, err := collectGoFilesWith(root, policy{GoRoots: []string{"internal"}}, func(string, fs.WalkDirFunc) error {
		return walkFailure
	}, filepath.Rel); !errors.Is(err, walkFailure) {
		t.Fatalf("walk error = %v", err)
	}
	relativeFailure := errors.New("relative path failed")
	if _, err := collectGoFilesWith(root, policy{GoRoots: []string{"internal"}}, filepath.WalkDir, func(string, string) (string, error) {
		return "", relativeFailure
	}); !errors.Is(err, relativeFailure) {
		t.Fatalf("relative error = %v", err)
	}
	if _, err := collectGoFilesWith(root, policy{GoRoots: []string{"internal"}}, func(path string, visit fs.WalkDirFunc) error {
		entry, statErr := os.Stat(path)
		if statErr != nil {
			return statErr
		}
		return visit(path, fileInfoDirEntry{entry}, walkFailure)
	}, filepath.Rel); !errors.Is(err, walkFailure) {
		t.Fatalf("visit error = %v", err)
	}
}

type fileInfoDirEntry struct{ os.FileInfo }

func (entry fileInfoDirEntry) Type() os.FileMode { return entry.Mode().Type() }

func (entry fileInfoDirEntry) Info() (os.FileInfo, error) { return entry.FileInfo, nil }

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

func TestMainCoversEntry(t *testing.T) {
	originalArgs := os.Args
	originalExit := exitFunc
	defer func() {
		os.Args = originalArgs
		exitFunc = originalExit
	}()
	var code int
	exitFunc = func(c int) { code = c }
	root := t.TempDir()
	policyPath := writePolicy(t, root, validPolicy)
	writeFile(t, filepath.Join(root, "scripts", "check", "a.sh"), "ok\n")
	writeFile(t, filepath.Join(root, "internal", "pkg", "core.go"), "package pkg\n")
	os.Args = []string{"architecture", "-root", root, "-policy", policyPath}
	main()
	if code != 0 {
		t.Fatalf("main exit=%d", code)
	}
}

func TestRunReportsAnalysisFailure(t *testing.T) {
	root := t.TempDir()
	policyPath := writePolicy(t, root, validPolicy)
	if err := os.MkdirAll(filepath.Join(root, "internal"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "missing-go-source"), filepath.Join(root, "internal", "broken.go")); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-root", root, "-policy", policyPath}, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "analyze repository") {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
}

func TestShouldIgnoreRelPathEmptyParts(t *testing.T) {
	p := mustPolicy(t)
	// strings.Split("", "/") yields []string{""}; first part "" not in ignore roots
	if shouldIgnoreRelPath("", p) {
		t.Fatal("empty")
	}
}
