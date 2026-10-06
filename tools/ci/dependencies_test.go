package main

import (
	"aigw-cli/tools/release/construction"
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestNativeMathConsumersRejectInheritedTrust(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	code := `
import assert from "node:assert/strict";
import { createRequire } from "node:module";
for (const consumer of ["mermaid", "micromark-extension-math"]) {
  const require = createRequire(import.meta.resolve(consumer));
  const { default: katex } = await import(require.resolve("katex"));
  const expression = "\\href{https://example.invalid/owned}{owned}";
  assert.ok(!katex.renderToString(expression).includes("<a "), consumer);
  Object.prototype.trust = true;
  try {
    assert.ok(!katex.renderToString(expression).includes("<a "), consumer);
  } finally {
    delete Object.prototype.trust;
  }
  assert.ok(katex.renderToString(expression, { trust: true }).includes("<a "), consumer);
  for (const output of ["mathml", "htmlAndMathml"]) {
    assert.ok(katex.renderToString("x^2", {
      throwOnError: true, displayMode: true, output,
    }).includes("<math"), consumer);
  }
}
const { micromark } = await import("micromark");
const { math, mathHtml } = await import("micromark-extension-math");
assert.ok(micromark("$x^2$", {
  extensions: [math()], htmlExtensions: [mathHtml()],
}).includes('class="katex"'));
`
	call := exec.CommandContext(ctx, "node", "--input-type=module", "-e", code)
	call.Dir = repositoryRoot(t)
	if output, err := call.CombinedOutput(); err != nil {
		t.Fatalf("native math security and compatibility: %v\n%s", err, output)
	}
}

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

func TestDependencyAdmissionRunsNativeScannerInTheActualOwner(t *testing.T) {
	root := t.TempDir()
	policy := filepath.Join(root, ".config", "checks", "dependencies", "policy.toml")
	if err := os.MkdirAll(filepath.Dir(policy), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policy, []byte("IgnoredVulns = []\n[[PackageOverrides]]\nname='example.invalid/dependency'\nversion='1.0.0'\necosystem='Go'\nlicense.override=['MIT']\n[[PackageOverrides]]\nname='fixture'\nversion='1.0.0'\necosystem='npm'\nlicense.override=['MIT']\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for path, value := range map[string]string{
		"go.mod":            "module example.invalid/fixture\ngo 1.27.1\nrequire example.invalid/dependency v1.0.0\n",
		"package-lock.json": `{"name":"fixture","lockfileVersion":3,"packages":{"node_modules/fixture":{"version":"1.0.0","dev":true}}}`,
	} {
		if err := os.WriteFile(filepath.Join(root, path), []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cache := filepath.Join(root, "cache")
	for _, ecosystem := range []string{"Go", "npm"} {
		path := filepath.Join(cache, "osv-scalibr", ecosystem, "all.zip")
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		var data bytes.Buffer
		if err := zip.NewWriter(&data).Close(); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("OSV_SCALIBR_LOCAL_DB_CACHE_DIRECTORY", cache)
	output := filepath.Join(root, "evidence")
	if err := construction.ScanDependencies(t.Context(), root, output); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(output, "scan-*", "dependencies.*.json"))
	if err != nil || len(files) != 4 {
		t.Fatalf("native raw/decision evidence incomplete: %v %v", files, err)
	}
}
