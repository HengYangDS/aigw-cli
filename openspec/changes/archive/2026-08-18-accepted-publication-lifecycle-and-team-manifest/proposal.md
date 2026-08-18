## Why

The accepted branches carried completed active Changes because source CI only
validated OpenSpec syntax. The repository also exposed a fictitious manifest
instead of the reviewed token-free team configuration the product exists to
distribute.

## What Changes

- Admit accepted publication trees only when every OpenSpec Change is archived.
- Replace the fictitious manifest with the reviewed token-free team manifest.
- Keep format examples in documentation and tests instead of a second manifest.

## Capabilities

### Modified Capabilities

- `product-control-plane`: publish one usable team configuration manifest.
- `product-quality`: enforce the accepted-tree OpenSpec lifecycle boundary.

## Impact

The change affects repository admission, source CI, team onboarding documents,
the configuration manifest, and their tests. It adds no runtime dependency,
credential, provider-specific code path, or compatibility surface.
