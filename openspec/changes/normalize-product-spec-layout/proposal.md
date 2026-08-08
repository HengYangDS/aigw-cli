## Why

The canonical product-control-plane specification violates the repository's
terminal text-layout rule and therefore blocks exact proof.

## What Changes

- Remove the extra terminal blank line without changing specification meaning.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. This change is formatting-only and declares `skip_specs: true`.

## Impact

Only the canonical product-control-plane specification bytes change. Product
behavior, APIs, dependencies, authority boundaries, and publication remain
unchanged; no compatibility or parallel enforcement surface is introduced.
