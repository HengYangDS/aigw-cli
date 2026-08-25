## Why

Team setup already imports a token-free catalogue and accepts one connected
Account, but route fallback currently selects the first profile by identifier.
That loses the reviewed model recommendation when the preferred Provider is not
the connected Provider. A user who connects AIHubMix, for example, receives the
Luna profile even though the team manifest recommends Sol for Codex.

## What Changes

Preserve the reviewed client-and-model intent when selecting equivalent profiles
for the connected Account. Continue to permit token-free import, exactly one
connected Account, and deferred client installation. Verify the complete journey
with isolated host state and every supported secret-backend mode.

## Capabilities

### Modified Capabilities

- `product-control-plane`: route selection keeps the manifest's model intent
  while changing only the Provider Account needed to obtain a usable route.

## Impact

The configuration model and setup acceptance tests change. Provider credentials,
client private state, Proxy lifecycle, and external traffic remain outside this
Change.
