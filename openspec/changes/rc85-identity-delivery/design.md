# Design

## Decision

Build an unreachable signed replacement DAG, verify complete-DAG semantic
equivalence, then let `ethos lane repair-identity` derive and apply the sole
executable receipt. The receipt binds the current Work Lane, Lease, index,
overlay, proof, trusted signatures, and all local role refs.

## Sequence

1. Complete and prove this delivery Change.
2. Recreate the suffix with identical unsigned commit payloads and trusted
   signatures.
3. Derive the immutable replacement receipt and verify its mapping.
4. Re-read all coordinates, then apply the same receipt once.
5. Verify local roles, publish Forge-native projections, and retire temporary
   lanes only after delivery acceptance.

Any semantic, proof, signature, Lease, index, overlay, or ref drift aborts
before mutation.
