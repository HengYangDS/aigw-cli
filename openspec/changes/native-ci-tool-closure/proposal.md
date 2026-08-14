## Why

Native acceptance executes the Go CI driver, which invokes CUE-backed
repository checks. Limiting mise to Go therefore produces a deterministic
`cue: executable file not found` failure on clean hosted runners.

## What Changes

- Declare the native command's exact runtime closure as Go and CUE.
- Regenerate both Forge projections from the existing CUE authority.
- Bind the closure to a focused projection contract.

## Boundary

This Change repairs repository CI only. It does not weaken native platform
coverage, repair a runner host, publish a release, or add a second tool owner.
