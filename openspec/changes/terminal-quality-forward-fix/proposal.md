## Why

The accepted product specification contains one surplus terminal blank line,
which violates the repository's canonical text-layout rule and blocks the
native architecture gate without changing product meaning.

## What Changes

- Remove the surplus terminal blank line from the product-control-plane spec.
- Preserve every requirement and scenario byte-for-byte otherwise.

## Capabilities

### Modified Capabilities

- `product-control-plane`: retain the existing contract in canonical text form.

## Impact

- **Authority:** repository text-layout policy remains the sole owner.
- **Breaking changes:** none.
- **Reuse:** the existing architecture verifier proves the repair.
- **Non-goals:** no product, CLI, dependency, release, or runtime behavior changes.
