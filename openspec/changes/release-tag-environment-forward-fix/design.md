## Context

The repository check correctly reads Forge tag variables when validating an
actual release. The defect is test-process scope: unrelated fixture tests run
inside the same process and inherit those ambient variables.

## Goals / Non-Goals

**Goals:**

- Make the generic test suite hermetic under GitHub and GitLab tag jobs.
- Keep dedicated tag-provenance tests explicit and complete.
- Produce a forward-only release without rewriting rc.80 evidence.

**Non-Goals:**

- Change production tag selection or Changelog validation.
- Add a second release-policy owner, wrapper, or Forge-specific code path.

## Decisions

1. Clear ambient tag selection once in the `tools/repository` test entrypoint.
   Individual provenance tests retain authority by setting complete inputs with
   `t.Setenv`. This centralizes fixture isolation without weakening production
   validation.
2. Prove the regression by running the package with GitHub and GitLab tag
   variables set. A unit test that only repeats `t.Setenv` would not reproduce
   process-level inheritance.
3. Advance to rc.81. Published rc.80 objects and failed runs remain immutable;
   a new release is the only SemVer-safe correction.

## Risks / Trade-offs

- **An intended tag test could accidentally rely on ambient state** -> dedicated
  provenance tests must set every required variable explicitly.
- **Local success could again differ from Forge execution** -> the focused gate
  runs with tag-shaped process variables before full proof and publication.

## Migration Plan

1. Add the failing inherited-environment regression.
2. Add one test-entrypoint isolation mechanism and rerun the focused package.
3. Advance release metadata, execute the full exact-HEAD proof, and publish
   signed rc.81 objects independently on GitLab and GitHub.
