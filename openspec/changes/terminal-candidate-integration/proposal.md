# Terminal candidate integration

## Why

The work lane contains proven, archived changes not yet represented by
`candidate/dev`. Candidate integration therefore needs one explicit authority
bound to the lane's complete accumulated delta.

## What changes

- Bind the exact archived lane tree to candidate integration authority.
- Permit only an exact compare-and-swap update of local `candidate/dev`.

## Non-goals

- No product, provider, protocol, dependency, documentation, or release change.
- No remote publication or accepted-root mutation.
