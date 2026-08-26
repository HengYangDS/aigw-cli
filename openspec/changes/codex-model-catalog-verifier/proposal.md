## Why

Codex removed the private configuration-lockfile export consumed by AIGW's
catalog verifier. The projection still works, but the verifier now fails before
it can observe the client, so it cannot qualify current Codex releases.

## What Changes

- Replace prompt-shape inference with the public `codex debug models` catalog
  surface.
- Observe the bundled catalog and an isolated effective catalog separately.
- Prove that the generated alias is present and metadata-identical to its base
  entry apart from `slug`, while unrelated unknown entries remain absent.
- Preserve exact client version and executable digest evidence without reading
  the user's Codex Home or sending a model request.
- Delete the removed config-lockfile and prompt item-count semantics.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `product-control-plane`: Qualify model-catalog projection through a current,
  public, isolated Codex observation surface.

## Impact

The change is limited to the Codex catalog verifier, its developer command,
documentation, tests, and specification. It does not change provider routing,
wire model IDs, credentials, client installation, sessions, or proxy lifecycle.
