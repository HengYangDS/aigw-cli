## Context

See [proposal.md](proposal.md). Local `main`, `dev`, and `candidate/dev` now
identify the same accepted onboarding commit. `VERSION` owns product version;
`CHANGELOG.md` owns release chronology. GitLab and GitHub are independent
projections of one signed source and tag object.

## Goals / Non-Goals

**Goals:**

- Create one immutable rc.106 source identity from the accepted product.
- Reuse the existing source, native-platform, packaging, signing, publication,
  installation, rollback, and uninstall paths.
- Preserve exact Git objects and byte-identical assets across both Forges.

**Non-Goals:**

- Change onboarding, credential, client projection, or transport behavior.
- Add release wrappers, compatibility paths, or Forge-specific product logic.

## Decisions

Advance only the existing release authorities and use the existing lifecycle.
This avoids a parallel release mechanism and keeps versioning, chronology,
verification, publication, and runtime acceptance independently provable.

## Risks / Trade-offs

- **A platform may differ from local macOS** → require existing native macOS,
  Linux, and Windows acceptance for the exact release source.
- **One Forge may finish first** → observe and verify each peer independently;
  never use one peer as the other's source.
- **A release effect may fail after source acceptance** → keep the signed source
  immutable and retry only the failed external effect.

## Migration Plan

Publish rc.106 alongside earlier release candidates. Rollback selects the prior
verified release through the existing package lifecycle; no schema or state
migration is introduced.
