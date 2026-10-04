package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestNPMDependencyGraphHasNoDeprecatedPackages(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(repositoryRoot(t), "package-lock.json"))
	if err != nil {
		t.Fatal(err)
	}
	var lock struct {
		Packages map[string]struct {
			Deprecated string `json:"deprecated"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(content, &lock); err != nil {
		t.Fatal(err)
	}

	var deprecated []string
	for name, dependency := range lock.Packages {
		if dependency.Deprecated != "" {
			deprecated = append(deprecated, name+": "+dependency.Deprecated)
		}
	}
	if len(deprecated) != 0 {
		sort.Strings(deprecated)
		t.Fatalf("deprecated npm dependencies:\n%s", strings.Join(deprecated, "\n"))
	}
}

func TestNativeDependencyDispositionPreservesRawAndOtherFindings(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "cache", "osv-scalibr", "npm")
	if err := os.MkdirAll(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	for path, value := range map[string]string{
		"package-lock.json": `{"lockfileVersion":3,"packages":{"node_modules/braces":{"version":"3.0.3","dev":true}}}`,
		"raw.toml":          "IgnoredVulns = []\n",
		"policy.toml":       "[[IgnoredVulns]]\nid = 'GHSA-vfj7-8cjw-p6xm'\nreason = 'Fixture exact approved advisory'\n",
		"osv-scanner.toml":  "[[PackageOverrides]]\nignore = true\n",
	} {
		if err := os.WriteFile(filepath.Join(root, path), []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, unrelated := range []bool{false, true} {
		var database bytes.Buffer
		writer := zip.NewWriter(&database)
		ids := []string{"GHSA-vfj7-8cjw-p6xm"}
		if unrelated {
			ids = append(ids, "OSV-FIXTURE-UNRELATED")
		}
		for _, id := range ids {
			entry, err := writer.Create(id + ".json")
			if err != nil {
				t.Fatal(err)
			}
			finding := map[string]any{"schema_version": "1.7.0", "id": id, "modified": "2026-10-04T00:00:00Z", "affected": []any{map[string]any{"package": map[string]string{"ecosystem": "npm", "name": "braces"}, "versions": []string{"3.0.3"}}}}
			if err := json.NewEncoder(entry).Encode(finding); err != nil {
				t.Fatal(err)
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(cache, "all.zip"), database.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
		for _, policy := range []string{"raw.toml", "policy.toml"} {
			call := exec.CommandContext(t.Context(), "osv-scanner", "scan", "source", "--offline", "--no-ignore", "--no-call-analysis=go", "--config", filepath.Join(root, policy), "--lockfile", filepath.Join(root, "package-lock.json"), "--format", "json", "--all-packages", "--all-vulns")
			call.Dir = root
			call.Env = append(call.Environ(), "OSV_SCALIBR_LOCAL_DB_CACHE_DIRECTORY="+filepath.Join(root, "cache"))
			content, err := call.Output()
			wantFailure := policy == "raw.toml" || unrelated
			if (err != nil) != wantFailure {
				t.Fatalf("native disposition: policy=%s unrelated=%v error=%v", policy, unrelated, err)
			}
			if err != nil {
				exit, ok := errors.AsType[*exec.ExitError](err)
				if !ok || exit.ExitCode() != 1 {
					t.Fatalf("native scan did not report a finding: %v", err)
				}
			}
			if bytes.Contains(content, []byte("GHSA-vfj7-8cjw-p6xm")) != (policy == "raw.toml") {
				t.Fatalf("native raw/admitted finding contract: %s", content)
			}
			if unrelated && !bytes.Contains(content, []byte("OSV-FIXTURE-UNRELATED")) {
				t.Fatalf("unapproved native finding was lost: %s", content)
			}
		}
	}
}
