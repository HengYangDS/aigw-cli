import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import fs from "node:fs/promises";
import { createRequire } from "node:module";
import path from "node:path";
import process from "node:process";
import { pathToFileURL } from "node:url";

import { format } from "prettier";

const providerRoot = path.resolve(import.meta.dirname);
const repositoryRoot = path.resolve(providerRoot, "../..");
const scratchParent = path.join(repositoryRoot, "build", "tmp");
const sha256 = (bytes) => createHash("sha256").update(bytes).digest("hex");
const jsonBytes = async (value) =>
  Buffer.from(await format(JSON.stringify(value), { parser: "json" }));

function fail(message) {
  throw new Error(message);
}

function options(argv) {
  const selected = { check: false };
  for (let index = 0; index < argv.length; index += 1) {
    const argument = argv[index];
    if (argument === "--check") {
      selected.check = true;
      continue;
    }
    if (
      !["--archive", "--package-identities", "--release-manifest"].includes(
        argument,
      ) ||
      !argv[index + 1]
    )
      fail(
        "usage: node architecture/edition-provider/materialize.mjs " +
          "--archive <package.tgz> --package-identities <json> " +
          "--release-manifest <json> [--check]",
      );
    selected[argument.slice(2).replaceAll("-", "_")] = path.resolve(
      argv[(index += 1)],
    );
  }
  for (const name of ["archive", "package_identities", "release_manifest"])
    if (!selected[name]) fail(`missing --${name.replaceAll("_", "-")}`);
  return selected;
}

async function readBytes(file, maximum = 16 * 1024 * 1024) {
  const absolute = path.resolve(file);
  const stat = await fs.lstat(absolute);
  if (
    !stat.isFile() ||
    stat.isSymbolicLink() ||
    stat.size > maximum ||
    (await fs.realpath(absolute)) !== absolute
  )
    fail(`not a bounded regular file: ${absolute}`);
  return fs.readFile(absolute);
}

async function readJson(file) {
  try {
    return JSON.parse(await readBytes(file));
  } catch (error) {
    if (error instanceof SyntaxError)
      fail(`invalid JSON: ${path.resolve(file)}`);
    throw error;
  }
}

function run(
  command,
  args,
  { cwd = repositoryRoot, environment = process.env } = {},
) {
  const result = spawnSync(command, args, {
    cwd,
    encoding: "utf8",
    env: environment,
    timeout: 120_000,
  });
  if (result.status !== 0)
    fail(`${command} ${args.join(" ")}\n${result.stdout}\n${result.stderr}`);
  return result.stdout;
}

function parseResult(output, operation) {
  try {
    return JSON.parse(output);
  } catch {
    fail(`${operation} returned invalid JSON`);
  }
}

async function verifyPublisher(selected, selection) {
  const [archiveBytes, identitiesBytes, releaseBytes] = await Promise.all([
    readBytes(selected.archive),
    readBytes(selected.package_identities),
    readBytes(selected.release_manifest),
  ]);
  if (sha256(archiveBytes) !== selection.publisher.archiveSha256)
    fail("publisher archive identity mismatch");
  if (
    sha256(identitiesBytes) !==
    selection.publisher.release.packageIdentitiesSha256
  )
    fail("publisher package identities mismatch");
  if (sha256(releaseBytes) !== selection.publisher.release.manifestSha256)
    fail("publisher release manifest mismatch");

  const identities = JSON.parse(identitiesBytes);
  const release = JSON.parse(releaseBytes);
  const packageIdentity = identities.packages?.find(
    ({ file }) => file === path.basename(selected.archive),
  );
  if (!packageIdentity) fail("publisher package identity missing");
  if (
    packageIdentity.archiveSha256 !== selection.publisher.archiveSha256 ||
    packageIdentity.contentTarSha256 !== selection.publisher.contentTarSha256
  )
    fail("publisher package identity does not match the selected release");
  if (
    release.release !== selection.publisher.release.tag ||
    release.commit !== selection.publisher.release.commit
  )
    fail("publisher release identity mismatch");
}

