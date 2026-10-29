package main

import (
	"bytes"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Every run applies the rule: a listed package that exports an identifier no
// other package uses fails the gate.
func TestRunEnforcesUsedExportsOnly(t *testing.T) {
	root := t.TempDir()
	policyPath := writePolicy(t, root, validPolicy+"used_exports_only = [\"internal/model\"]\n")
	writeFile(t, filepath.Join(root, "go.mod"), "module fixture\n")
	writeFile(t, filepath.Join(root, "internal", "model", "model.go"), "// Package model owns the fixture model.\npackage model\n\n// Unused is exported for no caller.\nfunc Unused() {}\n")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-root", root, "-policy", policyPath}, &stdout, &stderr); code != 1 {
		t.Fatalf("code=%d want 1 stderr=%q stdout=%s", code, stderr.String(), stdout.String())
	}
	if report := decodeReport(t, stdout.String()); !hasRule(report, "unused_export") {
		t.Fatalf("the run did not apply used_exports_only: %v", findingRules(report))
	}
}

// The listed package's exported surface is used when another package selects
// an identifier or reaches it through the exported API of one it selects.
func TestUsedExportsOnlyCountsOnlyUsersOutsideThePackage(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/aigw\n")
	writeFile(t, filepath.Join(root, "internal", "model", "model.go"), `package model

type Kind int

const (
	First Kind = iota
	Second
)

const Untyped = 1

type Result struct {
	Exported Detail
	hidden   Hidden
}

type Detail struct{}

type Hidden struct{}

type Via struct{}

func (Result) Method() Via { return Via{} }

type Constraint interface{ ~int }

type Box[T Constraint] struct{ Value T }

func Used(Kind) Result { return Result{} }

func UseBox() Box[int] { return Box[int]{} }

func OnlyOwnTests() {}

func OnlyExternalTest() {}

func ByOtherTest() {}

func Aliased() {}

func DotUsed() {}

func unexported() {}

var Variable = 1
`)
	writeFile(t, filepath.Join(root, "internal", "model", "model_test.go"), "package model\n\nfunc ExportedInTest() { OnlyOwnTests() }\n")
	writeFile(t, filepath.Join(root, "internal", "model", "external_test.go"), "package model_test\n\nimport \"example.com/aigw/internal/model\"\n\nvar _ = model.OnlyExternalTest\n")
	writeFile(t, filepath.Join(root, "internal", "user", "user.go"), `package user

import "example.com/aigw/internal/model"

var _, _ = model.Used, model.UseBox

func shadow() int {
	model := struct{ Variable int }{}
	return model.Variable
}
`)
	writeFile(t, filepath.Join(root, "internal", "user", "user_test.go"), "package user\n\nimport \"example.com/aigw/internal/model\"\n\nvar _ = model.ByOtherTest\n")
	writeFile(t, filepath.Join(root, "internal", "alias", "alias.go"), "package alias\n\nimport m \"example.com/aigw/internal/model\"\n\nvar _ = m.Aliased\n")
	writeFile(t, filepath.Join(root, "internal", "dot", "dot.go"), "package dot\n\nimport . \"example.com/aigw/internal/model\"\n\nvar _ = DotUsed\n")
	files := []goFileInfo{
		{relPath: "internal/alias/alias.go", dir: "internal/alias"},
		{relPath: "internal/dot/dot.go", dir: "internal/dot"},
		{relPath: "internal/model/external_test.go", dir: "internal/model", isTest: true},
		{relPath: "internal/model/model.go", dir: "internal/model"},
		{relPath: "internal/model/model_test.go", dir: "internal/model", isTest: true},
		{relPath: "internal/user/user.go", dir: "internal/user"},
		{relPath: "internal/user/user_test.go", dir: "internal/user", isTest: true},
	}
	report := newReport("policy", root)
	if err := checkUsedExportsOnly(root, files, policy{UsedExportsOnly: []string{"internal/model"}}, &report); err != nil {
		t.Fatal(err)
	}
	var unused []string
	for _, finding := range report.Findings {
		if finding.Rule != "unused_export" || finding.Path != "internal/model/model.go" || finding.Line == 0 {
			t.Fatalf("unexpected finding %+v", finding)
		}
		name := strings.TrimSuffix(strings.Fields(finding.Message)[3], ",")
		unused = append(unused, name)
	}
	slices.Sort(unused)
	if want := []string{"Hidden", "OnlyExternalTest", "OnlyOwnTests", "Untyped", "Variable"}; !slices.Equal(unused, want) {
		t.Fatalf("unused exports = %v, want %v", unused, want)
	}
}

func TestUsedExportsOnlyReportsAStaleEntry(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/aigw\n")
	writeFile(t, filepath.Join(root, "internal", "fixtures", "fixture_test.go"), "package fixtures\n\nfunc Exported() {}\n")
	files := []goFileInfo{{relPath: "internal/fixtures/fixture_test.go", dir: "internal/fixtures", isTest: true}}
	report := newReport("policy", root)
	if err := checkUsedExportsOnly(root, files, policy{UsedExportsOnly: []string{"internal/retired", "internal/fixtures"}}, &report); err != nil {
		t.Fatal(err)
	}
	if countRule(report, "used_exports_only_entry") != 2 || len(report.Findings) != 2 {
		t.Fatalf("a listed package without production files was accepted: %+v", report.Findings)
	}
}

func TestPolicyRejectsInvalidUsedExportsOnlyEntries(t *testing.T) {
	for _, entries := range [][]string{{"internal/domain", "internal/domain"}, {"../domain"}, {""}} {
		p := policy{GoRoots: []string{"internal"}, UsedExportsOnly: entries}
		if err := validatePackagePolicy(p); err == nil || !strings.Contains(err.Error(), "used_exports_only") {
			t.Fatalf("entries %q were accepted: %v", entries, err)
		}
	}
}
