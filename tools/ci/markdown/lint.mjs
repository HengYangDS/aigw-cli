// Lint the exact repository inventory using only its declared native rules.
import { text } from "node:stream/consumers";
import { accessSync } from "node:fs";

// Bare package resolution must not substitute a parent checkout's installation.
for (const name of ["markdownlint", "js-yaml"]) {
  accessSync(
    new URL(`../../../node_modules/${name}/package.json`, import.meta.url),
  );
}
const { default: helpers } = await import("markdownlint/helpers");
const { lint, readConfig } = await import("markdownlint/promise");
const { load: yaml } = await import("js-yaml");

// Upstream blank-line rules leave single-paragraph peer spacing unconstrained.
const spacingTokens = new Set([
  "lineEnding",
  "lineEndingBlank",
  "listItemIndent",
  "blockQuotePrefix",
  "linePrefix",
]);
const singleParagraphListSpacing = {
  names: ["single-paragraph-list-spacing"],
  description: "Single-paragraph list items have no blank separators",
  tags: ["blank_lines"],
  parser: "micromark",
  function(params, onError) {
    const pending = [...params.parsers.micromark.tokens];
    while (pending.length) {
      const list = pending.pop();
      pending.push(...list.children);
      if (list.type !== "listOrdered" && list.type !== "listUnordered")
        continue;
      const items = [];
      for (const child of list.children) {
        if (child.type === "listItemPrefix") items.push([]);
        else if (items.length && !spacingTokens.has(child.type))
          items.at(-1).push(child);
      }
      if (
        items.length < 2 ||
        !items.every(
          (item) =>
            item.length === 1 &&
            item[0].type === "content" &&
            item[0].children.length === 1 &&
            item[0].children[0].type === "paragraph",
        )
      )
        continue;
      for (let index = 1; index < items.length; index++) {
        const lineNumber = items[index - 1][0].endLine + 1;
        if (lineNumber < items[index][0].startLine)
          onError({
            lineNumber,
            detail:
              "Remove the blank separator between single-paragraph items.",
            fixInfo: { lineNumber, deleteCount: -1 },
          });
      }
    }
  },
};

const files = JSON.parse(await text(process.stdin));
const config = await readConfig(".config/checks/markdown/policy.yaml", [yaml]);
const results = await lint({
  files,
  config,
  noInlineConfig: true,
  customRules: [singleParagraphListSpacing],
});
const diagnostics = helpers.formatLintResults(results).join("\n");
if (diagnostics) {
  console.error(diagnostics);
}
console.log(`checked ${Object.keys(results).length} Markdown files`);
process.exitCode = diagnostics ? 1 : 0;
