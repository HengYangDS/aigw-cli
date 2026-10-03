// Check locked Prettier layout and canonical Markdown spacing without writes.
import { text } from "node:stream/consumers";
import { readFileSync } from "node:fs";
import { relative, sep } from "node:path";
import { format, getFileInfo } from "../../node_modules/prettier/index.mjs";

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
  const markdown = info.inferredParser === "markdown";
  let formatted = await format(original, {
    filepath: path,
    embeddedLanguageFormatting: markdown ? "off" : "auto",
  });
  if (markdown) {
    const { fixMarkdownSpacing } = await import("./markdown/lint.mjs");
    formatted = await fixMarkdownSpacing(formatted);
  }
  if (original !== formatted) {
    console.error(
      `${relative(process.cwd(), path).split(sep).join("/")}: formatting differs`,
    );
    failed = true;
  }
}
if (checked === 0) {
  throw new Error("no authored files supported by Prettier");
}
console.log(`checked ${checked} formatted files`);
process.exitCode = failed ? 1 : 0;
