package main

import (
	"bytes"
	"strings"
	"testing"
)

// The streams below are what Go 1.27.4's `go test -json` wrote on 2026-11-20
// for small packages built to fail each way, without their Time fields and with
// most stack frames removed.

const timeoutStream = `{"Action":"start","Package":"shapes/hang"}
{"Action":"run","Package":"shapes/hang","Test":"TestQuick"}
{"Action":"output","Package":"shapes/hang","Test":"TestQuick","Output":"=== RUN   TestQuick\n","OutputType":"frame"}
{"Action":"output","Package":"shapes/hang","Test":"TestQuick","Output":"    hang_test.go:8: quick output\n"}
{"Action":"output","Package":"shapes/hang","Test":"TestQuick","Output":"--- PASS: TestQuick (0.00s)\n","OutputType":"frame"}
{"Action":"pass","Package":"shapes/hang","Test":"TestQuick","Elapsed":0}
{"Action":"run","Package":"shapes/hang","Test":"TestHang"}
{"Action":"output","Package":"shapes/hang","Test":"TestHang","Output":"=== RUN   TestHang\n","OutputType":"frame"}
{"Action":"output","Package":"shapes/hang","Test":"TestHang","Output":"    hang_test.go:11: before the hang\n"}
{"Action":"output","Package":"shapes/hang","Test":"TestHang","Output":"panic: test timed out after 2s\n"}
{"Action":"output","Package":"shapes/hang","Test":"TestHang","Output":"\trunning tests:\n"}
{"Action":"output","Package":"shapes/hang","Test":"TestHang","Output":"\t\tTestHang (2s)\n"}
{"Action":"output","Package":"shapes/hang","Output":"FAIL\tshapes/hang\t2.426s\n","OutputType":"frame"}
{"Action":"fail","Package":"shapes/hang","Elapsed":2.427}
`

const panicDuringAnotherTestStream = `{"Action":"start","Package":"shapes/latepanic"}
{"Action":"run","Package":"shapes/latepanic","Test":"TestPassesThenPanics"}
{"Action":"output","Package":"shapes/latepanic","Test":"TestPassesThenPanics","Output":"=== RUN   TestPassesThenPanics\n","OutputType":"frame"}
{"Action":"output","Package":"shapes/latepanic","Test":"TestPassesThenPanics","Output":"--- PASS: TestPassesThenPanics (0.00s)\n","OutputType":"frame"}
{"Action":"pass","Package":"shapes/latepanic","Test":"TestPassesThenPanics","Elapsed":0}
{"Action":"run","Package":"shapes/latepanic","Test":"TestWaits"}
{"Action":"output","Package":"shapes/latepanic","Test":"TestWaits","Output":"=== RUN   TestWaits\n","OutputType":"frame"}
{"Action":"output","Package":"shapes/latepanic","Test":"TestWaits","Output":"    panic_test.go:16: waiting\n"}
{"Action":"output","Package":"shapes/latepanic","Test":"TestWaits","Output":"panic: late panic from a goroutine\n"}
{"Action":"output","Package":"shapes/latepanic","Test":"TestWaits","Output":"shapes/latepanic.TestPassesThenPanics.func1()\n"}
{"Action":"output","Package":"shapes/latepanic","Output":"FAIL\tshapes/latepanic\t0.664s\n","OutputType":"frame"}
{"Action":"fail","Package":"shapes/latepanic","Elapsed":0.664}
`

const panicAfterEveryTestStream = `{"Action":"start","Package":"shapes/afterpass"}
{"Action":"run","Package":"shapes/afterpass","Test":"TestPassesThenPanicsLater"}
{"Action":"output","Package":"shapes/afterpass","Test":"TestPassesThenPanicsLater","Output":"=== RUN   TestPassesThenPanicsLater\n","OutputType":"frame"}
{"Action":"output","Package":"shapes/afterpass","Test":"TestPassesThenPanicsLater","Output":"--- PASS: TestPassesThenPanicsLater (0.00s)\n","OutputType":"frame"}
{"Action":"pass","Package":"shapes/afterpass","Test":"TestPassesThenPanicsLater","Elapsed":0}
{"Action":"output","Package":"shapes/afterpass","Output":"PASS\n","OutputType":"frame"}
{"Action":"output","Package":"shapes/afterpass","Output":"panic: panic after every test passed\n"}
{"Action":"output","Package":"shapes/afterpass","Output":"shapes/afterpass.TestPassesThenPanicsLater.func1()\n"}
{"Action":"output","Package":"shapes/afterpass","Output":"FAIL\tshapes/afterpass\t0.625s\n","OutputType":"frame"}
{"Action":"fail","Package":"shapes/afterpass","Elapsed":0.626}
`

