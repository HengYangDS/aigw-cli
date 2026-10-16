package main

import (
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

func TestRenovateInputsResolveToRepositoryOwners(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	code := `
import assert from "node:assert/strict";
import { readFile, stat } from "node:fs/promises";
import { parsers } from "prettier/plugins/babel";
const policy = await parsers.json5.parse(
  await readFile(".config/dependencies/renovate.json5", "utf8"), {},
);
const inputs = policy.node.properties.find(
  ({ key }) => (key.name ?? key.value) === "includePaths",
).value.elements;
assert.ok(inputs.length > 0, "Renovate must declare dependency owners");
for (const input of inputs) {
  assert.equal(input.type, "StringLiteral");
  assert.ok((await stat(input.value)).isFile(), input.value);
}
`
	call := exec.CommandContext(ctx, "node", "--input-type=module", "-e", code)
	call.Dir = repositoryRoot(t)
	if output, err := call.CombinedOutput(); err != nil {
		t.Fatalf("Renovate dependency ownership: %v\n%s", err, output)
	}
}

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
