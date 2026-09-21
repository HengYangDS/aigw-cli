import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import fs from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const root = path.resolve(import.meta.dirname, "../../..");
const integration = path.join(root, "integrations/architecture-publisher");

async function json(file) {
  return JSON.parse(await fs.readFile(file, "utf8"));
}

async function filesBelow(directory, prefix = "") {
  const files = [];
  for (const entry of await fs.readdir(directory, { withFileTypes: true })) {
    const relative = prefix ? `${prefix}/${entry.name}` : entry.name;
    const file = path.join(directory, entry.name);
    assert.equal(entry.isSymbolicLink(), false, relative);
    if (entry.isDirectory()) files.push(...(await filesBelow(file, relative)));
    else if (entry.isFile()) files.push(relative);
    else assert.fail(`unsupported integration member: ${relative}`);
  }
  return files.sort();
}

test("AIGW owns a narrow optional Architecture Publisher integration", async () => {
  const manifest = await json(path.join(integration, "package.json"));
  assert.equal(manifest.name, "@architecture-publisher/aigw");
  assert.equal(manifest.version, "0.2.0-alpha.0");
  assert.equal(manifest.type, "module");
  assert.deepEqual(manifest.exports, {
    ".": "./src/index.mjs",
    "./runtime": "./src/runtime.mjs",
  });
  assert.deepEqual(manifest.files, ["authoring/", "src/", "README.md"]);
  assert.deepEqual(manifest.dependencies, {
    "architecture-publisher": "0.2.0-alpha.0",
  });
  assert.equal(Object.hasOwn(manifest, "bin"), false);
});

test("authoring binds current AIGW concepts to explicit tracked sources", async () => {
  const model = await json(path.join(integration, "authoring/model.json"));
  const edition = await json(path.join(integration, "authoring/edition.json"));
  assert.equal(model.schema, "aigw.architecture-authoring/v1");
  assert.equal(model.owner, "aigw-cli");
  assert.equal(edition.schema, "aigw.architecture-edition-authoring/v1");
  assert.equal(edition.id, "aigw-control-plane");

  const sourceIds = new Set(Object.keys(model.sources));
  const selectedPaths = Object.values(model.sources).map(
    ({ path: file }) => file,
  );
  assert.equal(new Set(selectedPaths).size, selectedPaths.length);
  for (const [id, source] of Object.entries(model.sources)) {
    assert.equal(typeof source.authority, "string", id);
    assert(source.authority.trim(), id);
    assert.equal(path.isAbsolute(source.path), false, id);
    assert.doesNotMatch(source.path, /(?:^|\/)\.\.(?:\/|$)|\\/);
    const stat = await fs.lstat(path.join(root, source.path));
    assert(stat.isFile() && !stat.isSymbolicLink(), source.path);
    execFileSync("git", ["ls-files", "--error-unmatch", source.path], {
      cwd: root,
      stdio: "ignore",
    });
  }

  for (const values of [model.entities, model.relations, model.claims])
    assert.equal(new Set(Object.keys(values)).size, Object.keys(values).length);
  for (const value of [
    ...Object.values(model.entities),
    ...Object.values(model.relations),
    ...Object.values(model.claims),
  ]) {
    assert(Array.isArray(value.provenance) && value.provenance.length > 0);
    assert(value.provenance.every((id) => sourceIds.has(id)));
  }

  const claims = new Set(Object.keys(model.claims));
  assert.deepEqual(new Set(edition.requiredClaims), claims);
  for (const scene of [
    edition.scenes.static,
    ...Object.values(edition.scenes.interactive),
  ]) {
    assert(
      scene.claims.every((id) => claims.has(id)),
      scene.label,
    );
  }
  assert.notDeepEqual(
    edition.scenes.static.composition,
    edition.scenes.interactive.overview.composition,
  );
});

test("runtime consumes explicit Publisher contracts without repository discovery", async () => {
  const files = await filesBelow(path.join(integration, "src"));
  const source = (
    await Promise.all(
      files
        .filter((file) => file.endsWith(".mjs"))
        .map((file) =>
          fs.readFile(path.join(integration, "src", file), "utf8"),
        ),
    )
  ).join("\n");
  assert.match(
    source,
    /from "architecture-publisher\/(?:source|semantics|edition)"/,
  );
  assert.doesNotMatch(
    source,
    /node:child_process|process\.env|(?:^|["'`])\.git(?:["'`/])|\/Users\/|[a-f0-9]{40}/,
  );
  assert.doesNotMatch(
    source,
    /internal\/(?:cli|client|configuration|synchronization)/,
  );
  assert.doesNotMatch(source, /register|plugin|discover/i);
});
