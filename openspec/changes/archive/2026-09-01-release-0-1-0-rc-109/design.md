## Context

See [proposal.md](proposal.md). Local `main`, `dev`, and `candidate/dev` share
the exact accepted credential-observation commit. `VERSION` owns product
version; `CHANGELOG.md` owns release chronology. GitLab and GitHub are peer
projections of one signed source and tag object.

## Goals / Non-Goals

**Goals:**

- Create one immutable rc.109 source identity from the accepted product.
- Reuse the existing source, native-platform, packaging, signing,
  publication, installation, rollback, and uninstall capabilities.
- Preserve exact Git objects and byte-identical assets across both Forges.

**Non-Goals:**

- Change credential storage, Account selection, client projection, or API
  transport behavior.
- Add release wrappers, compatibility paths, Proxy coupling, or
  Forge-specific product logic.

## Decisions

Advance only the existing version and changelog authorities, then use the
repository's existing release lifecycle. A new release mechanism would add a
parallel authority without improving correctness.

Release source acceptance and external publication remain separate effects:
the signed source object is immutable, while each Forge, native platform, and
installed runtime is observed independently.

## Risks / Trade-offs

- **A native platform differs from local macOS** -> require the existing
  macOS, Linux, and Windows acceptance for the exact release source.
- **One Forge completes before the other** -> retry only the incomplete peer
  and never rebuild or rewrite the signed object.
- **Installed acceptance fails** -> preserve the working rc.108 runtime and
  exercise the existing rollback path before retrying.

## Migration Plan

Publish rc.109 alongside the prior release candidate, install it through the
existing transactional lifecycle, and retain rc.108 as the rollback target
until the installed setup, sync, credential, and client-projection journey is
proven. No state-schema migration is introduced.
