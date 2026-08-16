# Design

## Rule Authority

Rules are classified by the property they can prove:

| Class | Merge authority | Required basis |
| --- | --- | --- |
| Product invariant | Blocking | Observable semantic violation and one owner |
| Risk threshold | Blocking only when admitted | Risk model, exact metric, repair path, review condition |
| Review heuristic | Non-blocking | Descriptive evidence for human or agent review |
| Historical ratchet | Temporary | Named debt, owner, target, and exit condition |

The architecture policy remains the single machine-readable owner of positive
package and dependency contracts. The checker reports exact semantic findings;
it no longer implements repository-specific size mathematics.

## Preserved Invariants

Package topology, import direction, composition roots, peer-package isolation,
module identity, portability, semantic naming, package documentation, aliases,
and forwarding wrappers remain blocking because each maps to a stable ownership
or portability failure. Coverage remains independently governed by the coverage
policy and is unchanged by this change.

## Removed Accidental Complexity

The private ELOC, decision-complexity, nesting, directory aggregation,
suffix-group, and ratchet implementations are deleted with their policy fields
and tests. No compatibility parser accepts the retired fields.

## Verification

Focused architecture tests prove the retained semantic contracts and reject
retired policy fields through the existing strict policy parser. The complete
source graph then proves behavior, static analysis, formatting, coverage,
governance, and portability on the same candidate tree.
