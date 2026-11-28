## Context

Sources, read on 2026-11-28 between 18:58 and 19:10 CST: the repository's Renovate image 44.239.1 in a dry run of
its own policy (`--platform=local --dry-run=full`, read-only mount of dev `4143fb7dc`), which found the eleven updates
of the proposal and held two; `mise latest` for every tool of `.config/mise/config.toml`, which agrees on the Mise
tools; GitHub's release lists for apple-codesign, lychee, Renovate and Mise, of which only Renovate is behind; the Go
module proxy for `ansi` v0.12.0 (2026-11-26 09:56Z). Hermes v0.23.4's tag names commit
`8fc0374447313e276f6b9a2764bfc750190c966a`; its `scripts/install.sh` has the SHA-256 that v0.23.2's has, read the way
that reproduces v0.23.2's pinned digest.

## Goals / Non-Goals

**Goals:**

- Every declared input at its latest stable release that the release age admits, except the two held below.

**Non-Goals:**

- Indirect Go requirements that no direct requirement asks for.
- A change of a supply rule.

## Decisions

### CUE 0.18.0 is taken, and the projections decide

- **Rests on:** CUE 0.18.0 makes embedding strict from language version v0.18.0 on; `.config/ci/pipeline.cue` has no
  module file, so it evaluates at the binary's language version. The tracked GitHub and GitLab pipelines are
  generated from it, and `tools/ci project --check` compares them.
- **Considered:** hold CUE at 0.17.1; take it and let the projection check decide.
- **Chosen because:** a changed meaning shows as drift in the generated pipelines, which the check refuses, so taking
  the release cannot change a pipeline silently.
- **Applies to:** `cue` in the Mise configuration. Under 0.18.0 the regenerated pipelines changed only in the native
  client pins of this Change, and its formatter aligns fields differently, so `.config/ci/pipeline.cue` takes
  `cue fmt`'s new layout with no change of meaning.

### The Renovate image and Hermes v0.23.5 are held

- **Rests on:** the dry run holds every newer Renovate image, because the policy requires a release timestamp
  (`minimumReleaseAgeBehaviour: "timestamp-required"`) and the docker datasource gives none; the host keeps one
  Renovate image for its repositories (the decision of 2026-11-18). Hermes v0.23.5 was published on 2026-11-28 at
  03:20Z, under 24 hours before the sweep.
- **Considered:** pin Renovate 44.264.1, whose GitHub release is 43 hours old; hold both.
- **Chosen because:** the repository's own policy holds the image, and moving the host's one image is the host supply
  owner's change, not this lane's. Hermes v0.23.5 is not yet admitted.
- **Applies to:** `vars.renovate_image` and `nativeClients.hermes`. Owner: this session's next supply refresh.

## Risks / Trade-offs

- **A tool's new release changes a check's output** → the lane runs the fast gates on each commit and the full proof
  on the archive head.
