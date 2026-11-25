// Check that Renovate's policy holds the declared release age: the age at its root as the JSON5 parser
// reads it, zero for a security fix, a strict internal-checks filter, a behaviour that holds a release
// without a publish time, and no other rule that holds or admits releases.
// Usage: release-age.mjs "<hours> hours" < renovate.json5; exit 1 names each difference on stderr.
import { text } from "node:stream/consumers";
import { parsers } from "prettier/plugins/babel";

const declared = process.argv[2];
const { node } = await parsers.json5.parse(await text(process.stdin), {});
const found = {};
const problems = [];
const walk = (value, path) => {
  if (value?.type === "ObjectExpression") {
    for (const { key, value: inner } of value.properties) {
      const name = key.name ?? key.value;
      if (
        name.startsWith("minimumReleaseAge") ||
        ["extends", "stabilityDays", "internalChecksFilter"].includes(name)
      ) {
        if (`${path}/${name}` in found) problems.push(`release_age_declared_twice:${path}/${name}`);
        found[`${path}/${name}`] = inner.value ?? null;
      } else {
        walk(inner, `${path}/${name}`);
      }
    }
  } else if (value?.type === "ArrayExpression") {
    for (const inner of value.elements) walk(inner, `${path}[]`);
  }
};
walk(node, "");

const admitted = [
  "/internalChecksFilter",
  "/minimumReleaseAge",
  "/minimumReleaseAgeBehaviour",
  "/vulnerabilityAlerts/minimumReleaseAge",
];
if (found["/internalChecksFilter"] !== "strict")
  problems.push("release_age_internal_checks_not_strict");
if (found["/minimumReleaseAgeBehaviour"] !== "timestamp-required") {
  problems.push("release_age_behaviour_not_timestamp_required");
}
if (found["/minimumReleaseAge"] !== declared) {
  problems.push(
    `release_age_differs:/minimumReleaseAge reads ${JSON.stringify(found["/minimumReleaseAge"])}, declared ${JSON.stringify(declared)}`,
  );
}
if (found["/vulnerabilityAlerts/minimumReleaseAge"] !== "0 hours") {
  problems.push("release_age_security_fix_not_skipped:/vulnerabilityAlerts/minimumReleaseAge");
}
for (const path of Object.keys(found)) {
  if (!admitted.includes(path)) problems.push(`release_age_other_rule:${path}`);
}
if (problems.length > 0) {
  process.stderr.write(`${problems.join("\n")}\n`);
  process.exitCode = 1;
}
