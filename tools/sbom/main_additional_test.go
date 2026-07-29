package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRunSBOMUsesRealModuleMetadata(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	getenv := func(key string) string {
		if key == "SOURCE_DATE_EPOCH" {
			return "1784246400"
		}
		return ""
	}
	if code := run([]string{"-version", "1.2.3"}, getenv, &stdout, &stderr); code != 0 {
		t.Fatalf("run() code = %d, stderr = %q", code, stderr.String())
	}
	var document map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &document); err != nil {
		t.Fatalf("decode output: %v\n%s", err, stdout.String())
	}
	if document["name"] != "aigw-1.2.3" {
		t.Fatalf("document name = %#v", document["name"])
	}
	creation := document["creationInfo"].(map[string]any)
	if creation["created"] != "2026-07-17T00:00:00Z" {
		t.Fatalf("creation time = %#v", creation["created"])
	}
	if packages := document["packages"].([]any); len(packages) < 2 {
		t.Fatalf("package count = %d, want root and dependencies", len(packages))
	}
}

func TestRunSBOMRejectsInvalidArguments(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		getenv func(string) string
		want   string
	}{
		{name: "missing epoch", getenv: func(string) string { return "" }, want: "SOURCE_DATE_EPOCH"},
		{name: "unknown flag", args: []string{"-unknown"}, getenv: func(string) string { return "1" }, want: "flag provided but not defined"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			if code := run(test.args, test.getenv, &stdout, &stderr); code != 2 {
				t.Fatalf("run() code = %d, want 2", code)
			}
			if !strings.Contains(stderr.String(), test.want) {
				t.Fatalf("run() stderr = %q, want %q", stderr.String(), test.want)
			}
		})
	}
}

func TestWriteSBOMFiltersVersionlessModules(t *testing.T) {
	modules := []module{
		{Path: "example.com/root"},
		{Path: "example.com/dependency", Version: "v1.2.3"},
	}
	var output bytes.Buffer
	if err := writeSBOM(&output, "test", time.Unix(1, 0).UTC(), modules); err != nil {
		t.Fatal(err)
	}
	var document struct {
		Packages []spdxPackage `json:"packages"`
	}
	if err := json.Unmarshal(output.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if len(document.Packages) != 2 || document.Packages[1].Name != "example.com/dependency" || document.Packages[1].SPDXID != "SPDXRef-Dependency-1" {
		t.Fatalf("packages = %#v", document.Packages)
	}
}

func TestWriteSBOMReportsOutputFailure(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "closed")
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := writeSBOM(file, "test", time.Unix(1, 0).UTC(), nil); err == nil {
		t.Fatal("writeSBOM() error = nil")
	}
}

func TestLoadModulesAdditionalFailures(t *testing.T) {
	if _, err := decodeModules([]byte("{")); err == nil || !strings.Contains(err.Error(), "decode go list module metadata") {
		t.Fatalf("decodeModules() error = %v", err)
	}
}

func TestRunSBOMReportsModuleAndOutputFailures(t *testing.T) {
	getenv := func(string) string { return "1" }

	t.Run("module query", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		if code := run(nil, getenv, &stdout, &stderr); code != 1 {
			t.Fatalf("run() code = %d, want 1", code)
		}
		if !strings.Contains(stderr.String(), "run go list") {
			t.Fatalf("run() stderr = %q", stderr.String())
		}
	})

	t.Run("document output", func(t *testing.T) {
		file, err := os.CreateTemp(t.TempDir(), "closed")
		if err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		var stderr bytes.Buffer
		if code := run(nil, getenv, file, &stderr); code != 1 {
			t.Fatalf("run() code = %d, want 1", code)
		}
		if stderr.Len() == 0 {
			t.Fatal("run() stderr is empty")
		}
	})
}
