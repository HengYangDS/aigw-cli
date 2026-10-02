import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";

const repository = path.resolve(import.meta.dirname, "../../..");

async function fixture(t) {
  const root = await fs.realpath(
    await fs.mkdtemp(path.join(os.tmpdir(), "aigw-text-checks-")),
  );
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  for (const relative of [
    ".prettierignore",
    ".config/checks/markdown/policy.yaml",
  ]) {
    const destination = path.join(root, relative);
    await fs.mkdir(path.dirname(destination), { recursive: true });
    await fs.copyFile(path.join(repository, relative), destination);
  }
  return root;
}

function run(
  root,
  module,
  files,
  input = JSON.stringify(files),
  timeout = 15_000,
) {
  const result = spawnSync(
    process.execPath,
    [
      path.isAbsolute(module)
        ? module
        : path.join(repository, "tools/ci", module),
    ],
    { cwd: root, input, encoding: "utf8", timeout },
  );
  assert.equal(result.error, undefined);
  return { status: result.status, output: result.stdout + result.stderr };
}

test("formatting checks current carriers without editing or inherited defaults", async (t) => {
  const root = await fixture(t);
  const files = {
    "document.md": "# Current\n\nRead the current instructions.\n",
    "docs/archive/current.md": "# Current archive operation\n",
    ".config/archive/metadata.json": '{\n  "version": 1\n}\n',
    ".config/fixture.yaml": "name: fixture\n",
    "openspec/changes/archive/old/spec.md": "#   Historic source\n",
    "build/tracked.json": '{\n  "version": 1\n}\n',
    "literal[1].json": '{\n  "version": 1\n}\n',
    "unsupported.txt": "not a prettier file",
  };
  for (const [relative, content] of Object.entries(files)) {
    const destination = path.join(root, relative);
    await fs.mkdir(path.dirname(destination), { recursive: true });
    await fs.writeFile(destination, content);
  }
  await fs.writeFile(path.join(root, ".prettierrc.json"), '{"tabWidth":7}\n');
  for (const [relative, content, valid] of [
    ["document.md", files["document.md"], true],
    ["document.md", "#   Current\n", false],
    ["docs/archive/current.md", "#   Current archive operation\n", false],
    [".config/archive/metadata.json", '{"version":1}\n', false],
    [".config/fixture.yaml", "name:    fixture\n", false],
    ["build/tracked.json", '{"version":1}\n', false],
    ["literal[1].json", '{"version":1}\n', false],
  ]) {
    await t.test(`${relative}: valid=${valid}`, async () => {
      const destination = path.join(root, relative);
      await fs.writeFile(destination, content);
      const result = run(root, "format.mjs", Object.keys(files));
      assert.equal(result.status === 0, valid, result.output);
      if (!valid) assert.ok(result.output.includes(relative), result.output);
      assert.equal(await fs.readFile(destination, "utf8"), content);
      await fs.writeFile(destination, files[relative]);
    });
  }
});

test("formatting requires declared inputs, policy and dependencies", async (t) => {
  for (const [name, files, diagnostic] of [
    ["empty inventory", [], "no authored files supported by Prettier"],
    [
      "unsupported source",
      ["source.txt"],
      "no authored files supported by Prettier",
    ],
    ["missing ignore", ["source.txt"], "ENOENT"],
    ["missing dependency", ["source.txt"], "ERR_MODULE_NOT_FOUND"],
  ]) {
    await t.test(name, async (t) => {
      const root = await fixture(t);
      const content = "unsupported authored source\n";
      await fs.writeFile(path.join(root, "source.txt"), content);
      if (name === "missing ignore")
        await fs.rm(path.join(root, ".prettierignore"));
      let module = "format.mjs";
      if (name === "missing dependency") {
        module = path.join(root, "tools/ci/format.mjs");
        await fs.mkdir(path.dirname(module), { recursive: true });
        await fs.copyFile(path.join(repository, "tools/ci/format.mjs"), module);
      }
      const result = run(root, module, files);
      assert.notEqual(result.status, 0, result.output);
      assert.ok(result.output.includes(diagnostic), result.output);
      assert.equal(
        await fs.readFile(path.join(root, "source.txt"), "utf8"),
        content,
      );
    });
  }
});

