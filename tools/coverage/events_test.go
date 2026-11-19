package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const nativeEvents = `{"Action":"start","Package":"example/a"}
{"Action":"run","Package":"example/a","Test":"TestPass"}
{"Action":"output","Package":"example/a","Test":"TestPass","Output":"=== RUN   TestPass\n"}
{"Action":"pass","Package":"example/a","Test":"TestPass","Elapsed":0.25}
{"Action":"run","Package":"example/a","Test":"TestFail"}
{"Action":"output","Package":"example/a","Test":"TestFail","Output":"    a_test.go:9: wrong value \u001b[31m<&>\n"}
{"Action":"fail","Package":"example/a","Test":"TestFail","Elapsed":0.5}
{"Action":"run","Package":"example/a","Test":"TestSkip"}
{"Action":"output","Package":"example/a","Test":"TestSkip","Output":"    a_test.go:12: not on this platform\n"}
{"Action":"skip","Package":"example/a","Test":"TestSkip"}
{"Action":"run","Package":"example/a","Test":"TestParent/child"}
{"Action":"pass","Package":"example/a","Test":"TestParent/child","Elapsed":0.1}
{"Action":"run","Package":"example/a","Test":"TestHang"}
{"Action":"output","Package":"example/a","Test":"TestHang","Output":"panic: test timed out\n"}
{"Action":"output","Package":"example/a","Output":"FAIL\texample/a\t1.200s\n"}
{"Action":"fail","Package":"example/a","Elapsed":1.2}
{"Action":"output","Package":"example/empty","Output":"?   \texample/empty\t[no test files]\n"}
{"Action":"skip","Package":"example/empty"}
{"ImportPath":"example/broken","Action":"build-output","Output":"broken.go:3:1: syntax error\n"}
{"ImportPath":"example/broken","Action":"build-fail"}
{"Action":"output","Package":"example/broken","Output":"FAIL\texample/broken [build failed]\n"}
{"Action":"fail","Package":"example/broken","Elapsed":0}
{"Action":"run","Package":"example/ok","Test":"TestOK"}
{"Action":"pass","Package":"example/ok","Test":"TestOK","Elapsed":0.01}
{"Action":"output","Package":"example/ok","Output":"ok  \texample/ok\t0.020s\n"}
{"Action":"pass","Package":"example/ok","Elapsed":0.02}`

// writeInChunks splits the stream across writes, so no event is assumed to
// arrive whole.
func writeInChunks(t *testing.T, writer io.Writer, data string) {
	t.Helper()
	for len(data) > 0 {
		size := min(7, len(data))
		if _, err := io.WriteString(writer, data[:size]); err != nil {
			t.Fatal(err)
		}
		data = data[size:]
	}
}

func decodeJUnit(t *testing.T, recorder *eventRecorder) junitSuites {
	t.Helper()
	var buffer bytes.Buffer
	if err := recorder.writeJUnit(&buffer); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(buffer.String(), xml.Header) {
		t.Fatalf("report lacks the XML header: %q", buffer.String()[:min(40, buffer.Len())])
	}
	var report junitSuites
	if err := xml.Unmarshal(buffer.Bytes(), &report); err != nil {
		t.Fatalf("report is not XML: %v\n%s", err, &buffer)
	}
	return report
}

func TestEventRecorderReportsEveryOutcomeOfTheAttempt(t *testing.T) {
	var view bytes.Buffer
	recorder := newEventRecorder(nil, &view)
	writeInChunks(t, recorder, nativeEvents)
	if err := recorder.finish(); err != nil {
		t.Fatal(err)
	}
	report := decodeJUnit(t, recorder)
	if len(report.Suites) != 3 {
		t.Fatalf("suites = %d, want example/a, example/broken and example/ok: %+v", len(report.Suites), report.Suites)
	}
	requireTestOutcomes(t, report.Suites[0])
	broken, ok := report.Suites[1], report.Suites[2]
	if broken.Tests != 1 || broken.Errors != 1 || broken.Cases[0].Name != packageFailureCase ||
		!strings.Contains(broken.Cases[0].Error.Text, "build failed") {
		t.Fatalf("a package failure outside any test must be an error case: %+v", broken)
	}
	if ok.Tests != 1 || ok.Failures+ok.Errors+ok.Skipped != 0 {
		t.Fatalf("passing package = %+v", ok)
	}
	requirePlainView(t, view.String())
}

