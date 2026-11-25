import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import path from "node:path";
import test from "node:test";

const reader = path.resolve(import.meta.dirname, "../release-age.mjs");

// A policy that holds its root age at zero, holds a release inside the age and
// without a publish time, and skips the age for a security fix.
const policy = `{
  minimumReleaseAge: "0 hours",
  minimumReleaseAgeBehaviour: "timestamp-required",
  internalChecksFilter: "strict",
  vulnerabilityAlerts: {
    minimumReleaseAge: "0 hours",
  },
}
`;

function read(text, declared = "0 hours") {
  const result = spawnSync(process.execPath, [reader, declared], {
    input: text,
    encoding: "utf8",
    timeout: 15_000,
  });
  assert.equal(result.error, undefined);
  return { status: result.status, output: result.stdout + result.stderr };
}

function refuses(text, expected, declared) {
  const { status, output } = read(text, declared);
  assert.equal(status, 1, output);
  assert.ok(output.includes(expected), `${output}\nwant it to hold ${expected}`);
}

test("the reader accepts a policy that holds the declared age", () => {
  assert.deepEqual(read(policy), { status: 0, output: "" });
});

test("the reader reads quoted keys and comments as the JSON5 parser does", () => {
  const quoted = `// the policy's ages\n${policy.replace("minimumReleaseAge:", '"minimumReleaseAge":')}`;
  assert.deepEqual(read(quoted), { status: 0, output: "" });
});

test("the reader names a root age other than the declared one", () => {
  refuses(policy, "release_age_differs:/minimumReleaseAge", "24 hours");
});

test("the reader names a security fix that the age holds", () => {
  refuses(
    policy.replace('    minimumReleaseAge: "0 hours"', '    minimumReleaseAge: "24 hours"'),
    "release_age_security_fix_not_skipped",
  );
});

test("the reader names an age of a rule's own", () => {
  refuses(
    policy.replace(
      "  vulnerabilityAlerts: {",
      '  packageRules: [{ minimumReleaseAge: "72 hours" }],\n  vulnerabilityAlerts: {',
    ),
    "release_age_other_rule:/packageRules[]/minimumReleaseAge",
  );
});

test("the reader names a preset, which can bring an age of its own", () => {
  refuses(
    policy.replace(
      "  vulnerabilityAlerts: {",
      '  extends: ["config:recommended"],\n  vulnerabilityAlerts: {',
    ),
    "release_age_other_rule:/extends",
  );
});

test("the reader names a behaviour that does not hold a release without a time", () => {
  refuses(
    policy.replace('"timestamp-required"', '"timestamp-optional"'),
    "release_age_behaviour_not_timestamp_required",
  );
});

test("the reader names checks that do not hold a release inside the age", () => {
  refuses(
    policy.replace('internalChecksFilter: "strict"', 'internalChecksFilter: "none"'),
    "release_age_internal_checks_not_strict",
  );
});

test("the reader names a key declared twice", () => {
  refuses(
    policy.replace(
      '  internalChecksFilter: "strict",\n',
      '  internalChecksFilter: "strict",\n  minimumReleaseAge: "0 hours",\n',
    ),
    "release_age_declared_twice:/minimumReleaseAge",
  );
});
