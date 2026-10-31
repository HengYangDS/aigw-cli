## Context

Mise declares the developer and CI tools. The CUE authority projects the Mise
image, its runner-owned Windows ARM64 identities and the native client pins
into both Forges. The repository's Renovate policy holds a new release for
three days.

## Goals / Non-Goals

**Goals:** every supply input on its latest stable release that the policy
admits, with each digest read from its publisher.

**Non-Goals:** a release, a change to the AIGW program, or overriding the
release age.

## Decisions

- Each digest is read from its publisher: the Mise and Renovate image index
  digests from their registries, and the Mise Windows ARM64 executable and
  shim digests from the official zip after its SHA-256 matched the release's
  SHASUMS256.txt.
- OpenSpec 1.16.0 waits for the release age that the policy declares; the
  refresh does not override the policy. The supply-chain specification states
  this hold as a scenario of its latest-stable requirement.
- The native client pins move with one real-client journey on every host
  before landing, as each earlier pin moved.
- Each document above the ETHOS ceiling gives up a whole section that has its
  own subject, rather than shortened wording, so each document keeps room to
  grow. The moved text keeps its words; only relative links and references to
  the text left behind change, and each old heading stays as a pointer so that
  existing anchors resolve.

## Risks / Trade-offs

- The GitLab Windows runner owns its Mise. Its executable must be replaced
  with the 2026.10.24 release before any aigw GitLab job consumes this
  declaration; the deferred GitLab catch-up does that first.
- A client release can change behavior that the journeys observe; a failing
  journey holds the pin at its earlier release.
