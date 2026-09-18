## Context

The repository already owns dependency refresh through native ecosystem
manifests and resolvers. The newly published Renovate release changes one direct
OCI input; repeating the prior broad refactor would add churn without value.

## Decisions

- Keep `mise.toml` as the sole owner of the Renovate image reference.
- Select the current stable tag and its immutable multi-platform digest.
- Preserve every other direct pin unless its authoritative source reports a
  newer compatible stable release.
- Require repeated resolution to be byte-clean and use existing checks rather
  than adding another supply-chain mechanism.

## Verification

1. Re-query official registries for every direct dependency family.
2. Refresh the one stale declaration and its native lock projection.
3. Run bootstrap, dependency checks, source verification, native acceptance,
   and release construction.
4. Re-query the final graph and verify the Work Lane is clean except for the
   intended closure.
