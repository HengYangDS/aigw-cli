package main

import (
	"slices"
	"testing"
)

func TestPackageBaseNamesRefuseARepeatedProductionName(t *testing.T) {
	files := []goFileInfo{
		{relPath: "internal/cli/client/client.go", dir: "internal/cli/client"},
		{relPath: "internal/cli/client/run.go", dir: "internal/cli/client"},
		{relPath: "internal/client/registry.go", dir: "internal/client"},
		{relPath: "tools/release/readiness/readiness.go", dir: "tools/release/readiness"},
		{relPath: "internal/app/readiness/state.go", dir: "internal/app/readiness"},
		{relPath: "internal/cli/acceptance/root_test.go", dir: "internal/cli/acceptance", isTest: true},
		{relPath: "internal/program/acceptance/peers_test.go", dir: "internal/program/acceptance", isTest: true},
		{relPath: "internal/domain/model.go", dir: "internal/domain"},
	}
	report := newReport("policy", t.TempDir())
	checkPackageBaseNames(files, policy{UniquePackageBaseNames: true}, &report)
	var got [][]string
	for _, finding := range report.Findings {
		if finding.Rule != "package_base_name" {
			t.Fatalf("unexpected finding %+v", finding)
		}
		got = append(got, append([]string{finding.Name}, finding.Files...))
	}
	want := [][]string{
		{"client", "internal/cli/client", "internal/client"},
		{"readiness", "internal/app/readiness", "tools/release/readiness"},
	}
	if !slices.EqualFunc(got, want, slices.Equal[[]string]) {
		t.Fatalf("findings = %v, want %v", got, want)
	}
	if report.Findings[0].Message != `packages internal/cli/client, internal/client share the base name "client"; name each after what it owns` {
		t.Fatalf("message = %q", report.Findings[0].Message)
	}
}

func TestPackageBaseNamesFollowThePolicy(t *testing.T) {
	files := []goFileInfo{
		{relPath: "internal/a/client/x.go", dir: "internal/a/client"},
		{relPath: "internal/b/client/x.go", dir: "internal/b/client"},
	}
	report := newReport("policy", t.TempDir())
	checkPackageBaseNames(files, policy{}, &report)
	if len(report.Findings) != 0 {
		t.Fatalf("a policy without the rule refused repeated names: %+v", report.Findings)
	}
}
