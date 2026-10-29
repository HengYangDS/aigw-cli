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

// A syntax error elsewhere stays the Go toolchain's to report, even when the
// rule must read every file for its users.
func TestRunWithUsedExportsOnlyLeavesGoSyntaxToGoToolchain(t *testing.T) {
	root := t.TempDir()
	body := strings.Replace(validPolicy, "  scripts = {", "  module = { responsibility = \"module fixture\", exact_paths = [\"go.mod\"] },\n  scripts = {", 1)
	policyPath := writePolicy(t, root, body+"used_exports_only = [\"internal/model\"]\n")
	writeFile(t, filepath.Join(root, "go.mod"), "module fixture\n")
	writeFile(t, filepath.Join(root, "internal", "model", "model.go"), "// Package model owns the fixture model.\npackage model\n\n// Used has a caller.\nfunc Used() {}\n")
	writeFile(t, filepath.Join(root, "internal", "user", "user.go"), "// Package user calls the model.\npackage user\n\nimport \"fixture/internal/model\"\n\nvar _ = model.Used\n")
	writeFile(t, filepath.Join(root, "internal", "bad", "bad.go"), "package bad\nfunc (\n")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-root", root, "-policy", policyPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("code=%d want 0 stderr=%q stdout=%s", code, stderr.String(), stdout.String())
	}
}

// usedExportsModel is the listed fixture package: each export is used, or
// reached, in exactly one way, or by nothing.
const usedExportsModel = `package model

import "example.com/aigw/internal/other"

type Kind int

const (
	First Kind = iota
	Second
	Reset = 5
	AfterReset
)

const Untyped = 1

type Result struct {
	Embedded
	promoted
	Exported Detail
	hidden   Hidden
}

type Embedded struct{}

type promoted struct{ Deep Promoted }

type Promoted struct{}

type Detail struct{}

type Hidden struct{}

type Via struct{}

type ViaPointer struct{}

func (Result) Method() Via { return Via{} }

func (*Result) PointerMethod() ViaPointer { return ViaPointer{} }

type Constraint interface{ ~int }

type Box[T Constraint] struct{ Value T }

type ViaBox struct{}

func (Box[T]) Get() ViaBox { return ViaBox{} }

type Pair[K comparable, V any] struct{}

type ViaPair struct{}

func (Pair[K, V]) Get() ViaPair { return ViaPair{} }

type Handler interface {
	Handle(Arg) error
	hidden(HiddenArg)
}

type Arg struct{}

type HiddenArg struct{}

type Thing struct{}

func Used(Kind) Result { return Result{} }

func UseBox() Box[int] { return Box[int]{} }

func UsePair() Pair[string, int] { return Pair[string, int]{} }

func Serve(Handler) {}

func Bridge(other.Thing) {}

func OnlyOwnTests() {}

func OnlyExternalTest() {}

func ByOtherTest() {}

func Aliased() {}

func DotUsed() {}

func BrokenUser() {}

func unexported() {}

type unexportedType struct{}

var Variable = 1
`

// writeUsedExportsFixture writes the listed package beside its users and
// returns the files that the gate collects.
func writeUsedExportsFixture(t *testing.T, root string) []goFileInfo {
	t.Helper()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/aigw\n")
	writeFile(t, filepath.Join(root, "internal", "model", "model.go"), usedExportsModel)
	// One declaration per platform: each file's API counts.
	writeFile(t, filepath.Join(root, "internal", "model", "platform_a.go"), "//go:build a\n\npackage model\n\ntype PlatformA struct{}\n\nfunc Platform() PlatformA { return PlatformA{} }\n")
	writeFile(t, filepath.Join(root, "internal", "model", "platform_b.go"), "//go:build b\n\npackage model\n\ntype PlatformB struct{}\n\nfunc Platform() PlatformB { return PlatformB{} }\n")
	// Go's parser accepts a method without a receiver.
	writeFile(t, filepath.Join(root, "internal", "model", "odd.go"), "package model\n\nfunc () Odd() {}\n")
	writeFile(t, filepath.Join(root, "internal", "model", "model_test.go"), "package model\n\nfunc ExportedInTest() { OnlyOwnTests() }\n")
	writeFile(t, filepath.Join(root, "internal", "model", "external_test.go"), "package model_test\n\nimport \"example.com/aigw/internal/model\"\n\nvar _ = model.OnlyExternalTest\n")
	writeFile(t, filepath.Join(root, "internal", "other", "other.go"), "package other\n\ntype Thing struct{}\n")
	writeFile(t, filepath.Join(root, "internal", "user", "user.go"), `package user

import "example.com/aigw/internal/model"

var _, _, _, _, _, _ = model.Used, model.UseBox, model.UsePair, model.Serve, model.Bridge, model.Platform

func shadow() int {
	model := struct{ Variable int }{}
	return model.Variable
}
`)
	writeFile(t, filepath.Join(root, "internal", "user", "user_test.go"), "package user\n\nimport \"example.com/aigw/internal/model\"\n\nvar _ = model.ByOtherTest\n")
	writeFile(t, filepath.Join(root, "internal", "alias", "alias.go"), "package alias\n\nimport m \"example.com/aigw/internal/model\"\n\nvar _ = m.Aliased\n")
	writeFile(t, filepath.Join(root, "internal", "dot", "dot.go"), "package dot\n\nimport . \"example.com/aigw/internal/model\"\n\nvar _ = DotUsed\n")
	// What parses before a syntax error still counts.
	writeFile(t, filepath.Join(root, "internal", "broken", "broken.go"), "package broken\n\nimport \"example.com/aigw/internal/model\"\n\nvar _ = model.BrokenUser\n\nfunc (\n")
	return []goFileInfo{
		{relPath: "internal/alias/alias.go", dir: "internal/alias"},
		{relPath: "internal/broken/broken.go", dir: "internal/broken"},
		{relPath: "internal/dot/dot.go", dir: "internal/dot"},
		{relPath: "internal/model/external_test.go", dir: "internal/model", isTest: true},
		{relPath: "internal/model/model.go", dir: "internal/model"},
		{relPath: "internal/model/model_test.go", dir: "internal/model", isTest: true},
		{relPath: "internal/model/odd.go", dir: "internal/model"},
		{relPath: "internal/model/platform_a.go", dir: "internal/model"},
		{relPath: "internal/model/platform_b.go", dir: "internal/model"},
		{relPath: "internal/other/other.go", dir: "internal/other"},
		{relPath: "internal/user/user.go", dir: "internal/user"},
		{relPath: "internal/user/user_test.go", dir: "internal/user", isTest: true},
	}
}

// The listed package's exported surface is used when another package selects
// an identifier or reaches it through the exported API of one it selects.
func TestUsedExportsOnlyCountsOnlyUsersOutsideThePackage(t *testing.T) {
	root := t.TempDir()
	files := writeUsedExportsFixture(t, root)
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
	want := []string{"AfterReset", "Hidden", "HiddenArg", "OnlyExternalTest", "OnlyOwnTests", "Reset", "Thing", "Untyped", "Variable"}
	if !slices.Equal(unused, want) {
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
