# Tasks

## 1. Repair the first hosted execution

- [x] 1.1 Clear the mise image entrypoint in the GitLab projection and add a projection regression.
- [x] 1.2 Isolate source-sequence tests from inherited Forge variables.
- [x] 1.3 Keep the Windows job bound only to its platform selector; retain no runner host identity or path in source.

## 2. Prove one source tree

- [x] 2.1 Regenerate and verify all Forge projections from the CUE authority.
- [x] 2.2 Pass focused tests and the complete local quality graph.
- [x] 2.3 Define exact-HEAD proof, candidate integration, and accepted closeout as post-Change lifecycle effects.

## Delivery boundary

Exact-HEAD proof, Change archive, candidate integration, accepted-head hosted
CI, runner-host repair, independent Forge publication, installation, runtime
acceptance, and lane retirement are post-Change lifecycle effects. Their
public command and Forge receipts own those facts; making them pre-archive
tasks would require the Change to be accepted before it could be completed.

## Requirement To Task To Proof

| Requirement | Task | Proof |
| --- | --- | --- |
| `product-quality:one complete quality graph` | `1.1` | `cue-projection-and-entrypoint-regression` |
| `product-quality:one complete quality graph` | `1.2` | `isolated-source-sequence-regression` |
| `product-quality:one complete quality graph` | `1.3` | `platform-selector-without-host-identity` |
| `product-quality:one complete quality graph` | `2.1` | `projection-generation-and-drift-check` |
| `product-quality:one complete quality graph` | `2.2` | `focused-tests-and-complete-source-graph` |
| `product-quality:one complete quality graph` | `2.3` | `post-change-lifecycle-boundary` |
