## Why

GitLab Linux jobs execute inside a root-owned container. Tests that model I/O
failure by clearing POSIX write bits therefore succeed instead of reaching the
error path, even though the product behavior is correct.

## What Changes

- Express filesystem failures with deterministic objects or narrow internal
  operations rather than runner privilege.
- Remove the duplicated high-level permission test where the same atomic-write
  stage is already proved by its owning package.
- Preserve the public filesystem transaction contract and error messages.

## Impact

- **Runtime:** no public behavior change.
- **Portability:** focused tests become stable for root and non-root runners.
- **Authority:** the locked repository test graph remains the acceptance owner.
