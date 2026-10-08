// Format or check the declared authored inventory with locked native Prettier.
import { text } from "node:stream/consumers";
import { readFileSync, writeFileSync } from "node:fs";
import { relative, sep } from "node:path";
import { format, getFileInfo } from "../../node_modules/prettier/index.mjs";

const options = process.argv.slice(2);
if (options.length > 1 || (options.length === 1 && options[0] !== "--write")) {
  throw new Error("usage: format.mjs [--write]");
}
const write = options[0] === "--write";
const files = JSON.parse(await text(process.stdin));
const ignorePath = ".prettierignore";
readFileSync(ignorePath);
let checked = 0;
let failed = false;
for (const path of files) {
  const info = await getFileInfo(path, {
    ignorePath,
    resolveConfig: false,
    withNodeModules: true,
  });
  if (info.ignored || !info.inferredParser) {
    continue;
  }
  checked++;
  const original = readFileSync(path, "utf8");
  const formatted = await format(original, {
    filepath: path,
    proseWrap: "preserve",
  });
  if (original !== formatted && write) {
    writeFileSync(path, formatted);
  } else if (original !== formatted) {
    console.error(
      `${relative(process.cwd(), path).split(sep).join("/")}: formatting differs`,
    );
    failed = true;
  }
}
if (checked === 0) {
  throw new Error("no authored files supported by Prettier");
}
console.log(`${write ? "formatted" : "checked"} ${checked} formatted files`);
process.exitCode = failed ? 1 : 0;
