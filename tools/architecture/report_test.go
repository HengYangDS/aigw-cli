package main

import (
	"bytes"
	"strings"
	"testing"
)

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
	if idxA, idxB := strings.Index(out, `"rule": "a"`), strings.Index(out, `"rule": "b"`); idxA < 0 || idxB < 0 || idxA > idxB {
		t.Fatalf("unstable order: %s", out)
	}
}

func TestFinalizeSortTies(t *testing.T) {
	report := newReport("p", ".")
	report.addFinding(Finding{Rule: "a", Path: "z", Message: "path-z"})
	report.addFinding(Finding{Rule: "a", Path: "a", Message: "path-a"})
	report.addFinding(Finding{Rule: "a", Path: "p", Line: 1, Prefix: "b", Name: "n2", Message: "m2"})
	report.addFinding(Finding{Rule: "a", Path: "p", Line: 1, Prefix: "a", Name: "n1", Message: "m1"})
	report.addFinding(Finding{Rule: "a", Path: "p", Line: 1, Prefix: "a", Name: "n1", Message: "m0"})
	if report.Summary["total"] != 5 {
		t.Fatalf("pre-summary=%v", report.Summary)
	}
	report.Summary = nil
	report.finalize()
	if report.Summary["total"] != 0 {
		t.Fatalf("summary=%v", report.Summary)
	}
	if report.Findings[0].Path != "a" || report.Findings[1].Prefix != "a" || report.Findings[1].Message != "m0" {
		t.Fatalf("findings=%+v", report.Findings)
	}

	report = newReport("p", ".")
	report.addFinding(Finding{Rule: "z", Path: "p", Line: 2, Message: "m"})
	report.addFinding(Finding{Rule: "z", Path: "p", Line: 1, Message: "m"})
	report.finalize()
	if report.Findings[0].Line != 1 {
		t.Fatalf("line sort: %+v", report.Findings)
	}
}

func TestFinalizeNameAndMessageOrder(t *testing.T) {
	report := newReport("p", ".")
	report.addFinding(Finding{Rule: "a", Path: "p", Line: 1, Prefix: "x", Name: "b", Message: "m2"})
	report.addFinding(Finding{Rule: "a", Path: "p", Line: 1, Prefix: "x", Name: "a", Message: "m1"})
	report.addFinding(Finding{Rule: "a", Path: "p", Line: 1, Prefix: "x", Name: "a", Message: "m0"})
	report.finalize()
	if report.Findings[0].Name != "a" || report.Findings[0].Message != "m0" {
		t.Fatalf("%+v", report.Findings)
	}
}