test("Markdown enforces native document structure without inline suppression", async (t) => {
  const root = await fixture(t);
  for (const [name, content, rule] of [
    ["document", "# Document\n\n## First\n\n```sh\naigw status\n```\n", ""],
    [
      "OpenSpec",
      "## ADDED Requirements\n\n### Requirement: First\n\n#### Scenario: Accepted\n\n- Expected.\n\n### Requirement: Second\n\n#### Scenario: Accepted\n\n- Expected.\n",
      "",
    ],
    ["heading progression", "# Document\n\n### Details\n", "MD001"],
    [
      "inline suppression",
      "<!-- markdownlint-disable MD001 -->\n# Document\n\n### Details\n",
      "MD001",
    ],
    [
      "sibling uniqueness",
      "# Document\n\n## Details\n\nText.\n\n## Details\n",
      "MD024",
    ],
    ["single title", "# First\n\nText.\n\n# Second\n", "MD025"],
    ["heading punctuation", "# Document\n\n## Details:\n", "MD026"],
    ["question heading", "# Document\n\n## Which backend should I use?\n", ""],
    ["semantic heading", "# Document\n\n**Details**\n\nText.\n", "MD036"],
    [
      "emphasized sentence",
      "# Document\n\n**Keep the original configuration.**\n",
      "",
    ],
    [
      "OpenSpec labels",
      "## Goals / Non-Goals\n\n**Goals:**\n\n- Preserve ownership.\n\n**Non-Goals:**\n\n- Change client state.\n",
      "",
    ],
    ["fence language", "# Document\n\n```\naigw status\n```\n", "MD040"],
    ["repeated blank lines", "# Document\n\n\nText.\n", "MD012"],
    ["missing heading separator", "# Document\nText.\n", "MD022"],
    [
      "missing fence separator",
      "# Document\n\nText.\n```sh\naigw status\n```\n",
      "MD031",
    ],
    ["missing list separator", "# Document\n\nText.\n- Item.\n", "MD032"],
    [
      "tight wrapped tasks",
      "# Tasks\n\n- [ ] First action\n      and its verification.\n- [ ] Second action.\n",
      "",
    ],
    [
      "isolated task separator",
      "# Tasks\n\n- [ ] First.\n- [ ] Second.\n\n- [ ] Third.\n",
      "single-paragraph-list-spacing",
    ],
    [
      "loose single-paragraph list",
      "# Document\n\n1. First.\n\n2. Second.\n",
      "single-paragraph-list-spacing",
    ],
    [
      "nested single-paragraph list",
      "# Document\n\n- Parent.\n  - First.\n\n  - Second.\n- Next parent.\n",
      "single-paragraph-list-spacing",
    ],
    [
      "quoted single-paragraph list",
      "# Document\n\n> - First.\n>\n> - Second.\n",
      "single-paragraph-list-spacing",
    ],
    [
      "multi-paragraph list",
      "# Document\n\n- First paragraph.\n\n  Second paragraph.\n\n- Next item.\n",
      "",
    ],
    [
      "list with fenced example",
      "# Document\n\n- Command.\n\n  ```sh\n  aigw status\n\n\n  aigw check\n  ```\n\n- Next item.\n",
      "",
    ],
    [
      "list-looking code",
      "# Document\n\n```markdown\n- First.\n\n- Second.\n```\n",
      "",
    ],
    [
      "independent lists",
      "# Document\n\n- First.\n\nParagraph.\n\n- Second.\n",
      "",
    ],
    [
      "blank-line rule cannot be disabled inline",
      "# Tasks\n\n<!-- markdownlint-disable blank_lines -->\n- [ ] First.\n\n- [ ] Second.\n",
      "single-paragraph-list-spacing",
    ],
    [
      "missing table separator",
      "# Document\n\nText.\n| Field |\n| ----- |\n| Value |\n",
      "MD058",
    ],
    ["separator style", "# Document\n\nText.\n\n***\n\nText.\n", "MD035"],
    [
      "table columns",
      "# Document\n\n| First | Second |\n| ----- | ------ |\n| Value |\n",
      "MD056",
    ],
    [
      "table alignment",
      "# Document\n\n| First | Second |\n| ----- | ------ |\n| Value  | Result |\n",
      "MD060",
    ],
  ]) {
    await t.test(name, async () => {
      const file = path.join(root, "document.md");
      await fs.writeFile(file, content);
      const result = run(root, "markdown/lint.mjs", [file]);
      assert.equal(result.status === 0, rule === "", result.output);
      assert.ok(
        result.output.includes(rule || "checked 1 Markdown files"),
        result.output,
      );
      assert.equal(await fs.readFile(file, "utf8"), content);
    });
  }
});

