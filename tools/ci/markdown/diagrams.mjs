// Validate the exact source inventory supplied by the repository command.
import { text } from "node:stream/consumers";
import { readFileSync } from "node:fs";
import {
  blockToDiagnostics,
  extractMermaidBlocks,
  selectFailures,
} from "@mermaid-lint/core";

const paths = JSON.parse(await text(process.stdin));
let count = 0;
let failed = false;
for (const path of paths) {
  for (const block of extractMermaidBlocks(path, readFileSync(path, "utf8"))) {
    count++;
    for (const diagnostic of selectFailures(
      await blockToDiagnostics(block),
      true,
    )) {
      console.error(`${path}:${diagnostic.line}: ${diagnostic.message}`);
      failed = true;
    }
  }
}
console.log(`checked ${count} diagrams in ${paths.length} files`);
process.exitCode = failed ? 1 : 0;