async function installPublisher(scratch, archive, selection) {
  const consumer = path.join(scratch, "publisher");
  const home = path.join(scratch, "home");
  const cache = path.join(scratch, "npm-cache");
  await Promise.all([fs.mkdir(consumer), fs.mkdir(home), fs.mkdir(cache)]);
  await fs.writeFile(
    path.join(consumer, "package.json"),
    await jsonBytes({ private: true, type: "module" }),
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
      environment: {
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
  const packageManifest = await readJson(
    resolve("architecture-publisher/package.json"),
  );
  if (
    packageManifest.name !== selection.publisher.name ||
    packageManifest.version !== selection.publisher.version
  )
    fail("installed Publisher does not match the selected package");
  return {
    cli: path.join(
      path.dirname(resolve("architecture-publisher/package.json")),
      "src",
      "cli",
      "main.mjs",
    ),
    environment: { HOME: home, PATH: home, LANG: "C.UTF-8" },
    resolve,
  };
}

async function selectedSources(semantic, canonical) {
  const rows = [];
  const livePaths = [];
  for (const [id, source] of Object.entries(semantic.sources).sort(
    ([left], [right]) => left.localeCompare(right),
  )) {
    if (!source.locator.startsWith("source/"))
      fail(`source ${id} has a non-repository locator`);
    const relative = source.locator.slice("source/".length);
    const file = path.resolve(repositoryRoot, ...relative.split("/"));
    if (
      file === repositoryRoot ||
      !file.startsWith(`${repositoryRoot}${path.sep}`)
    )
      fail(`source ${id} escapes the repository`);
    const bytes = await readBytes(file);
    source.sha256 = sha256(bytes);
    rows.push({
      id,
      path: source.locator,
      bytes,
      sha256: source.sha256,
      input: file,
    });
    livePaths.push(relative);
  }
  if (new Set(rows.map(({ path: selected }) => selected)).size !== rows.length)
    fail("Claim Model source locators must be unique");
  run("git", ["ls-files", "--error-unmatch", "--", ...livePaths]);
  const revisionInput = structuredClone(semantic);
  delete revisionInput.context.revision;
  const revision = sha256(Buffer.from(canonical(revisionInput)));
  semantic.context.revision = revision;
  return { revision, rows };
}

async function writePlan(file, value) {
  const bytes = await jsonBytes(value);
  await fs.writeFile(file, bytes, { flag: "wx" });
  return sha256(bytes);
}

function build(cli, planPath, digest, environment) {
  return parseResult(
    run(
      process.execPath,
      [cli, "edition", "build", planPath, "--sha256", digest],
      { environment },
    ),
    "edition build",
  );
}

async function selectedJson(root, selection) {
  const file = path.join(root, ...selection.path.split("/"));
  const bytes = await readBytes(file);
  if (sha256(bytes) !== selection.sha256)
    fail(`evolution input identity mismatch: ${selection.path}`);
  return JSON.parse(bytes);
}

async function derive(scratch, publisher, selection) {
  const stagedProvider = path.join(scratch, "provider-input");
  await fs.mkdir(stagedProvider);
  const semantic = structuredClone(
    await readJson(path.join(providerRoot, "_source", "semantic.json")),
  );
  const semantics = await import(
    pathToFileURL(publisher.resolve("architecture-publisher/semantics")).href
  );
  const editionApi = await import(
    pathToFileURL(publisher.resolve("architecture-publisher/edition")).href
  );
  const { revision, rows } = await selectedSources(
    semantic,
    semantics.canonical,
  );
  const semanticBytes = await jsonBytes(semantics.validateClaimModel(semantic));
  const semanticDigest = semantics.claimModelDigest(semantic);
  const semanticInput = path.join(scratch, "semantic.json");
  await fs.writeFile(semanticInput, semanticBytes, { flag: "wx" });

  const packPlan = path.join(scratch, "pack.json");
  const packDigest = await writePlan(packPlan, {
    schema: "architecture.pack-input/v1",
    source: {
      id: "dig/misc/tools/llm-third-party-api/aigw-cli",
      revision,
    },
    files: [
      {
        input: semanticInput,
        path: "semantic.json",
        bytes: semanticBytes.length,
        sha256: sha256(semanticBytes),
      },
      ...rows.map(({ path: selected, bytes, sha256: digest, input }) => ({
        input,
        path: selected,
        bytes: bytes.length,
        sha256: digest,
      })),
    ],
    output: path.join(stagedProvider, "_source"),
  });
  const packed = parseResult(
    run(
      process.execPath,
      [publisher.cli, "source", "pack", packPlan, "--sha256", packDigest],
      { environment: publisher.environment },
    ),
    "source pack",
  );

  const edition = structuredClone(
    await readJson(path.join(providerRoot, "edition.json")),
  );
  edition.product.version = selection.publisher.version;
  edition.source.manifestSha256 = packed.manifestSha256;
  edition.claimModel.digest = semanticDigest;
  edition.scope =
    `Selected AIGW control-plane owners at source revision ${revision}; ` +
    "curated from exact source bytes without inferred implementation completeness.";
  const validatedEdition = editionApi.validateEdition(semantic, edition);
  const editionDigest = editionApi.editionDigest(semantic, validatedEdition);
  const editionBytes = await jsonBytes(validatedEdition);
  const editionSha256 = sha256(editionBytes);
  await fs.writeFile(path.join(stagedProvider, "edition.json"), editionBytes, {
    flag: "wx",
  });

  const evolution = structuredClone(
    await readJson(path.join(providerRoot, "evolution.json")),
  );
  evolution.after.claimModel.sha256 = sha256(semanticBytes);
  for (const selected of evolution.after.editions) {
    if (selected.path !== "edition.json")
      fail(`unsupported current Edition selection: ${selected.path}`);
    selected.sha256 = editionSha256;
  }
  const evolutionBytes = await jsonBytes(evolution);
  await fs.writeFile(
    path.join(stagedProvider, "evolution.json"),
    evolutionBytes,
    {
      flag: "wx",
    },
  );
  await fs.cp(
    path.join(providerRoot, "history"),
    path.join(stagedProvider, "history"),
    {
      recursive: true,
    },
  );
  const providerEvolution = {
    before: {
      claimModel: await selectedJson(
        stagedProvider,
        evolution.before.claimModel,
      ),
      editions: await Promise.all(
        evolution.before.editions.map((selected) =>
          selectedJson(stagedProvider, selected),
        ),
      ),
    },
    after: {
      claimModel: semantic,
      editions: [validatedEdition],
    },
    correspondences: evolution.correspondences,
  };
  editionApi.compileEditionEvolution(providerEvolution);
  const providerEvolutionBytes = await jsonBytes(providerEvolution);
  const providerEvolutionSha256 = sha256(providerEvolutionBytes);
  await fs.writeFile(
    path.join(stagedProvider, "provider-evolution.json"),
    providerEvolutionBytes,
    { flag: "wx" },
  );

  const provider = structuredClone(
    await readJson(path.join(providerRoot, "provider.json")),
  );
  provider.provider.revision = `source-set:${packed.manifestSha256}`;
  provider.subject = {
    id: "dig/misc/tools/llm-third-party-api/aigw-cli",
    revision,
  };
  provider.delivery.sourceManifestSha256 = packed.manifestSha256;
  provider.delivery.editionSha256 = editionSha256;
  provider.delivery.evolutionSha256 = providerEvolutionSha256;
  provider.expected.sourceManifestSha256 = packed.manifestSha256;
  provider.expected.claimModelDigest = semanticDigest;
  provider.expected.editionDigest = editionDigest;
  provider.expected.evolutionSha256 = providerEvolutionSha256;
  const providerBytes = await jsonBytes(provider);
  const providerManifestSha256 = sha256(providerBytes);
  await fs.writeFile(
    path.join(stagedProvider, "provider.json"),
    providerBytes,
    {
      flag: "wx",
    },
  );

  const directPlan = path.join(scratch, "edition-direct.json");
  const directOutput = path.join(scratch, "candidate-direct");
  const directDigest = await writePlan(directPlan, {
    sourceManifest: path.join(stagedProvider, "_source", "manifest.json"),
    sourceSha256: packed.manifestSha256,
    semanticMember: "semantic.json",
    edition: path.join(stagedProvider, "edition.json"),
    editionSha256,
    output: directOutput,
  });
  const direct = build(
    publisher.cli,
    directPlan,
    directDigest,
    publisher.environment,
  );

  const providerPlan = path.join(scratch, "edition-provider.json");
  const providerOutput = path.join(scratch, "candidate-provider");
  const providerPlanDigest = await writePlan(providerPlan, {
    providerManifest: path.join(stagedProvider, "provider.json"),
    providerManifestSha256,
    output: providerOutput,
  });
  const materialized = build(
    publisher.cli,
    providerPlan,
    providerPlanDigest,
    publisher.environment,
  );
  for (const field of [
    "candidateSha256",
    "semanticDigest",
    "editionDigest",
    "staticSceneDigest",
    "interactiveSceneDigest",
  ])
    if (direct[field] !== materialized[field])
      fail(`direct and Provider builds disagree on ${field}`);

  const selected = structuredClone(selection);
  selected.providerManifestSha256 = providerManifestSha256;
  selected.providerOutput = {
    manifestSha256: materialized.manifestSha256,
    providerMaterializationSha256: materialized.providerMaterializationSha256,
  };
  selected.migrationBaseline.providerEvolutionSha256 = providerEvolutionSha256;
  selected.migrationBaseline.candidate = {
    manifestSha256: direct.manifestSha256,
    candidateSha256: direct.candidateSha256,
    semanticDigest: direct.semanticDigest,
    editionDigest: direct.editionDigest,
    staticSceneDigest: direct.staticSceneDigest,
    interactiveSceneDigest: direct.interactiveSceneDigest,
    pngSha256: sha256(await readBytes(path.join(directOutput, "overview.png"))),
    svgSha256: sha256(await readBytes(path.join(directOutput, "overview.svg"))),
    overviewHtmlSha256: sha256(
      await readBytes(path.join(directOutput, "views", "overview.html")),
    ),
  };
  const selectionBytes = await jsonBytes(selected);
  await fs.writeFile(
    path.join(stagedProvider, "selection.json"),
    selectionBytes,
    {
      flag: "wx",
    },
  );

  const desired = new Map([
    ["edition.json", editionBytes],
    ["evolution.json", evolutionBytes],
    ["provider-evolution.json", providerEvolutionBytes],
    ["provider.json", providerBytes],
    ["selection.json", selectionBytes],
  ]);
  const sourceManifest = await readJson(
    path.join(stagedProvider, "_source", "manifest.json"),
  );
  desired.set(
    "_source/manifest.json",
    await readBytes(path.join(stagedProvider, "_source", "manifest.json")),
  );
  for (const member of sourceManifest.files)
    desired.set(
      `_source/${member.path}`,
      await readBytes(
        path.join(stagedProvider, "_source", ...member.path.split("/")),
      ),
    );
  return { desired, sourceManifest, direct, materialized, packed };
}

async function obsoleteSourceFiles(sourceManifest) {
  const desired = new Set([
    "manifest.json",
    ...sourceManifest.files.map(({ path: selected }) => selected),
  ]);
  return (await filesBelow(path.join(providerRoot, "_source")))
    .filter((selected) => !desired.has(selected))
    .map((selected) => `_source/${selected}`);
}

async function filesBelow(directory, prefix = "") {
  const files = [];
  for (const entry of await fs.readdir(directory, { withFileTypes: true })) {
    const relative = prefix ? `${prefix}/${entry.name}` : entry.name;
    const file = path.join(directory, entry.name);
    if (entry.isSymbolicLink())
      fail(`generated source contains a symlink: ${relative}`);
    if (entry.isDirectory()) files.push(...(await filesBelow(file, relative)));
    else if (entry.isFile()) files.push(relative);
    else fail(`generated source contains an unsupported entry: ${relative}`);
  }
  return files.sort();
}

async function compare(desired, obsolete) {
  const mismatches = [];
  for (const [relative, bytes] of desired) {
    try {
      if (
        !(
          await readBytes(path.join(providerRoot, ...relative.split("/")))
        ).equals(bytes)
      )
        mismatches.push(relative);
    } catch {
      mismatches.push(relative);
    }
  }
  for (const relative of obsolete) {
    try {
      await fs.lstat(path.join(providerRoot, ...relative.split("/")));
      mismatches.push(relative);
    } catch (error) {
      if (error?.code !== "ENOENT") throw error;
    }
  }
  return mismatches.sort();
}

let temporaryIndex = 0;
async function atomicWrite(file, bytes) {
  await fs.mkdir(path.dirname(file), { recursive: true });
  const temporary = `${file}.materialize-${process.pid}-${temporaryIndex++}`;
  try {
    await fs.writeFile(temporary, bytes, { flag: "wx" });
    await fs.rename(temporary, file);
  } finally {
    await fs.rm(temporary, { force: true });
  }
}

async function apply(desired, obsolete) {
  const original = new Map();
  for (const relative of new Set([...desired.keys(), ...obsolete])) {
    const file = path.join(providerRoot, ...relative.split("/"));
    try {
      original.set(relative, await readBytes(file));
    } catch (error) {
      if (error?.code !== "ENOENT") throw error;
      original.set(relative, null);
    }
  }
  try {
    for (const [relative, bytes] of desired)
      await atomicWrite(path.join(providerRoot, ...relative.split("/")), bytes);
    for (const relative of obsolete)
      await fs.rm(path.join(providerRoot, ...relative.split("/")));
  } catch (error) {
    for (const [relative, bytes] of original) {
      const file = path.join(providerRoot, ...relative.split("/"));
      if (bytes === null) await fs.rm(file, { force: true });
      else await atomicWrite(file, bytes);
    }
    throw error;
  }
}

async function main() {
  const selected = options(process.argv.slice(2));
  const selection = await readJson(path.join(providerRoot, "selection.json"));
  await verifyPublisher(selected, selection);
  await fs.mkdir(scratchParent, { recursive: true });
  const scratch = await fs.realpath(
    await fs.mkdtemp(path.join(scratchParent, "edition-provider-materialize-")),
  );
  try {
    const publisher = await installPublisher(
      scratch,
      selected.archive,
      selection,
    );
    const result = await derive(scratch, publisher, selection);
    const obsolete = await obsoleteSourceFiles(result.sourceManifest);
    const mismatches = await compare(result.desired, obsolete);
    if (selected.check) {
      if (mismatches.length > 0)
        fail(`Edition Provider is stale:\n${mismatches.join("\n")}`);
    } else if (mismatches.length > 0) {
      await apply(result.desired, obsolete);
    }
    const remaining = await compare(result.desired, obsolete);
    if (remaining.length > 0)
      fail(
        `Edition Provider materialization is incomplete:\n${remaining.join("\n")}`,
      );
    process.stdout.write(
      `${JSON.stringify(
        {
          status: selected.check
            ? "current"
            : mismatches.length
              ? "updated"
              : "unchanged",
          sourceManifestSha256: result.packed.manifestSha256,
          candidateSha256: result.materialized.candidateSha256,
          providerManifestSha256: sha256(result.desired.get("provider.json")),
          changed: mismatches,
        },
        null,
        2,
      )}\n`,
    );
  } finally {
    await fs.rm(scratch, { recursive: true, force: true });
  }
}

await main();
