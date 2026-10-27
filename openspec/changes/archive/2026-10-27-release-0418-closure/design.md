# Design

## Context

Release 0.4.18 ported the maintenance branch's behavior and its forwarding
store, release-input, carrier and provenance tests; its design lists what was
not ported by decision. Two preview tests and the `projection-format` delta
from the audit's item 1 were neither ported nor decided. The first real-client
journey with a baseline that already reads retry policies exposed a journey
assumption, and 0.4.18's preflight verdict change reached Unix only.

## Goals / Non-Goals

**Goals:** close the remaining audited obligations so that the branch ref can
be retired under the user's rule, let the real-client journey accept any
published baseline, and give Windows benchmark preflight the verdict that Unix
already has.

**Non-Goals:** product behavior changes, a release, or porting what release-0418
already decided against.

## Decisions

### Port the preview tests to the existing harness

Both tests move into `internal/cli/selection` and use its `configuredRuntime`,
`staticDiscovery` and memory credential store; a store that fails on `Get`
proves that a preview reads no credential. `dev`'s `use --json` result names
its plans `targets`, where the branch named them `projections`, and names the
target's real path while the binding keeps the discovered path, so the second
test compares files rather than strings. The refusal test requires the typed
Codex conflict that 0.4.18 introduced. Each test fails when its guard is
reverted locally: the managed-block hash check for the refusal, and the
planning of discovered Codex homes for the new target.

### State forwarding in `projection-format`

The requirement "Projection conflicts are rejected before configuration
writes" gains one sentence and two scenarios, worded from the branch delta and
checked against `dev`'s tests:

- the forwarding sidecar shares the snapshot, backup, checkpoint and
  compensation boundary and leaves the predecessor-readable main schema
  unchanged (`TestForwardingSnapshotRestoresPresenceAndPriorBinding`,
  `TestForwardingWriteFailureCompensatesOwnedFilesAndPreservesNewerData`,
  `TestForwardingBackupConvergencePreservesCurrentGeneration`,
  `TestForwardingStorePreservesStrictPublishedConfiguration`);
- "A retained credential reader observes forwarding"
  (`TestForwardedCheckpointKeepsTheShapeARetainedReaderDecodes`, and the hosted
  `TestNativePublishedPredecessorForwardingJourney`);
- "Forwarding changes during verification or commit"
  (`TestForwardingChangedDuringVerificationRejectsCheckpoint`,
  `TestForwardingChangeDuringCheckpointWritePreservesNewerState`,
  `TestForwardingUnchangedCanonicalRejectsStalePreimage`).

The branch scenario "Restore direct mode before predecessor rollback" stays out,
as release-0418 decided: 0.4.17 reads forwarding, and rollback while forwarded
is already tested.

### The retry journey expects the baseline's own boundary

The product already decides by probing the retained program's reader, as
`route-client-selection` requires, not by a version threshold. The journey
needs an independent expectation, and the release history supplies one:
`--request-max-retries` first shipped in 0.4.17. The journey reads the
baseline's reported version; before 0.4.17 it requires the existing refusal
with both programs unchanged. From 0.4.17 on it requires the rollback to
succeed with the policy selected and the active program to become the baseline,
then rolls back again to restore the candidate, which proves that the candidate
reads the same declaration. A probe of the baseline's own configuration export
was rejected: it would repeat the product's decision instead of checking it.

### Windows retains the same preflight verdict as Unix

Windows keeps its observed single Hyperfine run, which supplies the process
tree, images and controller that only Windows records. Its preparation and
observed streams are now written through the same `writePreflightStreams` that
Unix uses, as `.preflight` streams, and a product diagnostic is no longer an
error: `Measure` reads it from the retained stderr and marks the workload, so
the remaining workloads are still measured. A failed preparation, a rejected
process image or an unwritable output still stops the preflight. Hyperfine
2.0.0 runs the command once here, without warmup, and writes nothing to stderr
itself, so the retained stderr is the product's own. Windows behavior is
accepted only by the hosted Windows run; locally the package builds and vets
for Windows.

### Retire the branch ref natively

The ref is retired through ETHOS publication retirement after this Change is
delivered. If every native route still refuses, the ref stays, recorded as
absorbed, and the user decides; nothing bypasses ETHOS or the pre-push hook.

## Risks / Trade-offs

- **The ported tests pin current behavior.** They add no new guard; their value
  is that each now fails when its guard is reverted.
- **The journey's version oracle is fixed history.** It names the release that
  first read retry policies and changes only if that history is rewritten.
