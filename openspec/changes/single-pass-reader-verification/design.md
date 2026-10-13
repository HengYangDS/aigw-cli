# Design

## Current cost

One `sync` hashes the running program to derive its versioned reader path,
hashes the reader in the projection plan, hashes the reader again while
ensuring it, and hashes the program again to confirm the reader matches it.
`TestExecutableDigestRechecksConcurrentSources` deliberately forbids reusing a
digest by file identity, because Linux and Windows timestamps are too coarse to
reveal a same-size rewrite.

## Direction

Carry verified identity within one command instead of re-reading files: derive
the program digest once, verify the reader once from the handle that the
projection then copies or keeps, and pass both results to later checks in the
same transaction. A later phase that needs fresh evidence reads again on
purpose. Measure the result with the existing native performance contract.
