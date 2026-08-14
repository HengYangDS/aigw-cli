## Decision

The immutable container remains a reproducible base, not a version authority.
The bootstrap reads `min_version` from `mise.toml`, downloads that official
installer by exact release URL, and installs it before locked execution. It
does not call release-discovery APIs or introduce a second version literal.
The CUE projection owns this container bootstrap directly; the repository does
not add a shell program beside its portable Go automation.

Native jobs declare only `go` through mise's positive tool-selection setting.
Source and release jobs retain their broader repository-declared tool graph.

## Verification

1. Projection tests require exact-version bootstrap and native tool scope.
2. A quota-exhausted Linux container reaches locked Go execution.
3. Projection reconciliation and the complete source graph pass before land.
