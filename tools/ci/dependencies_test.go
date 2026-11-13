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
  await readFile(".config/checks/renovate/renovate.json5", "utf8"), {},
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

// renovatePolicy is a Node module prelude that binds the parsed Renovate policy
// to policy as plain values.
const renovatePolicy = `
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { parsers } from "prettier/plugins/babel";
const value = (node) => {
  switch (node.type) {
    case "ObjectExpression":
      return Object.fromEntries(
        node.properties.map(({ key, value: item }) => [key.name ?? key.value, value(item)]),
      );
    case "ArrayExpression":
      return node.elements.map(value);
    case "NullLiteral":
      return null;
    default:
      return node.value;
  }
};
const policy = value((await parsers.json5.parse(
  await readFile(".config/checks/renovate/renovate.json5", "utf8"), {},
)).node);
`

func TestRenovateReadsEveryNativeClientPin(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	root := repositoryRoot(t)
	export := exec.CommandContext(ctx, "cue", "export", ".config/ci/pipeline.cue", "--expression", "nativeClients", "--out", "json")
	export.Dir = root
	pins, err := export.Output()
	if err != nil {
		t.Fatalf("native client pins: %v", err)
	}
	// The candidate calculation applies the policy's regex rules to the CUE
	// authority; apply them the same way and require one read of each pin.
	code := renovatePolicy + `
const authority = ".config/ci/pipeline.cue";
const text = await readFile(authority, "utf8");
const pins = JSON.parse(process.argv[1]);
const reads = [];
for (const rule of policy.customManagers) {
  const files = (rule.managerFilePatterns ?? []).map((pattern) => new RegExp(pattern.slice(1, -1)));
  if (!files.some((file) => file.test(authority))) continue;
  for (const pattern of rule.matchStrings) {
    for (const { groups = {} } of text.matchAll(new RegExp(pattern, "g"))) {
      reads.push({
        depName: groups.depName ?? rule.depNameTemplate,
        datasource: groups.datasource ?? rule.datasourceTemplate,
        currentValue: groups.currentValue,
        currentDigest: groups.currentDigest,
      });
    }
  }
}
const expected = {
  codex: { depName: "@openai/codex", datasource: "npm", currentValue: pins.codex },
  claude: { depName: "@anthropic-ai/claude-code", datasource: "npm", currentValue: pins.claude },
  hermes: { depName: "NousResearch/hermes-agent", datasource: "github-tags", currentDigest: pins.hermes?.commit },
};
assert.deepEqual(Object.keys(pins).sort(), Object.keys(expected).sort(), "every native client pin needs a candidate rule");
for (const pin of Object.values(expected)) {
  const found = reads.filter(({ depName }) => depName === pin.depName);
  assert.equal(found.length, 1, pin.depName + " must be read by exactly one candidate rule");
  for (const [field, want] of Object.entries(pin)) {
    assert.equal(found[0][field], want, pin.depName + " " + field);
  }
}
const hermes = reads.find(({ depName }) => depName === "NousResearch/hermes-agent");
assert.match(hermes.currentValue, /^v\d+\.\d+\.\d+$/, "the Hermes commit must name its release tag");
`
	call := exec.CommandContext(ctx, "node", "--input-type=module", "-e", code, string(pins))
	call.Dir = root
	if output, err := call.CombinedOutput(); err != nil {
		t.Fatalf("Renovate native client pins: %v\n%s", err, output)
	}
}

func TestRenovateReadsOnlyReleaseTagsOfHermes(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	// Hermes tags canary builds as vX.Y.Z+canary.<time>. Semver coercion drops
	// that build metadata, so on 2026-11-13 the candidate calculation offered a
	// canary tag as the release v0.23.1. The rule must parse release tags only,
	// which also excludes the publisher's calendar tags.
	code := renovatePolicy + `
const rule = policy.customManagers.find(
  ({ depNameTemplate }) => depNameTemplate === "NousResearch/hermes-agent",
);
assert.ok(rule, "the policy must read the Hermes release tag");
const versioning = rule.versioningTemplate ?? "";
assert.ok(versioning.startsWith("regex:"), "Hermes tags must parse as release tags only, not with " + versioning);
const release = new RegExp(versioning.slice("regex:".length));
for (const tag of ["v0.23.0", "v0.23.1", "v1.0.0"]) {
  assert.match(tag, release, tag);
}
for (const tag of ["v0.23.1+canary.20261110T064422Z", "v0.24.0-rc.1", "0.23.1", "v0.23", "v2026.9.24"]) {
  assert.doesNotMatch(tag, release, tag);
}
`
	call := exec.CommandContext(ctx, "node", "--input-type=module", "-e", code)
	call.Dir = repositoryRoot(t)
	if output, err := call.CombinedOutput(); err != nil {
		t.Fatalf("Renovate Hermes release tags: %v\n%s", err, output)
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

func TestRenovateProposesTransitiveLockRefreshes(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	// Direct updates alone leave transitive releases in the lock unreported, so
	// the candidate calculation must also propose lock maintenance, and that
	// proposal must keep the repository's release age.
	code := `
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { parsers } from "prettier/plugins/babel";
const { node } = await parsers.json5.parse(
  await readFile(".config/checks/renovate/renovate.json5", "utf8"), {},
);
const field = (object, name) =>
  object.properties.find(({ key }) => (key.name ?? key.value) === name)?.value;
const maintenance = field(node, "lockFileMaintenance");
assert.ok(maintenance, "the policy must configure lock maintenance");
assert.equal(field(maintenance, "enabled")?.value, true, "lock maintenance must be enabled");
assert.equal(field(maintenance, "minimumReleaseAge"), undefined, "lock maintenance must keep the policy's release age");
`
	call := exec.CommandContext(ctx, "node", "--input-type=module", "-e", code)
	call.Dir = repositoryRoot(t)
	if output, err := call.CombinedOutput(); err != nil {
		t.Fatalf("Renovate lock maintenance: %v\n%s", err, output)
	}
}

func TestRenovateHoldsEveryReleaseForADay(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	// The owner's floor of 2026-11-13: no release is taken before it is one day
	// old, npm keeps three days, and only a security fix skips the age. Every
	// age in the policy is read, so a new rule below one day fails here.
	code := `
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { parsers } from "prettier/plugins/babel";
const { node } = await parsers.json5.parse(
  await readFile(".config/checks/renovate/renovate.json5", "utf8"), {},
);
const ages = [];
const walk = (value, path) => {
  if (value?.type === "ObjectExpression") {
    for (const { key, value: inner } of value.properties) {
      const name = key.name ?? key.value;
      if (name === "minimumReleaseAge") ages.push(path + "=" + inner.value);
      else if (name === "minimumReleaseAgeBehaviour") ages.push(path + "!" + inner.value);
      else walk(inner, path + "/" + name);
    }
  } else if (value?.type === "ArrayExpression") {
    for (const inner of value.elements) walk(inner, path + "[]");
  }
};
walk(node, "");
assert.deepEqual(ages.sort(), [
  "!timestamp-required",
  "/packageRules[]=1 day",
  "/packageRules[]=1 day",
  "/vulnerabilityAlerts=0 days",
  "=3 days",
]);
`
	call := exec.CommandContext(ctx, "node", "--input-type=module", "-e", code)
	call.Dir = repositoryRoot(t)
	if output, err := call.CombinedOutput(); err != nil {
		t.Fatalf("Renovate release ages: %v\n%s", err, output)
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
