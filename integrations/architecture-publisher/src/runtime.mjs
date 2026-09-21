import { createHash } from "node:crypto";
import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const sha256 = (bytes) => createHash("sha256").update(bytes).digest("hex");

async function visit(directory, prefix, members) {
  for (const entry of await fs.readdir(directory, { withFileTypes: true })) {
    const relative = prefix ? `${prefix}/${entry.name}` : entry.name;
    const file = path.join(directory, entry.name);
    if (entry.isSymbolicLink())
      throw Error("AIGW architecture runtime symlink");
    if (entry.isDirectory()) await visit(file, relative, members);
    else if (entry.isFile()) {
      const bytes = await fs.readFile(file);
      members.push({
        path: relative,
        bytes: bytes.length,
        sha256: sha256(bytes),
      });
    } else throw Error("Unsupported AIGW architecture runtime member");
  }
}

/** Return the exact installed integration closure without source-repository access. */
export async function readAigwArchitectureRuntime() {
  const members = [];
  await visit(path.join(root, "authoring"), "authoring", members);
  await visit(path.join(root, "src"), "src", members);
  for (const name of ["package.json", "README.md"]) {
    const bytes = await fs.readFile(path.join(root, name));
    members.push({ path: name, bytes: bytes.length, sha256: sha256(bytes) });
  }
  return members.sort((left, right) => left.path.localeCompare(right.path));
}
