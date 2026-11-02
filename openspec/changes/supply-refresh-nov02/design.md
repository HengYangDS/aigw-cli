## Context

Mise declares the developer and CI tools. The CUE authority projects the Mise
image, its runner-owned Windows ARM64 identities and the native client pins
into both Forges. The repository's Renovate policy holds a new npm release for
three days; the native client pins follow the same age by practice.

## Goals / Non-Goals

**Goals:** every supply input on its latest stable release that the policy
admits, with each digest read from its publisher.

**Non-Goals:** a release, a change to the AIGW program, or overriding the
release age.

## Decisions

- Each digest is read from its publisher: the Mise and Renovate image index
  digests from their registries, the Mise Windows ARM64 executable and shim
  digests from the official zip after its SHA-256 matched the release's
  SHASUMS256.txt, and the mise-action commit from its v5.2.5 tag. The
  executable's digest also matches the standalone executable's entry in that
  SHASUMS256.txt.
- A release that ships as several packages ages from the latest of the packages
  that the supplying hosts install: Claude Code 2.1.316's Windows x64 package
  was published last, at 2026-10-31T00:43:22Z, four minutes after its macOS
  package. The supply-chain specification states this as a scenario.
- The refresh lands the updates that the policy admits on 2026-11-02 at once
  and holds OpenSpec 1.16.0 and Claude Code 2.1.316, which the next refresh
  takes when their ages end; admitted updates do not wait a day for them.
- `go.yaml.in/yaml/v3` v3.0.6 holds one fix: a literal `<<` string is quoted
  when encoded, so that it does not read back as a merge key. The Hermes
  adapter links it into released programs; without an advisory, it ships with
  the next release.
- The native client pins move with one real-client journey on every host
  before landing, as each earlier pin moved.

## Risks / Trade-offs

- The GitLab Windows runner owns its Mise. Its executable must be replaced
  with the 2026.10.25 release before any aigw GitLab job consumes this
  declaration; the deferred GitLab catch-up does that first.
- A client release can change behavior that the journeys observe; a failing
  journey holds the pin at its earlier release.
