## Context

Pinning a container digest provides reproducible startup, but the upstream
image release cadence can lag the latest stable mise binary. Lowering
`min_version`, duplicating the version in CI, or creating an AIGW-owned image
would respectively weaken policy, create parallel version authority, or add an
unnecessary product entity.

## Decision

Retain the verified upstream image digest as the hermetic base and run the
official version-independent `mise self-update --yes --no-plugins` command
before locked execution. The command resolves the current stable release;
`mise.toml` remains the lower-bound and project-tool authority.

The CUE model owns the bootstrap command and ordering. `.gitlab-ci.yml` remains
a generated projection.

## Verification

1. A focused projection regression proves every image-backed job refreshes
   mise before any locked command.
2. Projection reconciliation proves `.gitlab-ci.yml` matches the CUE model.
3. The complete source graph and exact-HEAD proof run before landing.
