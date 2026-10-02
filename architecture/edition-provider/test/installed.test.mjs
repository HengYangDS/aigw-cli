import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import fs from "node:fs/promises";
import { createRequire } from "node:module";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { pathToFileURL } from "node:url";

const providerRoot = path.resolve(import.meta.dirname, "..");
const sha256 = (bytes) => createHash("sha256").update(bytes).digest("hex");

function run(command, args, options = {}) {
  const result = spawnSync(command, args, {
    encoding: "utf8",
    timeout: 120_000,
    ...options,
  });
  assert.equal(
    result.status,
    0,
    `${command} ${args.join(" ")}\n${result.stdout}\n${result.stderr}`,
  );
  return result.stdout;
}

async function writeJson(file, value) {
  const bytes = Buffer.from(`${JSON.stringify(value, null, 2)}\n`);
  await fs.writeFile(file, bytes);
  return sha256(bytes);
}

async function build(cli, input, output, environment) {
  const requestPath = path.join(input, "build-request.json");
  const request = await fs.readFile(requestPath);
  return JSON.parse(
    run(
      process.execPath,
      [
        cli,
        "edition",
        "build",
        requestPath,
        "--sha256",
        sha256(request),
        "--output",
        output,
      ],
      {
        cwd: input,
        env: environment,
      },
    ),
  );
}

async function bundleBytes(output) {
  const manifest = JSON.parse(
    await fs.readFile(path.join(output, "manifest.json")),
  );
  return Object.fromEntries(
    await Promise.all(
      manifest.files.map(async ({ path: relative }) => [
        relative,
        sha256(await fs.readFile(path.join(output, relative))),
      ]),
    ),
  );
}

