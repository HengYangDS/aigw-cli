// Lint the exact repository inventory using only its declared native rules.
import { text } from "node:stream/consumers";
import { accessSync, readFileSync } from "node:fs";

// Bare package resolution must not substitute a parent checkout's installation.
for (const name of ["markdownlint", "js-yaml"]) {
  accessSync(new URL(`../../../node_modules/${name}/package.json`, import.meta.url));
}
const { default: helpers } = await import("markdownlint/helpers");
const { lint } = await import("markdownlint/promise");
const { load: yaml } = await import("js-yaml");

const carrier = yaml(
  readFileSync(".config/checks/markdownlint-cli2/.markdownlint-cli2.yaml", "utf8"),
);
if (
  !carrier ||
  typeof carrier !== "object" ||
  Array.isArray(carrier) ||
  Object.keys(carrier).length !== 1 ||
  !Object.hasOwn(carrier, "config") ||
  !carrier.config ||
  typeof carrier.config !== "object" ||
  Array.isArray(carrier.config)
) {
  throw new Error("Markdown native carrier must contain only its config rules");
}
const config = carrier.config;
const files = JSON.parse(await text(process.stdin));
const results = await lint({ files, config, noInlineConfig: true });
const diagnostics = helpers.formatLintResults(results).join("\n");
if (diagnostics) console.error(diagnostics);
console.log(`checked ${Object.keys(results).length} Markdown files`);
process.exitCode = diagnostics ? 1 : 0;
