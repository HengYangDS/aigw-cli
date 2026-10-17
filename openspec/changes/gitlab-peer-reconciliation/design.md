# Design

## Context

AIGW publishes to two independent peers, GitHub and GitLab. While GitLab was
unreachable, accepted branches, release tags and Releases reached GitHub only.
ETHOS owns branch and tag publication to each peer; the release owner in
`tools/release` publishes each peer's Release from a local bundle; GitLab's
pipeline installs its locked tool closure through the source the project
selects. The maintenance tags `v0.3.4` and `v0.3.5` were created with
`--release-base` from a maintenance line outside accepted history, which the
current ETHOS publication admission does not accept.

## Goals / Non-Goals

**Goals:** GitLab holds the exact branch heads, tag objects and Releases the
other peer received, passes its own protected pipelines for them, and
qualifies its admission controls as GitHub's did.

**Non-Goals:** rebuilding, re-signing or rewriting published objects; copying
another peer's Release assets; producing the peer tool mirror; a private tag
map or reference checker in AIGW.

## Decisions

### ETHOS publishes branches and tags

`ethos publish` previews and applies each peer's refs; branches fast-forward, so
no merge request is needed. A peer that missed a release receives the attested
tag object when its release commit is in accepted history. The maintenance tags
need the maintenance publication admission of ETHOS
`work/20261002-native-trust-closure` and a runtime rebind; until then, the
user-authorized break glass published their existing signed objects, and task
3.1 reconciles the missing ETHOS receipts.

### The operator selects tool supply

The peer mirror package is versioned by the exact SHA-256 of
`.config/mise/mise.lock`, and AIGW does not produce it. The operator chose
upstream supply, so the CUE-projected default applies and branch pushes alone do
not claim supply recovery.

### Each peer's Release comes from a local bundle

A Release is published only from the retained local bundle that publication
verifies, never from the other peer. A version whose bundle is gone stays
without that peer's Release: by the user's decision `v0.4.2` has none on
GitLab. Bundles whose notarization uploads were gone were notarized again from
their unchanged executables.

### Review isolation proves denial without relaxing the runner

The macOS review probe must show that the review identity cannot read the
protected runner's configuration and holds no elevation grant. That
configuration sits inside directories the review identity cannot traverse, so a
denied traversal counts as a denied read, and the probe reports the resource
identity as asserted by the operator. Relaxing the directories to fit the probe
would weaken the isolation it measures.

### Catch-up precedes admission

GitLab must hold the accepted branches, tags and tool closure before its
protected checks can describe them. The ETHOS identity and reference successor
is ETHOS's own deliverable and may complete in either order.

## Risks / Trade-offs

- [Break-glass tags carry no ETHOS publication receipt] → an ETHOS
  reconciliation may report them as unrecorded until task 3.1 completes.
- [A closed directory hides the protected resource from the review identity] →
  the probe cannot confirm the resource exists; the operator's exact path is its
  only identity, and the report says so.
- [Windows cannot run the protected-file probe] → Windows review isolation
  remains a bounded limitation; Linux review jobs run in disposable containers.
- [GitLab's source-branch auto-delete ignores ETHOS fast-forward merges] →
  ETHOS retires proposal refs itself.
