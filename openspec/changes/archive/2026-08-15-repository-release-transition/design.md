## Context

`dev` and `candidate/dev` contain the verified source. `main` remains behind because the adopted branch-role policy exposes no release edge.

## Decision

Track `.ethos/workspace.toml` as the sole branch-role policy and declare `accepted-to-release` with `proof:execution` as its required evidence.

## Rejected Alternatives

- Raw Git fast-forward: bypasses the adopted public lifecycle.
- A second release script: duplicates transition authority.