const dataRaceStream = `{"Action":"start","Package":"shapes/race"}
{"Action":"run","Package":"shapes/race","Test":"TestRacePassesFirst"}
{"Action":"output","Package":"shapes/race","Test":"TestRacePassesFirst","Output":"=== RUN   TestRacePassesFirst\n","OutputType":"frame"}
{"Action":"output","Package":"shapes/race","Test":"TestRacePassesFirst","Output":"==================\n"}
{"Action":"output","Package":"shapes/race","Test":"TestRacePassesFirst","Output":"WARNING: DATA RACE\n"}
{"Action":"output","Package":"shapes/race","Test":"TestRacePassesFirst","Output":"Read at 0x00010240c268 by goroutine 8:\n"}
{"Action":"output","Package":"shapes/race","Test":"TestRacePassesFirst","Output":"==================\n"}
{"Action":"output","Package":"shapes/race","Test":"TestRacePassesFirst","Output":"    testing.go:1867: race detected during execution of test\n","OutputType":"error"}
{"Action":"output","Package":"shapes/race","Test":"TestRacePassesFirst","Output":"--- FAIL: TestRacePassesFirst (0.10s)\n","OutputType":"frame"}
{"Action":"fail","Package":"shapes/race","Test":"TestRacePassesFirst","Elapsed":0.1}
{"Action":"run","Package":"shapes/race","Test":"TestAfter"}
{"Action":"output","Package":"shapes/race","Test":"TestAfter","Output":"=== RUN   TestAfter\n","OutputType":"frame"}
{"Action":"output","Package":"shapes/race","Test":"TestAfter","Output":"--- PASS: TestAfter (0.20s)\n","OutputType":"frame"}
{"Action":"pass","Package":"shapes/race","Test":"TestAfter","Elapsed":0.2}
{"Action":"output","Package":"shapes/race","Output":"FAIL\n","OutputType":"frame"}
{"Action":"output","Package":"shapes/race","Output":"FAIL\tshapes/race\t0.727s\n","OutputType":"frame"}
{"Action":"fail","Package":"shapes/race","Elapsed":0.728}
`

// lateOutputStream is the one shape not observed: output for a test after its
// result. It is kept so that such output is not lost.
const lateOutputStream = `{"Action":"run","Package":"shapes/late","Test":"TestPassed"}
{"Action":"pass","Package":"shapes/late","Test":"TestPassed","Elapsed":0}
{"Action":"output","Package":"shapes/late","Test":"TestPassed","Output":"late report after the result\n"}
{"Action":"output","Package":"shapes/late","Output":"FAIL\tshapes/late\t0.1s\n"}
{"Action":"fail","Package":"shapes/late","Elapsed":0.1}
`

func TestEventRecorderShowsWhatAFailedPackageLeftUnreported(t *testing.T) {
	for _, test := range []struct {
		name, stream string
		shown        []string
		hidden       string
		// failing names the test case that must carry the failure, or the
		// package failure case.
		failing, failure string
	}{
		{"timeout", timeoutStream, []string{"before the hang", "panic: test timed out after 2s", "TestHang (2s)"}, "quick output", "TestHang", "timed out"},
		{"panic during another test", panicDuringAnotherTestStream, []string{"panic: late panic from a goroutine"}, "--- PASS: TestPassesThenPanics", "TestWaits", "late panic"},
		{"panic after every test passed", panicAfterEveryTestStream, []string{"panic: panic after every test passed"}, "--- PASS: TestPassesThenPanicsLater", packageFailureCase, "after every test passed"},
		{"data race", dataRaceStream, []string{"WARNING: DATA RACE", "race detected"}, "--- PASS: TestAfter", "TestRacePassesFirst", "DATA RACE"},
		{"output after a result", lateOutputStream, []string{"late report after the result"}, "", packageFailureCase, "late report after the result"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var view bytes.Buffer
			recorder := newEventRecorder(nil, &view)
			writeInChunks(t, recorder, test.stream)
			if err := recorder.finish(); err != nil {
				t.Fatal(err)
			}
			for _, want := range test.shown {
				if !strings.Contains(view.String(), want) {
					t.Fatalf("plain view lacks %q:\n%s", want, &view)
				}
			}
			if test.hidden != "" && strings.Contains(view.String(), test.hidden) {
				t.Fatalf("plain view shows a passed test's output %q:\n%s", test.hidden, &view)
			}
			report := decodeJUnit(t, recorder)
			if len(report.Suites) != 1 || report.Suites[0].Failures+report.Suites[0].Errors != 1 {
				t.Fatalf("want one suite with one failing case: %+v", report.Suites)
			}
			for _, testCase := range report.Suites[0].Cases {
				failed := testCase.Failure != nil || testCase.Error != nil
				if failed != (testCase.Name == test.failing) {
					t.Fatalf("case %q failed=%v, want the failure on %q", testCase.Name, failed, test.failing)
				}
				if failed && !strings.Contains(failureText(testCase), test.failure) {
					t.Fatalf("case %q lost its output: %+v", testCase.Name, testCase)
				}
			}
		})
	}
}

func failureText(testCase junitCase) string {
	if testCase.Failure != nil {
		return testCase.Failure.Text
	}
	return testCase.Error.Text
}
