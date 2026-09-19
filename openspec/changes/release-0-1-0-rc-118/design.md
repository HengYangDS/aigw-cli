## Context

See [proposal.md](proposal.md). The accepted source has completed and archived
its dependency refresh. Local `main`, `dev`, and `candidate/dev`, plus both
Forge `main` and `dev`, identify the same signed commit. `VERSION` owns product
version; `CHANGELOG.md` owns release chronology.

## Goals / Non-Goals

**Goals:**

- Create one immutable rc.118 source identity from the accepted product.
- Reuse the existing source, native-platform, packaging, signing, publication,
  installation, rollback, and uninstall paths.
- Preserve exact Git objects and byte-identical assets across both Forges.

**Non-Goals:**

- Change runtime behavior, configuration, credentials, or client projections.
- Add release wrappers, compatibility paths, or Forge-specific product logic.

## Decisions

Advance only the existing release authorities and use the established lifecycle.
The release includes all accepted commits after rc.117 without another product
change. Rebuilding rc.117 was rejected because released tags are immutable and
would obscure which dependency graph an installed artifact contains.

## Risks / Trade-offs

- **A platform may differ from local macOS** → Require native macOS, Linux, and
  Windows acceptance for the exact release source.
- **One Forge may finish first** → Verify each peer independently and never use
  one peer as the other's publication source.
- **Installation may fail after publication** → Retain rc.117 as the verified
  rollback target and use the existing update, rollback, and uninstall paths.

## Migration Plan

Publish rc.118 beside rc.117, install the verified native artifact, prove the
current configuration and client projections remain healthy, then verify
rollback to rc.117 and re-upgrade to rc.118 before retiring the release lane.
