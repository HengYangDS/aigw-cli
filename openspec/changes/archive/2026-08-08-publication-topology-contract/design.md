# Design

## Decision

Keep `.ethos/release.toml` as the sole publication-topology declaration. Reuse
the repository's existing executable verification and installation surfaces
and each Forge's existing CI file; do not introduce a wrapper or second policy.

## Verification

- run the governance check and the declared local verification command;
- compile ETHOS publication readiness for both remotes;
- execute the exact-HEAD proof before archive and promotion.
