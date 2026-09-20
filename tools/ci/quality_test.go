package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepositoryQualityGraphCoversTrackedCarrierAuthority(t *testing.T) {
	root := repositoryRoot(t)
	classes, err := loadTrackedCarrierClasses(filepath.Join(root, ".config", "checks", "architecture", "policy.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateQualityGraph(repositoryQualityGraph, classes); err != nil {
		t.Fatal(err)
	}
}

func TestQualityGraphRejectsIncompleteOrParallelCoverage(t *testing.T) {
	valid := repositoryQualityGraph
	valid.Carriers = []carrierQuality{{Class: "source", Required: []qualityConcern{qualityFormat, qualityTest}, Gates: []string{"format", "coverage"}}}
	tests := []struct {
		name    string
		graph   qualityGraph
		classes []string
		want    string
	}{
		{"missing carrier", valid, []string{"source", "documentation"}, "missing carrier quality coverage"},
		{"unknown carrier", valid, nil, "unknown carrier quality coverage"},
		{"unknown gate", qualityGraph{Gates: valid.Gates, CommonGates: valid.CommonGates, Carriers: []carrierQuality{{Class: "source", Required: []qualityConcern{qualityFormat}, Gates: []string{"missing"}}}}, []string{"source"}, "unknown quality gate"},
		{"missing concern", qualityGraph{Gates: valid.Gates, CommonGates: valid.CommonGates, Carriers: []carrierQuality{{Class: "source", Required: []qualityConcern{qualityType}, Gates: []string{"format"}}}}, []string{"source"}, "missing required quality concern"},
		{"unused gate", qualityGraph{Gates: valid.Gates, CommonGates: valid.CommonGates, Carriers: []carrierQuality{{Class: "source", Required: []qualityConcern{qualityFormat}, Gates: []string{"format"}}}}, []string{"source"}, "quality gate has no carrier consumer"},
		{"duplicate gate", qualityGraph{Gates: append(valid.Gates, valid.Gates[0]), CommonGates: valid.CommonGates, Carriers: valid.Carriers}, []string{"source"}, "duplicate quality gate"},
		{"duplicate carrier", qualityGraph{Gates: valid.Gates, CommonGates: valid.CommonGates, Carriers: append(valid.Carriers, valid.Carriers[0])}, []string{"source"}, "duplicate carrier quality coverage"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := validateQualityGraph(test.graph, test.classes); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("quality graph error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestQualityCoverageCommandUsesTheArchitectureCarrierAuthority(t *testing.T) {
	root := t.TempDir()
	policy := filepath.Join(root, ".config", "checks", "architecture", "policy.toml")
	write := func(content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(policy), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(policy, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("[tracked_carrier_classes.unknown]\nresponsibility = 'fixture'\nprefixes = ['fixture']\n")
	if err := checkQualityCoverage(root, nil); err == nil || !strings.Contains(err.Error(), "unknown carrier quality coverage") {
		t.Fatalf("quality coverage error = %v", err)
	}
}
