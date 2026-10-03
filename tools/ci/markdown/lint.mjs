// Lint the exact repository inventory using only its declared native rules.
import { text } from "node:stream/consumers";
import { accessSync } from "node:fs";
import { fileURLToPath } from "node:url";

// Bare package resolution must not substitute a parent checkout's installation.
for (const name of ["markdownlint", "js-yaml"]) {
  accessSync(
    new URL(`../../../node_modules/${name}/package.json`, import.meta.url),
  );
}
const { default: helpers } = await import("markdownlint/helpers");
const { applyFixes } = await import("markdownlint");
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

const config = await readConfig(".config/checks/markdown/policy.yaml", [yaml]);
const spacingRules = new Set([
  "MD012",
  "MD022",
  "MD031",
  "MD032",
  "MD047",
  "MD058",
  "single-paragraph-list-spacing",
]);

function lintMarkdown(input) {
  return lint({
    ...input,
    config,
    noInlineConfig: true,
    customRules: [singleParagraphListSpacing],
  });
}

// Apply only whitespace fixes; structural repairs are not formatting authority.
export async function fixMarkdownSpacing(content) {
  const results = await lintMarkdown({ strings: { content } });
  return applyFixes(
    content,
    results.content.filter((issue) => spacingRules.has(issue.ruleNames[0])),
  );
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const files = JSON.parse(await text(process.stdin));
  const results = await lintMarkdown({ files });
  const diagnostics = helpers.formatLintResults(results).join("\n");
  if (diagnostics) console.error(diagnostics);
  console.log(`checked ${Object.keys(results).length} Markdown files`);
  process.exitCode = diagnostics ? 1 : 0;
}