// requireTestOutcomes checks each test result of the example/a package.
func requireTestOutcomes(t *testing.T, a junitSuite) {
	t.Helper()
	if a.Name != "example/a" || a.Tests != 5 || a.Failures != 1 || a.Skipped != 1 || a.Errors != 1 || a.Time != "1.200" {
		t.Fatalf("example/a suite = %+v", a)
	}
	outcomes := map[string]junitCase{}
	for _, testCase := range a.Cases {
		if testCase.Classname != "example/a" {
			t.Fatalf("case %q has class %q", testCase.Name, testCase.Classname)
		}
		outcomes[testCase.Name] = testCase
	}
	if pass := outcomes["TestPass"]; pass.Failure != nil || pass.Error != nil || pass.Skipped != nil || pass.Time != "0.250" {
		t.Fatalf("passing case = %+v", pass)
	}
	if fail := outcomes["TestFail"]; fail.Failure == nil || !strings.Contains(fail.Failure.Text, "wrong value") {
		t.Fatalf("failing case lost its output: %+v", fail)
	}
	if skip := outcomes["TestSkip"]; skip.Skipped == nil || !strings.Contains(skip.Skipped.Text, "not on this platform") {
		t.Fatalf("skipped case = %+v", skip)
	}
	if child := outcomes["TestParent/child"]; child.Failure != nil || child.Error != nil {
		t.Fatalf("subtest case = %+v", child)
	}
	if hang := outcomes["TestHang"]; hang.Error == nil || !strings.Contains(hang.Error.Text, "timed out") {
		t.Fatalf("a test without a result must be an error: %+v", hang)
	}
}

// requirePlainView checks that the plain view shows package lines and
// failures, but not a passing test's output.
func requirePlainView(t *testing.T, shown string) {
	t.Helper()
	for _, want := range []string{"wrong value", "FAIL\texample/a", "syntax error", "ok  \texample/ok", "[no test files]"} {
		if !strings.Contains(shown, want) {
			t.Fatalf("plain view lacks %q:\n%s", want, shown)
		}
	}
	if strings.Contains(shown, "=== RUN   TestPass") {
		t.Fatalf("plain view shows a passing test's output:\n%s", shown)
	}
}

func TestEventRecorderPassesNativeBytesUnchanged(t *testing.T) {
	var raw bytes.Buffer
	recorder := newEventRecorder(&raw, nil)
	writeInChunks(t, recorder, nativeEvents)
	if err := recorder.finish(); err != nil {
		t.Fatal(err)
	}
	if raw.String() != nativeEvents {
		t.Fatal("native event bytes changed")
	}
	if report := decodeJUnit(t, recorder); len(report.Suites) != 3 {
		t.Fatalf("native passthrough lost results: %+v", report.Suites)
	}
}

func TestEventRecorderFailsClosed(t *testing.T) {
	recorder := newEventRecorder(nil, &bytes.Buffer{})
	writeInChunks(t, recorder, "{\"Action\":\"pass\"}\nnot an event\n")
	if err := recorder.finish(); err == nil || !strings.Contains(err.Error(), "not JSON") {
		t.Fatalf("a line outside the event stream must fail the report: %v", err)
	}

	recorder = newEventRecorder(nil, rejectingWriter{})
	writeInChunks(t, recorder, nativeEvents)
	if err := recorder.finish(); err == nil || !strings.Contains(err.Error(), "write test output") {
		t.Fatalf("an unwritable view must fail the attempt: %v", err)
	}

	recorder = newEventRecorder(rejectingWriter{}, nil)
	if _, err := recorder.Write([]byte(nativeEvents)); err == nil {
		t.Fatal("an unwritable native stream must fail the write")
	}
}

func TestWriteReportPublishesOnlyCompleteReports(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "reports")
	path := filepath.Join(directory, "report.xml")
	refused := errors.New("encoder refused")
	if err := writeReport(path, func(writer io.Writer) error {
		if _, err := io.WriteString(writer, "partial"); err != nil {
			return err
		}
		return refused
	}); !errors.Is(err, refused) {
		t.Fatalf("writer failure = %v", err)
	}
	if entries, err := os.ReadDir(directory); err != nil || len(entries) != 0 {
		t.Fatalf("a failed report left files behind: %v, %v", entries, err)
	}

	if err := writeReport(path, func(writer io.Writer) error {
		_, err := io.WriteString(writer, "complete")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "complete" {
		t.Fatalf("report = %q, %v", data, err)
	}
	if entries, err := os.ReadDir(directory); err != nil || len(entries) != 1 {
		t.Fatalf("temporary report remains: %v, %v", entries, err)
	}

	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeReport(filepath.Join(blocker, "report.xml"), func(io.Writer) error { return nil }); err == nil {
		t.Fatal("a report below a regular file must fail")
	}
	if err := writeReport(directory, func(io.Writer) error { return nil }); err == nil {
		t.Fatal("a report that would replace a directory must fail")
	}
}
