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

function run(root, module, files, input = JSON.stringify(files)) {
  const result = spawnSync(
    process.execPath,
    [path.join(repository, "tools/ci", module)],
    { cwd: root, input, encoding: "utf8", timeout: 30_000 },
  );
  assert.equal(result.error, undefined);
  return { status: result.status, output: result.stdout + result.stderr };
}

test("formatting checks exact authored inputs without writing or ambient config", async (t) => {
  const root = await fixture(t);
  await fs.writeFile(path.join(root, ".prettierrc.json"), '{"tabWidth":7}\n');
  const current = path.join(root, "document.md");
  const ignored = path.join(root, "openspec/changes/archive/old/spec.md");
  const unsupported = path.join(root, "unsupported.txt");
  await fs.mkdir(path.dirname(ignored), { recursive: true });
  await fs.writeFile(ignored, "#   Historical source\n");
  await fs.writeFile(unsupported, "not a formatter input");
  for (const [content, valid] of [
    ["# Document\n\nRead the current instructions.\n", true],
    ["#   Document\n", false],
  ]) {
    await t.test(`valid=${valid}`, async () => {
      await fs.writeFile(current, content);
      const result = run(root, "format.mjs", [current, ignored, unsupported]);
      assert.equal(result.status === 0, valid, result.output);
      if (!valid)
        assert.ok(result.output.includes("document.md"), result.output);
      assert.equal(await fs.readFile(current, "utf8"), content);
    });
  }
  const empty = run(root, "format.mjs", [ignored, unsupported]);
  assert.notEqual(empty.status, 0);
  assert.ok(empty.output.includes("no authored files supported by Prettier"));
  assert.equal(await fs.readFile(ignored, "utf8"), "#   Historical source\n");
});

test("Markdown uses declared rules and refuses inline suppression without writing", async (t) => {
  const root = await fixture(t);
  const file = path.join(root, "document.md");
  await fs.writeFile(
    path.join(root, ".markdownlint.json"),
    '{"default":false}',
  );
  for (const [content, valid] of [
    ["# Document\n\n## Details\n\nRead the selected rules.\n", true],
    ["<!-- markdownlint-disable MD001 -->\n# Document\n\n### Details\n", false],
  ]) {
    await t.test(`valid=${valid}`, async () => {
      await fs.writeFile(file, content);
      const result = run(root, "markdown/lint.mjs", [file]);
      assert.equal(result.status === 0, valid, result.output);
      if (!valid) assert.ok(result.output.includes("MD001"), result.output);
      assert.equal(await fs.readFile(file, "utf8"), content);
    });
  }
  await fs.unlink(path.join(root, ".config/checks/markdown/policy.yaml"));
  assert.notEqual(run(root, "markdown/lint.mjs", [file]).status, 0);
});

test("Mermaid validates only requested source and retains invalid input", async (t) => {
  const root = await fixture(t);
  const files = [];
  for (const [name, diagram] of [
    ["current", "flowchart LR\n  A[Source] --> B[Target]\n"],
    ["invalid", "flowchart LR\n  A[Unclosed\n"],
  ]) {
    const file = path.join(root, `${name}.md`);
    const content = `# Diagram\n\n\`\`\`mermaid\n${diagram}\`\`\`\n`;
    await fs.writeFile(file, content);
    files.push({ file, content });
  }
  const accepted = run(root, "markdown/diagrams.mjs", [files[0].file]);
  assert.equal(accepted.status, 0, accepted.output);
  const rejected = run(root, "markdown/diagrams.mjs", [files[1].file]);
  assert.equal(rejected.status, 1, rejected.output);
  assert.ok(rejected.output.includes("invalid.md:"), rejected.output);
  for (const { file, content } of files)
    assert.equal(await fs.readFile(file, "utf8"), content);
});

test("native text checks consume the complete pipe and preserve source bytes", async (t) => {
  const root = await fixture(t);
  const file = path.join(root, "document.md");
  const content = "# Document\n\n```mermaid\nflowchart LR\n  A --> B\n```\n";
  await fs.writeFile(file, content);
  const encoded = JSON.stringify([file]);
  const input = encoded.slice(0, 1) + " \n".repeat(1 << 17) + encoded.slice(1);
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
