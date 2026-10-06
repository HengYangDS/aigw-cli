package main

import (
	"aigw-cli/tools/release/construction"
	"bytes"
	"context"
	"encoding/json"
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
import { pathToFileURL } from "node:url";
for (const consumer of ["mermaid", "micromark-extension-math"]) {
  const require = createRequire(import.meta.resolve(consumer));
  const { default: katex } = await import(pathToFileURL(require.resolve("katex")).href);
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

func TestNativeDependencyEvidenceRetainsFindings(t *testing.T) {
	root := t.TempDir()
	policy := filepath.Join(root, ".config", "checks", "dependencies", "policy.toml")
	if err := os.MkdirAll(filepath.Dir(policy), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policy, []byte("[[PackageOverrides]]\nname='example.invalid/dependency'\nversion='1.0.0'\necosystem='Go'\nlicense.override=['MIT']\n[[PackageOverrides]]\nname='braces'\nversion='3.0.3'\necosystem='npm'\nlicense.override=['MIT']\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for path, value := range map[string]string{
		"go.mod":            "module example.invalid/fixture\ngo 1.27.1\nrequire example.invalid/dependency v1.0.0\n",
		"package-lock.json": `{"name":"fixture","lockfileVersion":3,"packages":{"node_modules/braces":{"version":"3.0.3","dev":true}}}`,
	} {
		if err := os.WriteFile(filepath.Join(root, path), []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	output := filepath.Join(root, "evidence")
	if err := construction.ScanDependencies(t.Context(), root, output); err != nil {
		t.Fatal(err)
	}
	scans, err := filepath.Glob(filepath.Join(output, "scan-*"))
	if err != nil || len(scans) != 1 {
		t.Fatalf("expected one native scan: %v %v", scans, err)
	}
	files, err := filepath.Glob(filepath.Join(scans[0], "dependencies.*.json"))
	if err != nil || len(files) != 2 {
		t.Fatalf("native raw/exit evidence incomplete: %v %v", files, err)
	}
	for _, name := range []string{"dependencies.raw.json", "vulnerabilities.json", "licenses.json"} {
		content, err := os.ReadFile(filepath.Join(scans[0], name))
		if err != nil || !json.Valid(content) {
			t.Fatalf("native evidence %s is unavailable or invalid: %v", name, err)
		}
		if name != "licenses.json" {
			if !bytes.Contains(content, []byte("GHSA-vfj7-8cjw-p6xm")) {
				t.Fatalf("native evidence %s lost the selected advisory", name)
			}
		} else {
			for _, value := range []string{"go.mod", "package-lock.json", "example.invalid/dependency", "braces", "MIT"} {
				encoded, err := json.Marshal(value)
				if err != nil || !bytes.Contains(content, encoded) {
					t.Fatalf("native license evidence lost %s: %v", value, err)
				}
			}
		}
	}
	content, err := os.ReadFile(filepath.Join(scans[0], "dependencies.raw.json.exit.json"))
	if err != nil {
		t.Fatal(err)
	}
	var exit struct {
		Exit     int  `json:"exit"`
		Failed   bool `json:"failed"`
		Findings bool `json:"findings"`
	}
	if err := json.Unmarshal(content, &exit); err != nil || exit.Exit != 1 || exit.Failed || !exit.Findings {
		t.Fatalf("native findings must be retained and nonblocking: %+v %v", exit, err)
	}
}
