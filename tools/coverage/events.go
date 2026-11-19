package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
)

// testEvent is one record of the native `go test -json` stream (cmd/test2json).
type testEvent struct {
	Action  string  `json:"Action"`
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Elapsed float64 `json:"Elapsed"`
	Output  string  `json:"Output"`
}

type testResult struct {
	name    string
	action  string
	elapsed float64
	output  bytes.Buffer
}

type packageResult struct {
	action  string
	elapsed float64
	output  bytes.Buffer
	tests   []*testResult
	byName  map[string]*testResult
}

// eventRecorder reads the native event stream of the one test attempt as it
// arrives. It passes the bytes on unchanged when native output is selected,
// or shows package lines and failing tests' output otherwise, and keeps each
// test's result for the JUnit report.
type eventRecorder struct {
	raw      io.Writer
	view     io.Writer
	pending  []byte
	packages map[string]*packageResult
	err      error
}

func newEventRecorder(raw, view io.Writer) *eventRecorder {
	return &eventRecorder{raw: raw, view: view, packages: map[string]*packageResult{}}
}

func (r *eventRecorder) Write(data []byte) (int, error) {
	if r.raw != nil {
		if _, err := r.raw.Write(data); err != nil {
			return 0, err
		}
	}
	r.pending = append(r.pending, data...)
	start := 0
	for {
		index := bytes.IndexByte(r.pending[start:], '\n')
		if index < 0 {
			break
		}
		r.record(r.pending[start : start+index])
		start += index + 1
	}
	r.pending = append(r.pending[:0], r.pending[start:]...)
	return len(data), nil
}

// finish records a final event that lacks its newline and returns the first
// event or view error.
func (r *eventRecorder) finish() error {
	if len(r.pending) > 0 {
		r.record(r.pending)
		r.pending = nil
	}
	return r.err
}

func (r *eventRecorder) record(line []byte) {
	if len(bytes.TrimSpace(line)) == 0 {
		return
	}
	var event testEvent
	if err := json.Unmarshal(line, &event); err != nil {
		r.show(string(line) + "\n")
		r.fail(fmt.Errorf("native test event is not JSON: %w", err))
		return
	}
	switch event.Action {
	case "build-output":
		r.show(event.Output)
		return
	case "output", "run", "pass", "fail", "skip":
	default:
		return
	}
	pkg := r.packageResult(event.Package)
	if event.Test == "" {
		if event.Action == "output" {
			pkg.output.WriteString(event.Output)
			r.show(event.Output)
		} else if event.Action != "run" {
			pkg.action, pkg.elapsed = event.Action, event.Elapsed
		}
		return
	}
	test := pkg.test(event.Test)
	switch event.Action {
	case "output":
		test.output.WriteString(event.Output)
	case "pass":
		test.action, test.elapsed = event.Action, event.Elapsed
		test.output.Reset()
	case "fail", "skip":
		test.action, test.elapsed = event.Action, event.Elapsed
		if event.Action == "fail" {
			r.show(test.output.String())
		}
	}
}

func (r *eventRecorder) show(text string) {
	if r.view == nil || text == "" {
		return
	}
	if _, err := io.WriteString(r.view, text); err != nil {
		r.fail(fmt.Errorf("write test output: %w", err))
	}
}

func (r *eventRecorder) fail(err error) {
	if r.err == nil {
		r.err = err
	}
}

func (r *eventRecorder) packageResult(name string) *packageResult {
	pkg := r.packages[name]
	if pkg == nil {
		pkg = &packageResult{byName: map[string]*testResult{}}
		r.packages[name] = pkg
	}
	return pkg
}

func (pkg *packageResult) test(name string) *testResult {
	test := pkg.byName[name]
	if test == nil {
		test = &testResult{name: name}
		pkg.byName[name] = test
		pkg.tests = append(pkg.tests, test)
	}
	return test
}

type junitSuites struct {
	XMLName xml.Name     `xml:"testsuites"`
	Suites  []junitSuite `xml:"testsuite"`
}

type junitSuite struct {
	Name     string      `xml:"name,attr"`
	Tests    int         `xml:"tests,attr"`
	Failures int         `xml:"failures,attr"`
	Errors   int         `xml:"errors,attr"`
	Skipped  int         `xml:"skipped,attr"`
	Time     string      `xml:"time,attr"`
	Cases    []junitCase `xml:"testcase"`
}

type junitCase struct {
	Classname string        `xml:"classname,attr"`
	Name      string        `xml:"name,attr"`
	Time      string        `xml:"time,attr"`
	Failure   *junitMessage `xml:"failure,omitempty"`
	Error     *junitMessage `xml:"error,omitempty"`
	Skipped   *junitMessage `xml:"skipped,omitempty"`
}

type junitMessage struct {
	Message string `xml:"message,attr"`
	Text    string `xml:",chardata"`
}

// packageFailureCase names the test case that carries a package failure
// outside any test, such as a build failure or a panic in TestMain.
const packageFailureCase = "package failure"

// junit returns one suite per package that reported a test or failed, in
// package order, with one case per test result in the order tests started.
func (r *eventRecorder) junit() junitSuites {
	var report junitSuites
	for _, name := range slices.Sorted(maps.Keys(r.packages)) {
		if suite, ok := r.packages[name].suite(name); ok {
			report.Suites = append(report.Suites, suite)
		}
	}
	return report
}

func (pkg *packageResult) suite(name string) (junitSuite, bool) {
	suite := junitSuite{Name: name, Time: seconds(pkg.elapsed)}
	for _, test := range pkg.tests {
		testCase := junitCase{Classname: name, Name: test.name, Time: seconds(test.elapsed)}
		switch test.action {
		case "pass":
		case "fail":
			testCase.Failure = &junitMessage{Message: "Failed", Text: test.output.String()}
			suite.Failures++
		case "skip":
			testCase.Skipped = &junitMessage{Message: "Skipped", Text: test.output.String()}
			suite.Skipped++
		default:
			testCase.Error = &junitMessage{Message: "No result", Text: test.output.String()}
			suite.Errors++
		}
		suite.Cases = append(suite.Cases, testCase)
	}
	if pkg.action != "pass" && pkg.action != "skip" && suite.Failures+suite.Errors == 0 {
		suite.Cases = append(suite.Cases, junitCase{
			Classname: name,
			Name:      packageFailureCase,
			Time:      seconds(pkg.elapsed),
			Error:     &junitMessage{Message: "Package failed", Text: pkg.output.String()},
		})
		suite.Errors++
	}
	suite.Tests = len(suite.Cases)
	return suite, suite.Tests > 0
}

func seconds(value float64) string {
	return strconv.FormatFloat(value, 'f', 3, 64)
}

func (r *eventRecorder) writeJUnit(writer io.Writer) error {
	if _, err := io.WriteString(writer, xml.Header); err != nil {
		return err
	}
	encoder := xml.NewEncoder(writer)
	encoder.Indent("", "  ")
	if err := encoder.Encode(r.junit()); err != nil {
		return err
	}
	if err := encoder.Close(); err != nil {
		return err
	}
	_, err := io.WriteString(writer, "\n")
	return err
}

// writeReport makes a report visible at path only once it is complete.
func writeReport(path string, write func(io.Writer) error) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(directory, "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	writeErr := write(file)
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return errors.Join(err, os.Remove(temporary))
	}
	if err := os.Rename(temporary, path); err != nil {
		return errors.Join(err, os.Remove(temporary))
	}
	return nil
}
