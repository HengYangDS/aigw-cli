## Why

Accepted `dev` is the local product truth, while release `main` can remain
behind it. Remote projection cannot safely repair that local gap: GitLab and
GitHub are independent publication planes, and neither owns local release
promotion.

## What Changes

- Add one exact-CAS local `dev` to `main` release-promotion command.
- Keep GitHub identity projection separate and atomic across `main` and `dev`.
- Reject stale coordinates, dirty state, divergence, candidate branches, and
  work branches before any ref moves.

## Capabilities

### Modified Capabilities

- `product-control-plane`: define one local release-root transaction before
  independent Forge publication.

## Impact

- **Authority:** accepted `dev` supplies the release source; local `main` is its
  explicit release projection.
- **Breaking changes:** none to the AIGW user CLI or configuration model.
- **Non-goals:** no remote push, tag, release, Proxy lifecycle, workstation
  state, or conversation state is added to local promotion.
