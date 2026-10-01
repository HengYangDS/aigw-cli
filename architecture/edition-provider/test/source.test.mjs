import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import fs from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const providerRoot = path.resolve(import.meta.dirname, "..");
const repositoryRoot = path.resolve(providerRoot, "../..");
const sha256 = (bytes) => createHash("sha256").update(bytes).digest("hex");
const json = async (relative) => JSON.parse(await regularBytes(relative));
const git = (...args) => execFileSync("git", args, { cwd: repositoryRoot });

async function regularBytes(relative) {
  const file = path.join(providerRoot, ...relative.split("/"));
  const stat = await fs.lstat(file);
  assert.equal(stat.isFile(), true, relative);
  assert.equal(stat.isSymbolicLink(), false, relative);
  return fs.readFile(file);
}

test("the selected native Claim Model uses Provider v2 without source snapshots", async () => {
  const provider = await json("provider.json");
  assert.equal(provider.schema, "architecture.edition-provider/v2");
  assert.equal(provider.delivery.kind, "declarative");
  assert.equal(provider.provider.id, "aigw-client-projection-edition");
  assert.equal(provider.selected.native.length, 1);
  const native = provider.selected.native[0];
  assert.deepEqual(native.format, {
    id: "architecture.claim-model",
    version: "2",
  });
  assert.deepEqual(native.revision, { kind: "content", algorithm: "sha256" });
  assert.equal(native.claimant, "aigw-cli");
  assert.deepEqual(provider.selected.derived, []);
  const modelBytes = await regularBytes("claim-model.json");
  assert.equal(native.sha256, sha256(modelBytes));
  assert.equal(native.bytes, modelBytes.length);
  const editionBytes = await regularBytes("edition.json");
  assert.equal(provider.selected.edition.sha256, sha256(editionBytes));
  assert.equal(provider.selected.edition.bytes, editionBytes.length);
  assert.deepEqual(provider.delivery.files, [
    { kind: "native", id: native.id, path: "claim-model.json" },
    { kind: "edition", id: provider.selected.edition.id, path: "edition.json" },
  ]);
  for (const legacy of [
    "_source",
    "materialize.mjs",
    "provider-evolution.json",
    "evolution.json",
    "history",
  ])
    await assert.rejects(fs.stat(path.join(providerRoot, legacy)), {
      code: "ENOENT",
    });
});

test("the public Build Request pins one declared Provider and no extra inputs", async () => {
  const [request, provider, selection] = await Promise.all([
    json("build-request.json"),
    json("provider.json"),
    json("selection.json"),
  ]);
  assert.deepEqual(request, {
    schema: "architecture.build-request/v1",
    edition: { kind: "provider", provider: provider.provider.id },
    sources: [],
    providers: [
      {
        id: provider.provider.id,
        manifest: "provider.json",
        sha256: sha256(await regularBytes("provider.json")),
      },
    ],
    media: ["static", "interactive"],
    toolchain: { profile: "portable" },
  });
  assert.equal(selection.schema, "aigw.architecture-publisher-selection/v2");
  assert.equal(selection.publisher.name, "architecture-publisher");
  assert.equal(
    selection.publisher.release.tag,
    `v${selection.publisher.version}`,
  );
  for (const digest of [
    selection.publisher.archiveSha256,
    selection.publisher.release.manifestSha256,
  ])
    assert.match(digest, /^[0-9a-f]{64}$/u);
  assert.equal(selection.authority.authorizationOwner, "AIGW");
  assert.equal(selection.authority.scope, "Client Projection only");
  assert.equal(selection.authority.publisherRole, "non-authorizing compiler");
  assert.doesNotMatch(JSON.stringify(request), /\/Users\/|[A-Za-z]:\\\\/u);
});

test("authored claims and their live provenance retain one source owner", async () => {
  const model = await json("claim-model.json");
  const paths = new Set();
  for (const [id, source] of Object.entries(model.sources)) {
    assert.equal(source.locator.startsWith("source/"), true, id);
    const relative = source.locator.slice("source/".length);
    assert.equal(paths.has(relative), false, id);
    paths.add(relative);
    git("ls-files", "--error-unmatch", "--", relative);
    assert.equal(
      sha256(await fs.readFile(path.join(repositoryRoot, relative))),
      source.sha256,
      id,
    );
  }
  const { rollback } = await json("selection.json");
  const baseline = JSON.parse(
    git(
      "show",
      `${rollback.commit}:architecture/edition-provider/edition.json`,
    ),
  );
  const edition = await json("edition.json");
  assert.equal(edition.schema, "architecture.edition-project/v1");
  assert.equal(edition.id, baseline.id);
  assert.equal(edition.title, baseline.title);
  assert.equal(edition.editor, baseline.owner);
  assert.deepEqual(
    edition.questions,
    Object.entries(baseline.questions).map(([id, value]) => ({
      id,
      audience: value.audience,
      text: value.prompt,
    })),
  );
  for (const claim of baseline.requiredClaims)
    for (const kind of ["claim", "relation"])
      assert.equal(
        edition.scope.selections.some(
          ({ reference }) => reference === `${kind}:${claim}`,
        ),
        true,
        `${kind}:${claim}`,
      );
});

test("the complete predecessor is recoverable without retained mutable copies", async () => {
  const { rollback } = await json("selection.json");
  git("merge-base", "--is-ancestor", rollback.commit, "HEAD");
  assert.equal(
    git("rev-parse", `${rollback.commit}:architecture/edition-provider`)
      .toString()
      .trim(),
    rollback.tree,
  );
  assert.deepEqual(
    git(
      "show",
      `${rollback.commit}:architecture/edition-provider/_source/semantic.json`,
    ),
    await regularBytes("claim-model.json"),
  );
});
