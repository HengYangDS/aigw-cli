# Normalize product-control specification text

## Why

The archived rc.80 projection left one blank line after the canonical
product-control-plane specification. The repository text-layout contract
correctly rejects that non-semantic artifact.

## What changes

- remove the trailing blank line from the canonical specification;
- prove that architecture, portability, and governance accept the normalized
  text without changing product behavior.

## Out of scope

- product, provider, client, release, or dependency changes;
- compatibility behavior or a new formatting mechanism.
