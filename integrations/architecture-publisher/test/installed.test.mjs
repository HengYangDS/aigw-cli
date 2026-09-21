import assert from "node:assert/strict";
import { execFileSync, spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import fs from "node:fs/promises";
import { createRequire } from "node:module";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { pathToFileURL } from "node:url";

const root = path.resolve(import.meta.dirname, "../../..");
const integration = path.join(root, "integrations/architecture-publisher");
const sha256 = (bytes) => createHash("sha256").update(bytes).digest("hex");

function run(command, args, options = {}) {
  const result = spawnSync(command, args, {
    encoding: "utf8",
    timeout: 120000,
    ...options,
  });
  assert.equal(
    result.status,
    0,
    `${command} ${args.join(" ")}\n${result.stdout}\n${result.stderr}`,
  );
  return result.stdout;
}

function selectedPublisherRoot() {
  const value = process.env.ARCHITECTURE_PUBLISHER_ROOT;
  assert.equal(path.isAbsolute(value ?? ""), true);
  return value;
}

async function pack(cwd, destination, workspace) {
  const args = [
    "pack",
    "--json",
    "--ignore-scripts",
    "--pack-destination",
    destination,
  ];
  if (workspace) args.push("--workspace", workspace);
  return JSON.parse(run("npm", args, { cwd }))[0].filename;
}

test("packed AIGW Edition builds exact sibling media from current selected source", async (t) => {
  const publisherRoot = selectedPublisherRoot();
  const temporary = await fs.realpath(
    await fs.mkdtemp(path.join(os.tmpdir(), "aigw-architecture-publisher-")),
  );
  t.after(() => fs.rm(temporary, { recursive: true, force: true }));
  const coreArchive = path.join(
    temporary,
    await pack(publisherRoot, temporary, "architecture-publisher"),
  );
  const integrationArchive = path.join(
    temporary,
    await pack(integration, temporary),
  );
  const project = path.join(temporary, "consumer");
  const home = path.join(temporary, "home");
  const cache = path.join(temporary, "npm-cache");
  await fs.mkdir(project);
  await fs.mkdir(home);
  await fs.writeFile(
    path.join(project, "package.json"),
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
      coreArchive,
      integrationArchive,
    ],
    {
      cwd: project,
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

  const resolve = createRequire(path.join(project, "package.json")).resolve;
  const importPublic = async (specifier) =>
    import(pathToFileURL(resolve(specifier)).href);
  const sourceApi = await importPublic("architecture-publisher/source");
  const integrationApi = await importPublic("@architecture-publisher/aigw");
  const runtimeApi = await importPublic("@architecture-publisher/aigw/runtime");
  const model = JSON.parse(
    await fs.readFile(path.join(integration, "authoring/model.json"), "utf8"),
  );
  const revision = execFileSync("git", ["rev-parse", "HEAD"], {
    cwd: root,
    encoding: "utf8",
  }).trim();
  const selectedMembers = await Promise.all(
    Object.values(model.sources).map(async ({ path: relative }) => ({
      path: relative,
      content: await fs.readFile(path.join(root, relative)),
    })),
  );
  const evidence = await sourceApi.writeSourceBundle(
    path.join(project, "selected-source"),
    { id: "aigw-cli", revision },
    selectedMembers,
  );
  const architecture = await integrationApi.readAigwArchitecture(
    evidence.manifestPath,
    evidence.manifestSha256,
  );
  assert.equal(architecture.source.revision, revision);
  assert.match(architecture.claimModelDigest, /^[a-f0-9]{64}$/);

  const semanticBytes = Buffer.from(
    `${JSON.stringify(architecture.claimModel, null, 2)}\n`,
  );
  const candidateSource = await sourceApi.writeSourceBundle(
    path.join(project, "candidate-source"),
    architecture.source,
    [...selectedMembers, { path: "semantic.json", content: semanticBytes }],
  );
  const selectedEdition = await integrationApi.createAigwEdition(
    architecture,
    candidateSource.manifestSha256,
  );
  const editionBytes = Buffer.from(
    `${JSON.stringify(selectedEdition.edition, null, 2)}\n`,
  );
  const editionPath = path.join(project, "edition.json");
  await fs.writeFile(editionPath, editionBytes);
  const output = path.join(project, "candidate");
  const plan = {
    sourceManifest: candidateSource.manifestPath,
    sourceSha256: candidateSource.manifestSha256,
    semanticMember: "semantic.json",
    edition: editionPath,
    editionSha256: sha256(editionBytes),
    output,
  };
  const planBytes = Buffer.from(`${JSON.stringify(plan, null, 2)}\n`);
  const planPath = path.join(project, "build.json");
  await fs.writeFile(planPath, planBytes);
  const cli = resolve("architecture-publisher/package.json").replace(
    /package\.json$/,
    "src/cli/main.mjs",
  );
  const response = JSON.parse(
    run(
      process.execPath,
      [cli, "edition", "build", planPath, "--sha256", sha256(planBytes)],
      {
        cwd: project,
        env: { HOME: home, PATH: home, LANG: "C.UTF-8" },
      },
    ),
  );
  assert.equal(response.status, "built");
  assert.equal(response.editionDigest, selectedEdition.editionDigest);
  const candidate = await sourceApi.readSourceBundle(
    path.join(output, "manifest.json"),
    response.manifestSha256,
  );
  const members = new Map(
    candidate.members.map((member) => [member.path, member.content]),
  );
  assert(
    members
      .get("overview.png")
      .subarray(0, 8)
      .equals(Buffer.from([137, 80, 78, 71, 13, 10, 26, 10])),
  );
  assert(members.has("overview.svg"));
  for (const page of ["overview", "state", "projection", "safety"])
    assert(members.has(`views/${page}.html`), page);
  for (const content of members.values())
    assert.equal(content.includes(Buffer.from(root)), false);

  const runtime = await runtimeApi.readAigwArchitectureRuntime();
  assert(runtime.some(({ path: file }) => file === "authoring/model.json"));
  assert(runtime.some(({ path: file }) => file === "src/index.mjs"));
  assert(runtime.every(({ sha256: digest }) => /^[a-f0-9]{64}$/.test(digest)));
});
