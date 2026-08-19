## Why

AIGW currently rebuilds Git history and release tags for each Forge, so GitLab
and GitHub expose different product objects and one peer becomes an input to the
other. That contradicts local-first authority and multiplies signing, repair,
CI, documentation, and release semantics.

## What Changes

- **BREAKING** make one locally created, signed commit and annotated tag the
  only product Git objects;
- publish those exact objects independently to zero, one, or two optional Forge
  peers;
- **BREAKING** delete history replay, provider identity rewriting,
  provider-qualified tags, tree-only parity, cross-peer synchronization, and
  the repository-owned `dev` to `main` lifecycle command;
- separate product-object signing from each peer's transport authentication and
  hosted account-verification display;
- require exact remote observations and compare-and-swap leases for divergent
  one-time cutovers;
- align specifications, decisions, documentation, CI, and tests with the single
  authority model.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `product-control-plane`: replace Forge-specific histories with exact local
  object publication and remove the duplicate local release transition owner.
- `product-quality`: require exact commit and tag identity across independent
  publication peers.

## Impact

The breaking replacement affects `tools/forge`, CI provenance invocations,
release operations, governance documentation, decision records, and canonical
OpenSpec requirements. It does not change provider routing, client projection,
credentials, API traffic, Codex session state, or external gateway lifecycle.
