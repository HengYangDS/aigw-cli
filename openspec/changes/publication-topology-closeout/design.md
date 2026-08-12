# Design

## Decision

Use `.ethos/release.toml` as the sole publication topology. Local verification
invokes the accepted repository-owned CI command. Local installation invokes
the product CLI. GitLab and GitHub each declare their own remote and CI surface.

No peer is an input, fallback, or authority for the other peer.

## Verification

1. Validate the OpenSpec change.
2. Compile publication readiness from the work lane.
3. Execute exact-HEAD proof.
4. Land through the existing candidate and accepted train.
