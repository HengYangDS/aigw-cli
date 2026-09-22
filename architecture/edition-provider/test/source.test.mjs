import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import fs from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const providerRoot = path.resolve(import.meta.dirname, "..");
const repositoryRoot = path.resolve(providerRoot, "../..");
const sha256 = (bytes) => createHash("sha256").update(bytes).digest("hex");

async function json(relative) {
  return JSON.parse(
    await fs.readFile(path.join(providerRoot, relative), "utf8"),
  );
}

async function regularBytes(relative) {
  const file = path.join(providerRoot, ...relative.split("/"));
  const stat = await fs.lstat(file);
  assert.equal(stat.isFile(), true, relative);
  assert.equal(stat.isSymbolicLink(), false, relative);
  return fs.readFile(file);
}

test("AIGW owns one closed declarative Edition Provider", async () => {
  const [provider, selection, source] = await Promise.all([
    json("provider.json"),
    json("selection.json"),
    json("_source/manifest.json"),
  ]);

  assert.equal(provider.schema, "architecture.edition-provider/v1");
  assert.equal(provider.delivery.kind, "declarative");
  assert.equal(provider.provider.id, "aigw-client-projection-edition");
  assert.equal(
    provider.subject.id,
    "dig/misc/tools/llm-third-party-api/aigw-cli",
  );
  assert.deepEqual(provider.subject, source.source);
  assert.equal(selection.schema, "aigw.architecture-publisher-selection/v1");
  assert.equal(selection.publisher.name, "architecture-publisher");
  assert.equal(selection.publisher.version, "0.3.0-alpha.1");
  assert.equal(
    selection.providerManifestSha256,
    sha256(await regularBytes("provider.json")),
  );
  assert.equal(
    provider.delivery.sourceManifestSha256,
    sha256(await regularBytes(provider.delivery.sourceManifest)),
  );
  assert.equal(
    provider.delivery.editionSha256,
    sha256(await regularBytes(provider.delivery.edition)),
  );
  assert.equal(
    provider.delivery.evolutionSha256,
    sha256(await regularBytes(provider.delivery.evolution)),
  );
  assert.equal(
    provider.expected.sourceManifestSha256,
    provider.delivery.sourceManifestSha256,
  );
  assert.equal(
    provider.expected.editionDigest,
    selection.migrationBaseline.candidate.editionDigest,
  );
  assert.equal(
    provider.expected.evolutionSha256,
    selection.migrationBaseline.providerEvolutionSha256,
  );
});

test("the checked-in Source Bundle closes exact AIGW provenance", async () => {
  const [provider, selection, source, semantic] = await Promise.all([
    json("provider.json"),
    json("selection.json"),
    json("_source/manifest.json"),
    json("_source/semantic.json"),
  ]);
  const paths = source.files.map(({ path: relative }) => relative);
  assert.equal(new Set(paths).size, paths.length);
  assert.equal(paths.includes(provider.delivery.semanticMember), true);

  for (const member of source.files) {
    const bytes = await regularBytes(`_source/${member.path}`);
    assert.equal(bytes.length, member.bytes, member.path);
    assert.equal(sha256(bytes), member.sha256, member.path);
  }
  for (const [id, provenance] of Object.entries(semantic.sources)) {
    const member = source.files.find(
      ({ path: relative }) => relative === provenance.locator,
    );
    assert(member, id);
    assert.equal(member.sha256, provenance.sha256, id);
  }
  assert.equal(
    sha256(await regularBytes("_source/semantic.json")),
    selection.migrationBaseline.files["claim-model.json"],
  );
  const comparison = await json("evolution.json");
  const providerEvolution = await json("provider-evolution.json");
  const selected = async ({ path: relative, sha256: expected }) => {
    const normalized =
      relative === "claim-model.json" ? "_source/semantic.json" : relative;
    const bytes = await regularBytes(normalized);
    assert.equal(sha256(bytes), expected, normalized);
    return JSON.parse(bytes);
  };
  assert.deepEqual(providerEvolution, {
    before: {
      claimModel: await selected(comparison.before.claimModel),
      editions: await Promise.all(comparison.before.editions.map(selected)),
    },
    after: {
      claimModel: await selected(comparison.after.claimModel),
      editions: await Promise.all(comparison.after.editions.map(selected)),
    },
    correspondences: comparison.correspondences,
  });
});

test("the provider has one source owner and no executable integration", async () => {
  const selection = await json("selection.json");
  const expectedFiles = [
    "README.md",
    "edition.json",
    "evolution.json",
    "provider.json",
    "provider-evolution.json",
    "selection.json",
  ];
  for (const relative of expectedFiles) await regularBytes(relative);
  assert.equal(selection.authority.subject, "AIGW");
  assert.equal(selection.authority.authorizationOwner, "AIGW");
  assert.equal(
    selection.authority.generatedArtifacts,
    "untracked-reproducible-output",
  );
  await assert.rejects(
    fs.stat(path.join(repositoryRoot, "integrations/architecture-publisher")),
    {
      code: "ENOENT",
    },
  );
  await assert.rejects(
    fs.stat(path.join(repositoryRoot, "docs/architecture/client-projection")),
    { code: "ENOENT" },
  );
  const portable = JSON.stringify(await json("provider.json"));
  assert.doesNotMatch(portable, /\/Users\/|[A-Za-z]:\\\\/u);
});

test("the pre-migration owner remains exactly recoverable from Git", async () => {
  const selection = await json("selection.json");
  const baseline = selection.migrationBaseline;
  execFileSync(
    "git",
    ["merge-base", "--is-ancestor", baseline.ownerCommit, "HEAD"],
    {
      cwd: repositoryRoot,
      stdio: "ignore",
    },
  );
  for (const [relative, expected] of Object.entries(baseline.files)) {
    const bytes = execFileSync(
      "git",
      [
        "show",
        `${baseline.ownerCommit}:docs/architecture/client-projection/${relative}`,
      ],
      { cwd: repositoryRoot, encoding: null },
    );
    assert.equal(sha256(bytes), expected, relative);
  }
});