test("Markdown rejects warnings and missing native inputs", async (t) => {
  for (const [name, policy, diagnostic] of [
    ["warning", "MD001: warning\n", "MD001"],
    ["missing policy", "", "ENOENT"],
    ["missing dependency", "MD001: true\n", "ENOENT"],
  ]) {
    await t.test(name, async (t) => {
      const root = await fixture(t);
      const policyPath = path.join(root, ".config/checks/markdown/policy.yaml");
      if (policy) await fs.writeFile(policyPath, policy);
      else await fs.rm(policyPath);
      const file = path.join(root, "document.md");
      const content = "# Document\n\n### Details\n";
      await fs.writeFile(file, content);
      let module = "markdown/lint.mjs";
      if (name === "missing dependency") {
        module = path.join(root, "tools/ci/markdown/lint.mjs");
        await fs.mkdir(path.dirname(module), { recursive: true });
        await fs.copyFile(
          path.join(repository, "tools/ci/markdown/lint.mjs"),
          module,
        );
      }
      const result = run(root, module, [file]);
      assert.notEqual(result.status, 0, result.output);
      assert.ok(result.output.includes(diagnostic), result.output);
      assert.equal(await fs.readFile(file, "utf8"), content);
    });
  }
});

test("Mermaid validates requested diagrams without unrelated config overrides", async (t) => {
  const root = await fixture(t);
  await fs.writeFile(
    path.join(root, ".mermaidlintrc.json"),
    '{"ignore":["**/*"],"rules":{"no-self-loop":"off"}}',
  );
  const diagrams = [
    ["flowchart", "flowchart LR\n  A[Source] --> B[Target]\n", true],
    ["sequence", "sequenceDiagram\n  A->>B: Failure, no writes\n", true],
    [
      "semicolon message",
      "sequenceDiagram\n  A->>B: Failure; no writes\n",
      false,
    ],
    ["syntax", "flowchart LR\n  A[Unclosed\n", false],
  ];
  const files = [];
  for (const [name, diagram] of diagrams) {
    const file = path.join(root, `${name}.md`);
    await fs.writeFile(file, `# Diagram\n\n\`\`\`mermaid\n${diagram}\`\`\`\n`);
    files.push(file);
  }
  const accepted = run(root, "markdown/diagrams.mjs", files.slice(0, 2));
  assert.equal(accepted.status, 0, accepted.output);
  assert.ok(
    accepted.output.includes("checked 2 diagrams in 2 files"),
    accepted.output,
  );
  // Batch invalid input loads the authoritative fallback parser only once.
  const rejected = run(
    root,
    "markdown/diagrams.mjs",
    files.slice(2),
    JSON.stringify(files.slice(2)),
    60_000,
  );
  assert.equal(rejected.status, 1, rejected.output);
  assert.ok(
    rejected.output.includes("checked 2 diagrams in 2 files"),
    rejected.output,
  );
  for (const [index, [name, diagram, valid]] of diagrams.entries()) {
    await t.test(name, async () => {
      const result = valid ? accepted : rejected;
      assert.equal(
        result.output.includes(`${files[index]}:`),
        !valid,
        result.output,
      );
      assert.equal(
        await fs.readFile(files[index], "utf8"),
        `# Diagram\n\n\`\`\`mermaid\n${diagram}\`\`\`\n`,
      );
    });
  }
  const missing = run(root, path.join(root, "absent-checker.mjs"), []);
  assert.notEqual(missing.status, 0, missing.output);
  assert.ok(missing.output.includes("MODULE_NOT_FOUND"), missing.output);
});

test("native text checkers consume the complete pipe without changing source", async (t) => {
  const root = await fixture(t);
  const file = path.join(root, "document.md");
  const content = "# Document\n\n```mermaid\nflowchart LR\n  A --> B\n```\n";
  await fs.writeFile(file, content);
  const encoded = JSON.stringify([file]);
  const input = encoded.slice(0, 1) + " \n".repeat(1 << 19) + encoded.slice(1);
  for (const module of [
    "format.mjs",
    "markdown/lint.mjs",
    "markdown/diagrams.mjs",
  ]) {
    await t.test(module, async () => {
      const result = run(root, module, [file], input);
      assert.equal(result.status, 0, result.output);
      assert.ok(result.output.includes("checked 1"), result.output);
      assert.equal(await fs.readFile(file, "utf8"), content);
    });
  }
});
