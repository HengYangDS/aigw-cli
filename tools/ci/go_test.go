package main

import (
	"bytes"
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
)

func TestGoChecksUseCurrentRepositorySources(t *testing.T) {
	root := t.TempDir()
	process := exec.Command("git", "-C", root, "init", "--quiet")
	if output, err := process.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	for path, content := range map[string]string{
		".gitignore":         "build/\n",
		"source.go":          "package fixture\n",
		"pending.go":         "package fixture\n",
		"nested/source.go":   "package nested\n",
		"retired.go":         "package fixture\n",
		"build/generated.go": "not source\n",
	} {
		target := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	process = exec.Command("git", "-C", root, "add", "--", ".gitignore", "source.go", "retired.go")
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
		"fmt", "--diff", "--config", filepath.Join(root, ".config", "checks", "go", "policy.yml"), "--",
		filepath.Join(root, "nested", "source.go"), filepath.Join(root, "pending.go"), filepath.Join(root, "source.go"),
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

func TestGoChecksExecuteInRequestedRepository(t *testing.T) {
	policy, err := os.ReadFile(filepath.Join(repositoryRoot(t), ".config", "checks", "go", "policy.yml"))
	if err != nil {
		t.Fatal(err)
	}
	caller := t.TempDir()
	root := filepath.Join(caller, "checkout with spaces")
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
	if output, err := exec.Command("git", "-C", root, "init", "--quiet").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
	}
	t.Chdir(caller)
	for _, test := range []struct {
		name, root, file, source, diagnostic string
		valid                                bool
	}{
		{"absolute root", root, "doc.go", "// Package fixture owns a test module.\npackage fixture\n", "", true},
		{"relative root", filepath.Base(root), "doc.go", "// Package fixture owns a test module.\npackage fixture\n", "", true},
		{"format drift", root, "doc.go", "// Package fixture owns a test module.\npackage    fixture\n", "", false},
		{"type error", root, "doc.go", "// Package fixture owns a test module.\npackage fixture\n\nvar value int = \"invalid\"\n", "", false},
		{"checked type assertion", root, "doc.go", goCheckSource(t, "checked assertion", 0), "", true},
		{"unchecked type assertion", root, "doc.go", goCheckSource(t, "unchecked assertion", 0), "errcheck", false},
		{"unchecked test assertion", root, "doc_test.go", goCheckSource(t, "unchecked assertion", 0), "errcheck", false},
		{"bounded nesting", root, "doc.go", goCheckSource(t, "bounded nesting", 0), "", true},
		{"nested product branches", root, "doc.go", goCheckSource(t, "nested branches", 0), "nestif", false},
		{"nested test branches", root, "doc_test.go", goCheckSource(t, "nested branches", 0), "nestif", false},
		{"native product branches", root, "doc_" + runtime.GOOS + ".go", goCheckSource(t, "nested branches", 0), "nestif", false},
		{"native test branches", root, "doc_" + runtime.GOOS + "_test.go", goCheckSource(t, "nested branches", 0), "nestif", false},
		{"guarded pointer", root, "doc.go", goCheckSource(t, "guarded pointer", 0), "", true},
		{"nil dereference", root, "doc.go", goCheckSource(t, "nil dereference", 0), "govet nilness", false},
		{"nil dereference test", root, "doc_test.go", goCheckSource(t, "nil dereference", 0), "govet nilness", false},
		{"persistent field write", root, "doc.go", goCheckSource(t, "persistent field write", 0), "", true},
		{"unused field write", root, "doc.go", goCheckSource(t, "unused field write", 0), "govet unusedwrite", false},
		{"unused field write test", root, "doc_test.go", goCheckSource(t, "unused field write", 0), "govet unusedwrite", false},
		{"bounded parameters", root, "doc.go", goCheckSource(t, "parameters", 7), "", true},
		{"excess parameters", root, "doc.go", goCheckSource(t, "parameters", 8), "revive", false},
		{"test parameters", root, "doc_test.go", goCheckSource(t, "parameters", 8), "revive", false},
		{"bounded statements", root, "doc.go", goCheckSource(t, "statements", 60), "", true},
		{"excess statements", root, "doc.go", goCheckSource(t, "statements", 61), "funlen", false},
		{"test statements", root, "doc_test.go", goCheckSource(t, "statements", 61), "funlen", false},
		{"bounded lines", root, "doc.go", goCheckSource(t, "lines", 120), "", true},
		{"excess lines", root, "doc.go", goCheckSource(t, "lines", 121), "funlen", false},
		{"test lines", root, "doc_test.go", goCheckSource(t, "lines", 121), "funlen", false},
		{"bounded branches", root, "doc.go", goCheckSource(t, "branches", 25), "", true},
		{"excess branches", root, "doc.go", goCheckSource(t, "branches", 26), "cyclop", false},
		{"test branches", root, "doc_test.go", goCheckSource(t, "branches", 26), "cyclop", false},
		{"bounded cognition", root, "doc.go", goCheckSource(t, "cognition", 45), "", true},
		{"excess cognition", root, "doc.go", goCheckSource(t, "cognition", 46), "gocognit", false},
		{"test cognition", root, "doc_test.go", goCheckSource(t, "cognition", 46), "gocognit", false},
		{"companion diagnostics", root, "doc.go", goCheckSource(t, "cognition", 66), "cyclop gocognit", false},
		{"low product maintainability", root, "doc.go", goCheckSource(t, "branches", 61), "maintidx", false},
		{"low test maintainability", root, "doc_test.go", goCheckSource(t, "branches", 61), "maintidx", false},
		{"small repeated expressions", root, "doc.go", goCheckSource(t, "duplicates", 2), "", true},
		{"duplicated product operation", root, "doc.go", goCheckSource(t, "duplicates", 40), "dupl", false},
		{"duplicated test operation", root, "doc_test.go", goCheckSource(t, "duplicates", 40), "dupl", false},
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
		if d {
			return true
		}
	}
	return false
}
`)
	case "nested branches":
		source.WriteString(`func Measure(a, b, c, d bool) bool {
	if a {
		if b {
			if c {
				return true
			}
			if d {
				return true
			}
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