test("the exact released Publisher replays native AIGW inputs offline", async (t) => {
  const selection = JSON.parse(
    await fs.readFile(path.join(providerRoot, "selection.json")),
  );
  const archive = process.env.ARCHITECTURE_PUBLISHER_ARCHIVE;
  const releaseManifest = process.env.ARCHITECTURE_PUBLISHER_RELEASE_MANIFEST;
  for (const [name, selected] of Object.entries({
    ARCHITECTURE_PUBLISHER_ARCHIVE: archive,
    ARCHITECTURE_PUBLISHER_RELEASE_MANIFEST: releaseManifest,
  }))
    assert.equal(path.isAbsolute(selected ?? ""), true, `set ${name}`);
  const releaseBytes = await fs.readFile(releaseManifest);
  const release = JSON.parse(releaseBytes);
  assert.equal(
    sha256(releaseBytes),
    selection.publisher.release.manifestSha256,
  );
  assert.equal(release.tag, selection.publisher.release.tag);
  assert.equal(release.sourceCommit, selection.publisher.release.commit);
  const archiveBytes = await fs.readFile(archive);
  assert.equal(sha256(archiveBytes), selection.publisher.archiveSha256);
  const asset = release.selectedAssets.find(
    ({ name }) => name === path.basename(archive),
  );
  assert.equal(asset.sha256, selection.publisher.archiveSha256);
  assert.equal(asset.bytes, archiveBytes.length);

  const temporary = await fs.realpath(
    await fs.mkdtemp(path.join(os.tmpdir(), "aigw-native-edition-")),
  );
  t.after(() => fs.rm(temporary, { recursive: true, force: true }));
  const consumer = path.join(temporary, "consumer");
  const home = path.join(temporary, "home");
  const cache = path.join(temporary, "npm-cache");
  await Promise.all(
    [consumer, home, cache].map((directory) => fs.mkdir(directory)),
  );
  await writeJson(path.join(consumer, "package.json"), {
    private: true,
    type: "module",
  });
  run(
    "npm",
    [
      "install",
      "--offline",
      "--ignore-scripts",
      "--no-audit",
      "--no-fund",
      "--package-lock=false",
      archive,
    ],
    {
      cwd: consumer,
      env: {
        ...process.env,
        HOME: home,
        npm_config_cache: cache,
        npm_config_offline: "true",
        npm_config_registry: "https://registry.invalid/",
        npm_config_update_notifier: "false",
      },
    },
  );
  const resolve = createRequire(path.join(consumer, "package.json")).resolve;
  const packageManifest = JSON.parse(
    await fs.readFile(resolve("architecture-publisher/package.json")),
  );
  assert.equal(packageManifest.name, selection.publisher.name);
  assert.equal(packageManifest.version, selection.publisher.version);
  const cli = path.join(
    path.dirname(resolve("architecture-publisher/package.json")),
    "src/cli/main.mjs",
  );
  const environment = {
    HOME: home,
    PATH: home,
    LANG: "C.UTF-8",
    TMPDIR: temporary,
    TMP: temporary,
    TEMP: temporary,
  };
  const inputs = [];
  for (const name of ["a", "b"]) {
    const input = path.join(temporary, `input-${name}`);
    await fs.mkdir(input);
    for (const file of [
      "build-request.json",
      "provider.json",
      "claim-model.json",
      "edition.json",
    ])
      await fs.copyFile(path.join(providerRoot, file), path.join(input, file));
    inputs.push(input);
  }
  const first = await build(
    cli,
    inputs[0],
    path.join(temporary, "candidate-a"),
    environment,
  );
  const second = await build(
    cli,
    inputs[1],
    path.join(temporary, "candidate-b"),
    environment,
  );
  for (const key of [
    "manifestSha256",
    "lockSha256",
    "requestSha256",
    "candidateSha256",
  ])
    assert.equal(first[key], second[key], key);
  assert.deepEqual(
    await bundleBytes(first.output),
    await bundleBytes(second.output),
  );

  const model = JSON.parse(
    await fs.readFile(path.join(inputs[0], "claim-model.json")),
  );
  const project = JSON.parse(
    await fs.readFile(path.join(inputs[0], "edition.json")),
  );
  const candidate = JSON.parse(
    await fs.readFile(path.join(first.output, "candidate.json")),
  );
  const lock = JSON.parse(
    await fs.readFile(path.join(first.output, "input-lock.json")),
  );
  assert.equal(candidate.schema, "architecture.edition-candidate/v2");
  assert.equal(lock.sources.length, 1);
  assert.deepEqual(lock.derived, []);
  assert.equal(
    lock.sources[0].sha256,
    sha256(await fs.readFile(path.join(inputs[0], "claim-model.json"))),
  );
  assert.equal(candidate.qualification, "unqualified");
  for (const medium of ["static", "interactive"]) {
    const questions = [
      ...new Set(
        project.presentation[medium].views.flatMap((view) => view.questions),
      ),
    ];
    assert.deepEqual(candidate.obligations[medium].questionIds, questions);
    for (const question of questions) {
      const assertions =
        candidate.obligations[medium].assertionsByQuestion[question];
      for (const selected of project.scope.selections.filter(({ questions }) =>
        questions.includes(question),
      ))
        assert.equal(
          assertions.includes(selected.id),
          true,
          `${medium}/${question}/${selected.id}`,
        );
    }
  }
  const staticBytes = await fs.readFile(
    path.join(first.output, "static/architecture.svg"),
  );
  const png = await fs.readFile(
    path.join(first.output, "static/architecture.png"),
  );
  const interactive = await fs.readFile(
    path.join(first.output, "interactive/architecture.html"),
  );
  assert.equal(png.subarray(0, 8).toString("hex"), "89504e470d0a1a0a");
  for (const { label } of Object.values(model.entities)) {
    assert.equal(
      staticBytes.includes(Buffer.from(label)),
      true,
      `static entity ${label}`,
    );
    assert.equal(
      interactive.includes(Buffer.from(label)),
      true,
      `interactive entity ${label}`,
    );
  }
  for (const question of project.questions)
    assert.equal(
      interactive.includes(Buffer.from(question.text)),
      true,
      question.id,
    );

  const compiler = await import(
    pathToFileURL(resolve("architecture-publisher/compiler")).href
  );
  const provider = JSON.parse(
    await fs.readFile(path.join(inputs[0], "provider.json")),
  );
  const native = provider.selected.native[0];
  const directEdition = path.join(inputs[0], "direct-edition");
  await fs.mkdir(directEdition);
  await fs.copyFile(
    path.join(inputs[0], "edition.json"),
    path.join(directEdition, "edition.json"),
  );
  const directRequest = {
    schema: "architecture.build-request/v1",
    edition: {
      kind: "direct",
      root: "direct-edition",
      manifest: "edition.json",
    },
    sources: [
      {
        id: native.id,
        namespace: native.namespace,
        claimant: native.claimant,
        format: native.format,
        revision: native.revision,
        input: { kind: "pin", path: "claim-model.json", sha256: native.sha256 },
      },
    ],
    providers: [],
    media: ["static", "interactive"],
    toolchain: { profile: "portable" },
  };
  const directPath = path.join(inputs[0], "direct-request.json");
  const directDigest = await writeJson(directPath, directRequest);
  const direct = await compiler.compileEditionCandidateInput(
    directPath,
    directDigest,
    path.join(temporary, "direct"),
  );
  const directLock = JSON.parse(
    await fs.readFile(path.join(direct.output, "input-lock.json")),
  );
  assert.deepEqual(directLock, lock);
  const directCandidate = JSON.parse(
    await fs.readFile(path.join(direct.output, "candidate.json")),
  );
  assert.equal(directCandidate.input.closure, "selected-roots-only");
  assert.equal(candidate.input.closure, "selected-inputs-only");
  for (const field of [
    "lockSha256",
    "editionSha256",
    "semanticDigest",
    "semanticSupplies",
  ])
    assert.deepEqual(
      directCandidate.input[field],
      candidate.input[field],
      field,
    );
  for (const field of ["media", "obligations", "limits", "qualification"])
    assert.deepEqual(directCandidate[field], candidate[field], field);

  const original = await fs.readFile(path.join(inputs[0], "claim-model.json"));
  const changed = Buffer.from(original);
  changed[changed.indexOf(Buffer.from("aigw-cli"))] = "b".charCodeAt(0);
  await fs.writeFile(path.join(inputs[0], "claim-model.json"), changed);
  const refused = spawnSync(
    process.execPath,
    [
      cli,
      "edition",
      "build",
      path.join(inputs[0], "build-request.json"),
      "--sha256",
      first.requestSha256,
      "--output",
      path.join(temporary, "rejected"),
    ],
    { cwd: inputs[0], env: environment, encoding: "utf8", timeout: 30_000 },
  );
  assert.equal(refused.status, 1);
  assert.equal(
    JSON.parse(refused.stdout).error.code,
    "edition_provider_v2_member_digest_mismatch",
  );
  await assert.rejects(fs.stat(path.join(temporary, "rejected")), {
    code: "ENOENT",
  });
  await fs.writeFile(path.join(inputs[0], "claim-model.json"), original);
  const replay = await build(
    cli,
    inputs[0],
    path.join(temporary, "restored"),
    environment,
  );
  assert.equal(replay.candidateSha256, first.candidateSha256);
  for (const file of Object.keys(await bundleBytes(first.output))) {
    const bytes = await fs.readFile(path.join(first.output, file));
    assert.equal(bytes.includes(Buffer.from(providerRoot)), false, file);
    assert.equal(bytes.includes(Buffer.from(temporary)), false, file);
  }
});
