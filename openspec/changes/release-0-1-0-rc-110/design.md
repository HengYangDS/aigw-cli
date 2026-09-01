## Context

See [proposal.md](proposal.md). Local `main`, `dev`, and `candidate/dev` share
the exact accepted product commit. `VERSION` owns product version;
`CHANGELOG.md` owns release chronology. GitLab and GitHub are independent peer
projections of one signed source and tag object.

## Goals / Non-Goals

**Goals:**

- Create one immutable `0.1.0-rc.110` source and tag identity.
- Reuse the existing proof, native-platform, packaging, publication,
  installation, rollback, and uninstall capabilities.
- Preserve exact Git objects and byte-identical assets across both Forges.

**Non-Goals:**

- Change setup, credential storage, provider routing, client projection, or
  transport behavior.
- Add release wrappers, compatibility paths, or Forge-specific product logic.

## Decisions

Advance only the existing version and changelog authorities, then use the
repository's current release lifecycle. A new release mechanism would create a
parallel authority without improving correctness.

Keep release-source acceptance separate from external effects. The signed
source object is immutable; each Forge, native platform, published asset set,
and installed runtime is observed independently.

## Risks / Trade-offs

- **A native platform differs from local macOS** -> require the existing
  macOS, Linux, and Windows acceptance for the exact release source.
- **One Forge completes before the other** -> retry only the incomplete peer;
  never rebuild or rewrite the signed object.
- **Installed acceptance fails** -> retain `0.1.0-rc.109` as the rollback
  target until setup, sync, verification, rollback, and uninstall pass.

## Migration Plan

Publish `0.1.0-rc.110` alongside `0.1.0-rc.109`, verify both Forge asset sets,
install the new release through the existing transactional lifecycle, exercise
rollback and re-upgrade, then verify uninstall leaves no owned residue. No
state-schema migration is introduced.
