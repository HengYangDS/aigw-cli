# Design

## Owners

ETHOS owns branch and tag publication to each peer. Publish GitLab `dev` and
`main` with `ethos publish --ref <ref> --peer gitlab --probe-remote
--expect-head <head>` and apply its receipt; both refs fast-forward, so no merge
request is needed. ETHOS `e05c372cf` lets a peer that missed a release receive
its attested tag when the release commit is in accepted history, so each tag
keeps the object GitHub received. The maintenance tags `v0.3.4` and `v0.3.5`
were created with `--release-base` from a maintenance line outside accepted
history. Their catch-up needs the maintenance publication admission on ETHOS
lane `work/20261002-native-trust-closure`, delivered through accepted ETHOS
`dev` and an aigw-cli runtime rebind. ETHOS's repository paradigm admits only
catalogued root entries, so that rebind first needs the root `.cbmignore`
removed.

The GitLab pipeline selects peer tool supply. Its mirror package is versioned by
the exact SHA-256 of `.config/mise/mise.lock` and must hold verified upstream
assets; AIGW does not produce it. The operator decides whether to publish that
package or to run GitLab with upstream supply where its runners reach GitHub.
Pushing branches does not repair supply. On 2026-10-14 the operator chose
upstream supply: the project's `AIGW_TOOL_SOURCE=peer` override was removed, so
the CUE-projected default applies.

Peer admission reuses the checks that `terminal-product-convergence` task 7.4
applied to GitHub. The ETHOS identity and reference successor is ETHOS's own
deliverable; this Change consumes the accepted release and adds no private
checker.

## Order

Catch-up precedes admission: GitLab must first hold the accepted branches,
tags and tool closure before its protected checks can describe them. The ETHOS
successor is independent and may complete in either order.
