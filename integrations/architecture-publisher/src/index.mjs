import fs from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { isDigest, readSourceBundle } from "architecture-publisher/source";
import {
  claimModelDigest,
  validateClaimModel,
} from "architecture-publisher/semantics";
import { editionDigest, validateEdition } from "architecture-publisher/edition";

const authoringRoot = new URL("../authoring/", import.meta.url);

function fail(code) {
  throw Object.assign(new Error(code), { code });
}

async function authoring(name) {
  const bytes = await fs.readFile(new URL(name, authoringRoot));
  try {
    return JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(bytes));
  } catch {
    fail("aigw_architecture_authoring_invalid");
  }
}

const clone = (value) => structuredClone(value);

/** Compile AIGW-owned semantic authoring from one exact Source Bundle. */
export async function readAigwArchitecture(manifestPath, manifestSha256) {
  if (!isDigest(manifestSha256)) fail("aigw_source_digest_required");
  const [bundle, declared] = await Promise.all([
    readSourceBundle(manifestPath, manifestSha256),
    authoring("model.json"),
  ]);
  if (
    declared?.schema !== "aigw.architecture-authoring/v1" ||
    declared.owner !== "aigw-cli" ||
    !declared.sources ||
    typeof declared.sources !== "object" ||
    Array.isArray(declared.sources)
  )
    fail("aigw_architecture_authoring_invalid");
  const members = new Map(
    bundle.members.map((member) => [member.path, member]),
  );
  const selected = Object.entries(declared.sources);
  if (
    selected.length !== members.size ||
    selected.some(([, source]) => !members.has(source.path))
  )
    fail("aigw_source_members_mismatch");
  const sources = Object.fromEntries(
    selected.map(([id, source]) => {
      const member = members.get(source.path);
      return [
        id,
        {
          sha256: member.sha256,
          locator: member.path,
          authority: source.authority,
        },
      ];
    }),
  );
  const model = clone(declared);
  delete model.sources;
  model.schema = "architecture.claim-model/v2";
  model.sources = sources;
  model.context = {
    ...model.context,
    revision: bundle.source.revision ?? "unversioned",
  };
  const claimModel = validateClaimModel(model);
  return Object.freeze({
    source: clone(bundle.source),
    manifestSha256: bundle.manifestSha256,
    claimModel,
    claimModelDigest: claimModelDigest(claimModel),
  });
}

/** Bind the AIGW-owned Edition to one exact compiled semantic value. */
export async function createAigwEdition(architecture, sourceManifestSha256) {
  if (
    !architecture ||
    !isDigest(sourceManifestSha256) ||
    !isDigest(architecture.claimModelDigest)
  )
    fail("aigw_architecture_selection_invalid");
  const declared = await authoring("edition.json");
  if (
    declared?.schema !== "aigw.architecture-edition-authoring/v1" ||
    declared.owner !== "aigw-cli"
  )
    fail("aigw_edition_authoring_invalid");
  const edition = clone(declared);
  edition.schema = "architecture.edition/v1";
  edition.product = {
    name: "Architecture Publisher",
    schema: "architecture.publisher/v1",
    version: "0.2.0-alpha.0",
  };
  edition.source = { manifestSha256: sourceManifestSha256 };
  edition.claimModel = {
    schema: "architecture.claim-model/v2",
    digest: architecture.claimModelDigest,
  };
  const selected = validateEdition(architecture.claimModel, edition);
  return Object.freeze({
    edition: selected,
    editionDigest: editionDigest(architecture.claimModel, selected),
  });
}

export const integrationRoot = fileURLToPath(new URL("../", import.meta.url));
