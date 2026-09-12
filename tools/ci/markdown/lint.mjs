// Lint the exact repository inventory using only its declared native rules.
import { text } from "node:stream/consumers";
import { accessSync } from "node:fs";

// Bare package resolution must not substitute a parent checkout's installation.
accessSync(
  new URL(
    "../../../node_modules/markdownlint-cli2/package.json",
    import.meta.url,
  ),
);
const { default: helpers } =
  await import("markdownlint-cli2/markdownlint/helpers");
const { lint, readConfig } =
  await import("markdownlint-cli2/markdownlint/promise");
const { default: yaml } = await import("markdownlint-cli2/parsers/yaml");

const files = JSON.parse(await text(process.stdin));
const config = await readConfig(".config/checks/markdown/policy.yaml", [yaml]);
const results = await lint({ files, config, noInlineConfig: true });
const diagnostics = helpers.formatLintResults(results).join("\n");
if (diagnostics) {
  console.error(diagnostics);
}
console.log(`checked ${Object.keys(results).length} Markdown files`);
process.exitCode = diagnostics ? 1 : 0;
