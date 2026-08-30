## Context

The accepted source already contains the behavior and its exact-HEAD proof.
This Change assigns those immutable bytes a new release identity without
altering product behavior.

## Goals / Non-Goals

**Goals:**

- Keep `VERSION` as the sole machine-readable release version.
- Keep `CHANGELOG.md` as the sole user-facing release chronicle.
- Publish one signed commit and one signed annotated tag unchanged to both
  Forge peers.

**Non-Goals:**

- No new client, provider, Route, credential, or transport behavior.
- No Proxy installation or lifecycle coupling.
- No compatibility alias or duplicate release authority.

## Decisions

Create rc.103 from the exact accepted product shared by local `main`, `dev`,
and `candidate/dev`. The release source change is limited to `VERSION`,
`CHANGELOG.md`, and this OpenSpec Change.

After strict source and native-platform verification, create the signed tag
locally and project the same Git commit and tag objects to GitLab and GitHub.
Rebuilding separate histories or Forge-specific release commits is rejected
because the peers are projections of one product object.

## Risks / Trade-offs

- [Hosted runners or a Forge are unavailable] -> Preserve the immutable local
  object and publish only through exact-ref compare-and-swap after the peer is
  observable; never rewrite the release.
- [An installed candidate regresses a working client] -> Keep rc.101 installed
  until rc.103 assets pass the release matrix and installed-runtime check.
