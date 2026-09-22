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
const archive = process.env.ARCHITECTURE_PUBLISHER_ARCHIVE;
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

async function build(cli, input, output, environment, manifestSha256) {
  const plan = {
    providerManifest: path.join(input, "provider.json"),
    providerManifestSha256: manifestSha256,
    output,
  };
  const bytes = Buffer.from(`${JSON.stringify(plan, null, 2)}\n`);
  const planPath = path.join(
    path.dirname(output),
    `${path.basename(output)}.json`,
  );
  await fs.writeFile(planPath, bytes);
  return JSON.parse(
    run(
      process.execPath,
      [cli, "edition", "build", planPath, "--sha256", sha256(bytes)],
      { cwd: path.dirname(output), env: environment },
    ),
  );
}

test("the selected installed Publisher reproduces the AIGW provider", async (t) => {
  assert.equal(
    path.isAbsolute(archive ?? ""),
    true,
    "set ARCHITECTURE_PUBLISHER_ARCHIVE",
  );
  const selection = JSON.parse(
    await fs.readFile(path.join(providerRoot, "selection.json")),
  );
  assert.equal(
    sha256(await fs.readFile(archive)),
    selection.publisher.archiveSha256,
  );

  const temporary = await fs.realpath(
    await fs.mkdtemp(path.join(os.tmpdir(), "aigw-edition-provider-")),
  );
  t.after(() => fs.rm(temporary, { recursive: true, force: true }));
  const consumer = path.join(temporary, "consumer");
  const home = path.join(temporary, "home");
  const cache = path.join(temporary, "npm-cache");
  await fs.mkdir(consumer);
  await fs.mkdir(home);
  await fs.writeFile(
    path.join(consumer, "package.json"),
    JSON.stringify({ private: true, type: "module" }),
  );
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
    await fs.readFile(resolve("architecture-publisher/package.json"), "utf8"),
  );
  assert.equal(packageManifest.name, selection.publisher.name);
  assert.equal(packageManifest.version, selection.publisher.version);
  const cli = resolve("architecture-publisher/package.json").replace(
    /package\.json$/u,
    "src/cli/main.mjs",
  );
  const environment = { HOME: home, PATH: home, LANG: "C.UTF-8" };
  const results = [];
  for (const name of ["a", "b"]) {
    const input = path.join(temporary, `input-${name}`);
    await fs.cp(providerRoot, input, {
      recursive: true,
      filter: (source) => !source.includes(`${path.sep}test${path.sep}`),
    });
    results.push(
      await build(
        cli,
        input,
        path.join(temporary, `candidate-${name}`),
        environment,
        selection.providerManifestSha256,
      ),
    );
  }

  const expected = selection.migrationBaseline.candidate;
  for (const result of results) {
    for (const field of [
      "candidateSha256",
      "semanticDigest",
      "editionDigest",
      "staticSceneDigest",
      "interactiveSceneDigest",
    ])
      assert.equal(result[field], expected[field], field);
    assert.equal(
      result.manifestSha256,
      selection.providerOutput.manifestSha256,
    );
    assert.equal(
      result.providerMaterializationSha256,
      selection.providerOutput.providerMaterializationSha256,
    );
  }
  const stableResult = ({ manifestPath: _manifestPath, ...result }) => result;
  assert.deepEqual(stableResult(results[0]), stableResult(results[1]));

  const output = path.join(temporary, "candidate-a");
  for (const [relative, field] of [
    ["overview.png", "pngSha256"],
    ["overview.svg", "svgSha256"],
    ["views/overview.html", "overviewHtmlSha256"],
  ])
    assert.equal(
      sha256(await fs.readFile(path.join(output, relative))),
      expected[field],
      relative,
    );

  const receipt = JSON.parse(
    await fs.readFile(path.join(output, "provider.json")),
  );
  assert.equal(
    receipt.schema,
    "architecture.edition-provider-materialization/v1",
  );
  assert.equal(receipt.delivery, "declarative");
  assert.equal(receipt.producer.version, selection.publisher.version);
  assert.equal(receipt.subject.id, selection.authority.subjectRepository);
  assert.equal(
    sha256(await fs.readFile(path.join(output, "provider.json"))),
    results[0].providerMaterializationSha256,
  );

  const editionApi = await import(
    pathToFileURL(resolve("architecture-publisher/edition")).href
  );
  const evolution = await editionApi.compareEditionInput(
    path.join(providerRoot, "evolution.json"),
    sha256(await fs.readFile(path.join(providerRoot, "evolution.json"))),
  );
  assert.equal(evolution.delta.hasChanges, true);
  assert.equal(evolution.obligations.acceptance.length, 1);

  for (const file of await fs.readdir(output)) {
    const selected = path.join(output, file);
    const stat = await fs.stat(selected);
    if (!stat.isFile()) continue;
    const bytes = await fs.readFile(selected);
    assert.equal(bytes.includes(Buffer.from(providerRoot)), false, file);
    assert.equal(bytes.includes(Buffer.from(temporary)), false, file);
  }
});
