## Context

Provider-specific history projection rewrites identity and signatures. Distinct source commits with equal semantic inputs can become the same target commit. Therefore raw commit-count or ordered tree-occurrence equality is not an invariant of the projection. The product invariant needed by release and synchronization is exact source-tree equality at each accepted branch tip, with complete provider-native signature verification enforced separately.

The GitLab bootstrap already derives the exact mise version from `mise.toml`. Its remaining failure is transport-level: the upstream installer uses curl without protocol or retry controls.

## Decisions

### Forge parity

- `tree` parity compares the exact tip tree object.
- `commit` parity continues to require the exact commit object.
- provider projection collapses duplicate mapped parents while preserving the first-occurrence order.
- provenance and tag verification remain independent requirements and are not inferred from tree parity.

### CI bootstrap

- `.config/ci/pipeline.cue` remains the sole CI topology and command authority.
- the exact-version bootstrap forces HTTP/1.1 and bounded retries for the installer and the installer's asset fetch.
- no second shell script or package-manager bootstrap stack is introduced.

## Rejected alternatives

- Ordered raw tree-history equality: rejects correct deterministic collapse.
- Commit-count equality: not invariant under identity normalization.
- GitHub API lookup: creates cross-Forge coupling and anonymous rate-limit exposure.
- A parallel bootstrap script: duplicates the CUE-owned command surface.
