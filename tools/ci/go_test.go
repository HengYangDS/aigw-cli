package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"aigw-cli/internal/process"
)

func TestGoChecksUseCurrentRepositorySources(t *testing.T) {
	root := newGitRepository(t)
	for path, content := range map[string]string{
		".gitignore":          "build/\n",
		"source.go":           "package fixture\n",
		"pending.go":          "package fixture\n",
		"nested/source.go":    "package nested\n",
		"_snapshot/source.go": "captured bytes, not a Go package\n",
		".snapshot/source.go": "captured bytes, not a Go package\n",
		"retired.go":          "package fixture\n",
		"build/generated.go":  "not source\n",
	} {
		target := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	process := exec.Command(
		"git", "-C", root, "add", "--",
		".gitignore", "source.go", "retired.go", "_snapshot/source.go", ".snapshot/source.go",
	)
	if output, err := process.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, output)
	}
	if err := os.Remove(filepath.Join(root, "retired.go")); err != nil {
		t.Fatal(err)
	}
	var got []command
	if err := run([]string{"check-go", root}, &bytes.Buffer{}, func(call command) error {
		got = append(got, call)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	want := []command{{Name: "golangci-lint", Dir: root, Args: []string{
		"fmt", "--diff", "--config", filepath.Join(root, ".config", "checks", "go", "policy.yml"),
	}}, {Name: "golangci-lint", Dir: root, Args: []string{
		"run", "--config", filepath.Join(root, ".config", "checks", "go", "policy.yml"), "--", root, filepath.Join(root, "nested"),
	}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Go commands = %#v, want %#v", got, want)
	}
	for _, phase := range []int{1, 2} {
		wantErr := errors.New("native check failed")
		var calls int
		err := run([]string{"check-go", root}, &bytes.Buffer{}, func(command) error {
			calls++
			if calls == phase {
				return wantErr
			}
			return nil
		})
		if !errors.Is(err, wantErr) || calls != phase {
			t.Fatalf("phase %d failure = %v after %d calls", phase, err, calls)
		}
	}
}

func TestGoChecksIgnoreGeneratedToolchainSources(t *testing.T) {
	root := newGitRepository(t)
	policy, err := os.ReadFile(filepath.Join(repositoryRoot(t), ".config", "checks", "go", "policy.yml"))
	if err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string][]byte{
		"go.mod":                       []byte("module fixture\n"),
		".gitignore":                   []byte("build/\n"),
		"source.go":                    []byte("// Package fixture owns the authored source.\npackage fixture\n"),
		"build/tmp/generated.go":       []byte("package    generated\n"),
		".config/checks/go/policy.yml": policy,
	} {
		target := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if output, err := exec.Command("git", "-C", root, "add", "--", ".gitignore", "go.mod", "source.go").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, output)
	}
	var output []byte
	err = run([]string{"check-go", root}, &bytes.Buffer{}, func(call command) error {
		var commandErr error
		output, commandErr = systemOutputRunner(call)
		return commandErr
	})
	if err != nil {
		t.Fatalf("ignored generated Go source entered the quality gate: %v\n%s", err, output)
	}
	authored := filepath.Join(root, "internal", "build", "source.go")
	if err := os.MkdirAll(filepath.Dir(authored), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(authored, []byte("// Package build owns authored code.\npackage    build\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	output = nil
	err = run([]string{"check-go", root}, &bytes.Buffer{}, func(call command) error {
		var commandErr error
		output, commandErr = systemOutputRunner(call)
		return commandErr
	})
	if err == nil || !bytes.Contains(output, []byte("internal/build/source.go")) {
		t.Fatalf("authored nested build package escaped formatting: %v\n%s", err, output)
	}
}

func TestGoChecksExecuteInRequestedRepository(t *testing.T) {
	policy, err := os.ReadFile(filepath.Join(repositoryRoot(t), ".config", "checks", "go", "policy.yml"))
	if err != nil {
		t.Fatal(err)
	}
	root := newGitRepository(t)
	caller := filepath.Dir(root)
	for path, content := range map[string][]byte{
		"go.mod":                       []byte("module fixture\n"),
		".config/checks/go/policy.yml": policy,
	} {
		target := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(caller)
	nativeInvocations := 0
	for _, test := range []struct {
		name, root, file, source, diagnostic string
		valid                                bool
	}{
		{"absolute root", root, "doc.go", "// Package fixture owns a test module.\npackage fixture\n", "", true},
		{"relative root", filepath.Base(root), "doc.go", "// Package fixture owns a test module.\npackage fixture\n", "", true},
		{"format drift", root, "doc.go", "// Package fixture owns a test module.\npackage    fixture\n", "", false},
		{"type error", root, "doc.go", "// Package fixture owns a test module.\npackage fixture\n\nvar value int = \"invalid\"\n", "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(root, test.file)
			if err := os.WriteFile(path, []byte(test.source), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.Remove(path); err != nil {
					t.Error(err)
				}
			})
			var output []byte
			err := run([]string{"check-go", test.root}, &bytes.Buffer{}, func(call command) error {
				nativeInvocations++
				var err error
				output, err = systemOutputRunner(call)
				return err
			})
			if (err == nil) != test.valid {
				t.Fatalf("valid=%t error=%v\n%s", test.valid, err, output)
			}
			for diagnostic := range strings.FieldsSeq(test.diagnostic) {
				if !bytes.Contains(output, []byte(diagnostic)) {
					t.Fatalf("expected %s diagnostic: %v\n%s", diagnostic, err, output)
				}
			}
			if source, err := os.ReadFile(path); err != nil || string(source) != test.source {
				t.Fatalf("check changed source: %q error=%v", source, err)
			}
		})
	}
	if nativeInvocations > 8 {
		t.Fatalf("root/format/type conformance used %d native invocations; batch independent rule fixtures", nativeInvocations)
	}
}

func TestGoRulesExecuteInOneNativeBatch(t *testing.T) {
	root := t.TempDir()
	policy := readFile(t, filepath.Join(repositoryRoot(t), ".config", "checks", "go", "policy.yml"))
	policyPath := filepath.Join(root, "policy.yml")
	if err := os.WriteFile(policyPath, policy, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, file, source, diagnostic string
		valid                          bool
	}{
		{"checked type assertion", "doc.go", goCheckSource(t, "checked assertion", 0), "", true},
		{"unchecked type assertion", "doc.go", goCheckSource(t, "unchecked assertion", 0), "errcheck", false},
		{"unchecked test assertion", "doc_test.go", goCheckSource(t, "unchecked assertion", 0), "errcheck", false},
		{"bounded nesting", "doc.go", goCheckSource(t, "bounded nesting", 0), "", true},
		{"nested product branches", "doc.go", goCheckSource(t, "nested branches", 0), "nestif", false},
		{"nested test branches", "doc_test.go", goCheckSource(t, "nested branches", 0), "nestif", false},
		{"native product branches", "doc_" + runtime.GOOS + ".go", goCheckSource(t, "nested branches", 0), "nestif", false},
		{"native test branches", "doc_" + runtime.GOOS + "_test.go", goCheckSource(t, "nested branches", 0), "nestif", false},
		{"guarded pointer", "doc.go", goCheckSource(t, "guarded pointer", 0), "", true},
		{"nil dereference", "doc.go", goCheckSource(t, "nil dereference", 0), "govet nilness", false},
		{"nil dereference test", "doc_test.go", goCheckSource(t, "nil dereference", 0), "govet nilness", false},
		{"persistent field write", "doc.go", goCheckSource(t, "persistent field write", 0), "", true},
		{"unused field write", "doc.go", goCheckSource(t, "unused field write", 0), "govet unusedwrite", false},
		{"unused field write test", "doc_test.go", goCheckSource(t, "unused field write", 0), "govet unusedwrite", false},
		{"bounded parameters", "doc.go", goCheckSource(t, "parameters", 7), "", true},
		{"excess parameters", "doc.go", goCheckSource(t, "parameters", 8), "revive", false},
		{"test parameters", "doc_test.go", goCheckSource(t, "parameters", 8), "revive", false},
		{"bounded statements", "doc.go", goCheckSource(t, "statements", 60), "", true},
		{"excess statements", "doc.go", goCheckSource(t, "statements", 61), "funlen", false},
		{"test statements", "doc_test.go", goCheckSource(t, "statements", 61), "funlen", false},
		{"bounded lines", "doc.go", goCheckSource(t, "lines", 120), "", true},
		{"excess lines", "doc.go", goCheckSource(t, "lines", 121), "funlen", false},
		{"test lines", "doc_test.go", goCheckSource(t, "lines", 121), "funlen", false},
		{"bounded branches", "doc.go", goCheckSource(t, "branches", 25), "", true},
		{"excess branches", "doc.go", goCheckSource(t, "branches", 26), "cyclop", false},
		{"test branches", "doc_test.go", goCheckSource(t, "branches", 26), "cyclop", false},
		{"bounded cognition", "doc.go", goCheckSource(t, "cognition", 45), "", true},
		{"excess cognition", "doc.go", goCheckSource(t, "cognition", 46), "gocognit", false},
		{"test cognition", "doc_test.go", goCheckSource(t, "cognition", 46), "gocognit", false},
		{"companion diagnostics", "doc.go", goCheckSource(t, "cognition", 66), "cyclop gocognit", false},
		{"low product maintainability", "doc.go", goCheckSource(t, "branches", 61), "maintidx", false},
		{"low test maintainability", "doc_test.go", goCheckSource(t, "branches", 61), "maintidx", false},
		{"small repeated expressions", "doc.go", goCheckSource(t, "duplicates", 2), "", true},
		{"duplicated product operation", "doc.go", goCheckSource(t, "duplicates", 40), "dupl", false},
		{"duplicated test operation", "doc_test.go", goCheckSource(t, "duplicates", 40), "dupl", false},
	}
	for _, test := range cases {
		directory := filepath.Join(root, strings.ReplaceAll(test.name, " ", "_"))
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, test.file), []byte(test.source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	reportPath := filepath.Join(root, "issues.json")
	output, diagnostic, runErr := (process.Runner{}).RunCaptureStreams(ctx, process.Plan{
		Executable: "golangci-lint", Directory: root,
		Args: []string{"run", "--config", policyPath, "--path-mode", "abs", "--output.json.path", reportPath,
			"--max-issues-per-linter", "0", "--max-same-issues", "0", "--uniq-by-line=false", "--", "./..."},
	})
	var exitError *exec.ExitError
	if !errors.As(runErr, &exitError) || exitError.ExitCode() != 1 || ctx.Err() != nil || process.DiagnosticFailure(diagnostic) {
		t.Fatalf("native conformance batch must report rule findings: %v\n%s\n%s", runErr, output, diagnostic)
	}
	var report struct {
		Issues []struct {
			FromLinter string `json:"FromLinter"`
			Text       string `json:"Text"`
			Pos        struct {
				Filename string `json:"Filename"`
			} `json:"Pos"`
		} `json:"Issues"`
		Report struct {
			Error    string   `json:"Error"`
			Warnings []string `json:"Warnings"`
		} `json:"Report"`
	}
	if err := json.Unmarshal(readFile(t, reportPath), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Issues) == 0 {
		t.Fatal("native batch reported no isolated negative findings")
	}
	if report.Report.Error != "" || len(report.Report.Warnings) != 0 {
		t.Fatalf("native conformance report is unqualified: error=%q warnings=%v", report.Report.Error, report.Report.Warnings)
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			directory := filepath.Join(root, strings.ReplaceAll(test.name, " ", "_"))
			var diagnostics []string
			for _, issue := range report.Issues {
				if filepath.Dir(issue.Pos.Filename) == directory {
					diagnostics = append(diagnostics, issue.FromLinter+" "+issue.Text)
				}
			}
			if (len(diagnostics) == 0) != test.valid {
				t.Fatalf("valid=%t diagnostics=%v", test.valid, diagnostics)
			}
			for expected := range strings.FieldsSeq(test.diagnostic) {
				if !strings.Contains(strings.Join(diagnostics, "\n"), expected) {
					t.Fatalf("native batch omitted expected %s diagnostic: %v", expected, diagnostics)
				}
			}
			if actual := string(readFile(t, filepath.Join(directory, test.file))); actual != test.source {
				t.Fatal("native batch mutated its input")
			}
		})
	}
}

func goCheckSource(t *testing.T, metric string, value int) string {
	t.Helper()
	var source strings.Builder
	source.WriteString("// Package fixture owns a test module.\npackage fixture\n\n// Measure exercises one native quality boundary.\n")
	switch metric {
	case "checked assertion":
		source.WriteString("func Measure(value interface{}) string { text, ok := value.(string); if !ok { return \"\" }; return text }\n")
	case "unchecked assertion":
		source.WriteString("func Measure(value interface{}) string { return value.(string) }\n")
	case "bounded nesting":
		source.WriteString(`func Measure(a, b, c, d bool) bool {
	if a {
		if b {
			if c {
				return true
			}
		}
	}
	return d
}
`)
	case "nested branches":
		source.WriteString(`func Measure(a, b, c, d bool) bool {
	if a {
		if b {
			if c {
				return true
			}
		}
		if d {
			return true
		}
	}
	return false
}
`)
	case "guarded pointer":
		source.WriteString("func Measure(value *int) int { if value != nil { return *value }; return 0 }\n")
	case "nil dereference":
		source.WriteString("func Measure(value *int) int { if value == nil { return *value }; return 0 }\n")
	case "persistent field write":
		source.WriteString("func Measure(values []struct{ Value int }) { for index := range values { values[index].Value = 1 } }\n")
	case "unused field write":
		source.WriteString("func Measure(values []struct{ Value int }) { for _, value := range values { value.Value = 1 } }\n")
	case "parameters":
		names := strings.Split("a b c d e f g h", " ")[:value]
		fmt.Fprintf(&source, "func Measure(%s int) int { return %s }\n", strings.Join(names, ", "), strings.Join(names, " + "))
	case "statements":
		fmt.Fprintf(&source, "func Measure(value int) int {\n%sreturn value\n}\n", strings.Repeat("value++\n", value-1))
	case "lines":
		fmt.Fprintf(&source, "func Measure() string {\nreturn `%s`\n}\n", strings.Repeat("line\n", value-1))
	case "branches":
		source.WriteString("func Measure(value int) int {\nswitch value {\n")
		for index := range value - 1 {
			fmt.Fprintf(&source, "case %d: return %d\n", index, index+1)
		}
		source.WriteString("}\nreturn value\n}\n")
	case "cognition":
		source.WriteString("func Measure(values []int) int {\nvalue := 0\nif len(values) == 0 { return value }\n")
		for index := range (value - 4) % 3 {
			fmt.Fprintf(&source, "if len(values) == %d { return values[0] }\n", index+1)
		}
		source.WriteString("for _, item := range values {\nfor _, other := range values {\n")
		for index := range (value - 4) / 3 {
			fmt.Fprintf(&source, "if item > other + %d { value++ }\n", index)
		}
		source.WriteString("}\n}\nreturn value\n}\n")
	case "duplicates":
		for _, name := range []string{"Measure", "Repeated"} {
			fmt.Fprintf(&source, "// %s transforms the input.\nfunc %s(value int) int {\n", name, name)
			for index := range value {
				fmt.Fprintf(&source, "value += %d\n", index+1)
			}
			source.WriteString("return value\n}\n")
		}
	default:
		t.Fatalf("unknown metric %q", metric)
	}
	formatted, err := format.Source([]byte(source.String()))
	if err != nil {
		t.Fatal(err)
	}
	return string(formatted)
}
