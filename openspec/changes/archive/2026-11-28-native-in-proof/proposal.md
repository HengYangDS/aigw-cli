## Why

Local verification of a code Change runs the repository's Go tests twice on this host: once in the exact-HEAD proof,
whose behavior gate is the quality graph's coverage command, and once more in `mise run native`, whose coverage run
adds only the host's native resource tests (`-tags=native_resource_acceptance`, one file in `tools/release`) and runs in
native acceptance's isolated environment. The owner requires the local proof to run every check that a remote runs
(2026-11-25), so the native run cannot be dropped; it can join the proof. On 2026-11-28 the proof's behavior gate took
147 s and `mise run native` 4 min 56 s in its own host ticket, so each code Change waited in the host queue twice.

## What Changes

- The quality graph's coverage gate runs the host platform's native test attempt: the coverage command with the native
  resource tests, in native acceptance's isolated environment, wrapped as native acceptance wraps it on Linux. One
  repository command, `go run ./tools/ci native-tests`, owns that attempt; the coverage gate, the exact-HEAD proof's
  behavior gate (with its two reports) and `mise run native` all run it.
- The quality graph's `native-resource-fixture` gate goes: it runs one test of the native resource file, which that
  attempt now runs in the proof and in both hosted native jobs, the Linux one on the platform of the hosted quality job.
- `mise run native` keeps its meaning for a contributor and for the hosted native jobs; locally it adds nothing that the
  check and the proof do not run.

Not breaking: no product behavior, command, file or output changes.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `quantitative-quality`: the one coverage attempt that the proof judges includes the proving host's native resource
  tests, so no separate local native run repeats it.

## Impact

`tools/ci` (the native test attempt, the coverage gate and the fixture gate), `.ethos/profile.toml` (the behavior gate's command), their
tests, `CONTRIBUTING.md`'s description of native acceptance, and the accepted specification above. Authority boundary:
the hosted native jobs, their platforms and their selection-matrix runs do not change. Reuse: the native test command,
its isolated environment and its Linux session wrapper already exist; they get one owner. Non-goals: overlapping the
proof's gates, which a later Change takes up after the ETHOS rebind; the hosted jobs' repeated runs of one commit.
