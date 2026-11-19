package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func packageStream(t *testing.T, packages ...sourcePackage) string {
	t.Helper()
	var stream bytes.Buffer
	encoder := json.NewEncoder(&stream)
	for _, pkg := range packages {
		if err := encoder.Encode(pkg); err != nil {
			t.Fatal(err)
		}
	}
	return stream.String()
}

func TestListSourcesClassifiesEverySelectedProductionFile(t *testing.T) {
	root := t.TempDir()
	runner := &recordingRunner{metadata: packageStream(t,
		sourcePackage{ImportPath: "example", Dir: root, GoFiles: []string{"main.go"}},
		sourcePackage{
			ImportPath: "example/a", Dir: filepath.Join(root, "a"),
			GoFiles: []string{"a.go"}, CgoFiles: []string{"c.go"},
			IgnoredGoFiles: []string{"a_windows.go", "a_windows_test.go"},
		},
		sourcePackage{ImportPath: "example/onlytests", Dir: filepath.Join(root, "onlytests")},
	)}
	inventory, err := listSources(runner, root, []string{"-tags=native"}, []string{"./..."})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"example/main.go": "main.go", "example/a/a.go": "a/a.go", "example/a/c.go": "a/c.go"}
	if len(inventory.compiled) != len(want) {
		t.Fatalf("compiled = %v, want %v", inventory.compiled, want)
	}
	for name, path := range want {
		if inventory.compiled[name] != path {
			t.Fatalf("compiled = %v, want %v", inventory.compiled, want)
		}
	}
	if !slices.Equal(inventory.excluded, []string{"a/a_windows.go"}) {
		t.Fatalf("excluded = %v, want the non-test platform file only", inventory.excluded)
	}
	if !slices.Equal(runner.calls[0], []string{"list", "-json", "-tags=native", "./..."}) {
		t.Fatalf("inventory did not use the attempt's selection: %q", runner.calls[0])
	}
}

