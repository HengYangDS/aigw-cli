## Context

See [proposal.md](proposal.md). Local `main`, `dev`, and `candidate/dev` share
the exact accepted onboarding commit. `VERSION` is the product-version SSOT;
`CHANGELOG.md` is the release chronicle. GitLab and GitHub are independent
projections of one signed tag and one asset set.

## Goals / Non-Goals

**Goals:**

- Create one immutable rc.105 source identity from the accepted product.
- Reuse the existing release, signing, CI, installation, and rollback paths.
- Preserve byte-identical source and release assets across both Forges.

**Non-Goals:**

- Change onboarding behavior, credentials, client projection, or API traffic.
- Add a release wrapper, compatibility path, or Forge-specific product logic.

## Decisions

Advance only `VERSION` and `CHANGELOG.md`, then use the repository's existing
source gate, native acceptance, signed-tag, and dual-Forge publication flow.
This keeps release identity separate from behavior and leaves each semantic
owner singular.

## Risks / Trade-offs

- **Hosted platform behavior differs from local macOS** -> require the existing
  native Linux and Windows jobs before treating the release as complete.
- **One Forge succeeds before the other** -> verify each peer independently and
  publish only the same signed object and checksummed asset set.
