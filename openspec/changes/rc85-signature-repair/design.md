# Design

## Decision

Treat commit identity as publication metadata without changing the product
history's semantic payload. Build one complete replacement suffix from the
published GitLab rc.84 tip, then let the public ETHOS identity-repair command
compare the old and replacement DAGs and mint the only executable receipt.

The receipt must fail closed unless every replacement preserves the original
tree, message, author, timestamps, order, and parent topology. It must also bind
the observed work, candidate, development, and release refs with exact CAS.

## Verification

1. Execute the complete repository proof on the clean repair Change.
2. Archive and re-prove the Change without changing product source.
3. Verify the replacement mapping and immutable receipt before authorization.
4. Apply the receipt once and prove the complete GitLab commit provenance.
5. Re-run the exact-HEAD repository proof before lifecycle integration.