func TestListSourcesRefusesAnIncompleteInventory(t *testing.T) {
	root := t.TempDir()
	for _, test := range []struct {
		name, metadata, want string
		err                  error
	}{
		{"list fails", "", "go list", errors.New("exit status 1")},
		{"not JSON", "{", "parse go list", nil},
		{"outside the repository", packageStream(t, sourcePackage{ImportPath: "other", Dir: filepath.Dir(root), GoFiles: []string{"x.go"}}), "outside the repository", nil},
		{"no import path", packageStream(t, sourcePackage{Dir: root, GoFiles: []string{"x.go"}}), "outside the repository", nil},
		{"no production file", packageStream(t, sourcePackage{ImportPath: "example", Dir: root}), "no production files", nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := &recordingRunner{metadata: test.metadata, metadataErr: test.err}
			if _, err := listSources(runner, root, nil, []string{"./..."}); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func writeProfile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "coverage.out")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLineCountsDeriveLinesFromTheSameStatementBlocks(t *testing.T) {
	inventory := sourceInventory{
		compiled: map[string]string{"example/a/a.go": "a/a.go", "example/a/types.go": "a/types.go", "example/b/b.go": "b/b.go"},
		excluded: []string{"a/a_windows.go"},
	}
	profile := writeProfile(t, "mode: atomic\n"+
		"example/a/a.go:3.10,5.2 2 4\n"+
		"example/a/a.go:5.2,6.3 1 0\n"+
		"example/a/a.go:8.1,8.9 0 7\n"+
		"example/b/b.go:1.1,2.2 1 0\n")
	records, err := lineCounts(profile, inventory)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]map[int]int{
		"a/a.go":         {3: 4, 4: 4, 5: 4, 6: 0},
		"a/types.go":     {},
		"a/a_windows.go": {},
		"b/b.go":         {1: 0, 2: 0},
	}
	if len(records) != len(want) {
		t.Fatalf("records = %v, want %v", records, want)
	}
	for path, lines := range want {
		got, found := records[path]
		if !found || len(got) != len(lines) {
			t.Fatalf("%s lines = %v, want %v", path, got, lines)
		}
		for line, count := range lines {
			if got[line] != count {
				t.Fatalf("%s lines = %v, want %v", path, got, lines)
			}
		}
	}

	var report bytes.Buffer
	if err := writeLCOV(&report, records); err != nil {
		t.Fatal(err)
	}
	wantReport := "SF:a/a.go\nDA:3,4\nDA:4,4\nDA:5,4\nDA:6,0\nLF:4\nLH:3\nend_of_record\n" +
		"SF:a/a_windows.go\nLF:0\nLH:0\nend_of_record\n" +
		"SF:a/types.go\nLF:0\nLH:0\nend_of_record\n" +
		"SF:b/b.go\nDA:1,0\nDA:2,0\nLF:2\nLH:0\nend_of_record\n"
	if report.String() != wantReport {
		t.Fatalf("LCOV report =\n%s\nwant\n%s", &report, wantReport)
	}
	if strings.Contains(report.String(), "BRDA:") || strings.Contains(report.String(), "FN:") {
		t.Fatal("a statement profile must not yield branch or function records")
	}
	if err := writeLCOV(rejectingWriter{}, records); err == nil {
		t.Fatal("an unwritable report must fail")
	}
}

func TestLineCountsRefuseCountersOutsideTheInventory(t *testing.T) {
	inventory := sourceInventory{compiled: map[string]string{"example/a/a.go": "a/a.go"}}
	if _, err := lineCounts(writeProfile(t, "mode: atomic\nexample/other/x.go:1.1,2.2 1 1\n"), inventory); err == nil ||
		!strings.Contains(err.Error(), "outside the selected inventory") {
		t.Fatalf("foreign counter error = %v", err)
	}
	if _, err := lineCounts(filepath.Join(t.TempDir(), "missing.out"), inventory); err == nil {
		t.Fatal("a missing profile must fail")
	}
}

func TestRealMainWritesBothReportsFromTheSingleAttempt(t *testing.T) {
	for _, test := range []struct {
		name    string
		json    bool
		failure error
		code    int
	}{
		{"plain view", false, nil, 0},
		{"native events", true, nil, 0},
		{"failed tests keep diagnostic reports", false, errors.New("tests failed"), 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			usePolicy(t, validPolicy)
			root, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			events := "{\"Action\":\"run\",\"Package\":\"example\",\"Test\":\"TestValue\"}\n" +
				"{\"Action\":\"pass\",\"Package\":\"example\",\"Test\":\"TestValue\",\"Elapsed\":0.1}\n" +
				"{\"Action\":\"output\",\"Package\":\"example\",\"Output\":\"ok  \\texample\\t0.1s\\n\"}\n" +
				"{\"Action\":\"pass\",\"Package\":\"example\",\"Elapsed\":0.1}\n"
			runner := &recordingRunner{
				profile:  "mode: atomic\nexample/a.go:1.1,2.1 100 1\n",
				output:   events,
				err:      test.failure,
				metadata: packageStream(t, sourcePackage{ImportPath: "example", Dir: root, GoFiles: []string{"a.go", "doc.go"}}),
			}
			reports := filepath.Join(t.TempDir(), "native")
			arguments := []string{"--race", "--junit-output", filepath.Join(reports, "go.xml"), "--lcov-output", filepath.Join(reports, "go.lcov")}
			if test.json {
				arguments = append(arguments, "--json")
			}
			var stdout, stderr bytes.Buffer
			if code := realMain(arguments, &stdout, &stderr, runner); code != test.code {
				t.Fatalf("code = %d, want %d: %s", code, test.code, &stderr)
			}
			if test.json != (stdout.String() == events) {
				t.Fatalf("stdout = %q, json=%v", &stdout, test.json)
			}
			if !test.json && (!strings.Contains(stdout.String(), "ok  \texample") || strings.Contains(stdout.String(), "\"Action\"")) {
				t.Fatalf("plain view = %q", &stdout)
			}
			if !slices.Contains(runner.args, "-json") || runner.args[len(runner.args)-1] != "./..." {
				t.Fatalf("the one attempt must stream events: %q", runner.args)
			}
			attempts := 0
			for _, call := range runner.calls {
				if call[0] == "test" {
					attempts++
				}
			}
			if attempts != 1 {
				t.Fatalf("reports must come from one test attempt, got %d: %q", attempts, runner.calls)
			}
			junit, err := os.ReadFile(filepath.Join(reports, "go.xml"))
			if err != nil || !strings.Contains(string(junit), `name="TestValue"`) {
				t.Fatalf("JUnit report = %q, %v", junit, err)
			}
			lcov, err := os.ReadFile(filepath.Join(reports, "go.lcov"))
			if err != nil || string(lcov) != "SF:a.go\nDA:1,1\nDA:2,1\nLF:2\nLH:2\nend_of_record\nSF:doc.go\nLF:0\nLH:0\nend_of_record\n" {
				t.Fatalf("LCOV report = %q, %v", lcov, err)
			}
		})
	}
}

func TestRealMainFailsWhenAReportCannotBeWritten(t *testing.T) {
	usePolicy(t, validPolicy)
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &recordingRunner{profile: "mode: atomic\nexample/a.go:1.1,2.1 100 1\n", metadataErr: errors.New("list refused")}
	var stdout, stderr bytes.Buffer
	if code := realMain([]string{"--junit-output", filepath.Join(blocker, "go.xml"), "--lcov-output", filepath.Join(blocker, "go.lcov")}, &stdout, &stderr, runner); code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "native coverage attempt failed") || !strings.Contains(stderr.String(), "list refused") {
		t.Fatalf("report failure is not visible: %s", &stderr)
	}
}
