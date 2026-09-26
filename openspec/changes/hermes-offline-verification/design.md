# Design

## Context

See [the proposal](proposal.md). Hermes' installed `--version` path performs a
synchronous passive update check unless its native configuration disables it.
AIGW creates a new disposable Hermes home for every live verification, so a
version probe has no reusable update cache. The client adapter already owns
that home and its verification process; neither the user's Hermes home nor a
shared provider layer should acquire this policy.

## Goals / Non-Goals

**Goals:** Make Hermes verification depend only on the selected inference
endpoint, bound its version probe, and report its failure category safely.

**Non-Goals:** Change Hermes installation or update policy, user preferences,
provider routing, credential ownership, or the behavior of other clients.

## Decisions

1. Add Hermes' native `updates.check: false` beside the existing
   `security.allow_lazy_installs: false` in the disposable verification
   configuration. This is narrower than a wrapper, global environment switch,
   prepopulated cache, or user-home edit. The projection must preserve both
   settings before the version and model probes run.
2. Classify the version probe's existing bounded process result at the Hermes
   adapter: deadline, cancellation, and other execution failure. Keep raw child
   output out of public errors. Do not add a second process runner or retry
   policy; the selected model request remains a separate step.
3. Leave the signed, unpublished local `v0.3.2` object and ETHOS history intact,
   but fold its user changes into the pending 0.3.3 Changelog section. A local
   tag is not proof of publication: listed historical headings require tags,
   while unlisted local tags must not manufacture release history or constrain
   `VERSION`. Verify this with and without the local-only tag. Later exact-OID
   abandonment belongs to ETHOS, not AIGW's build or a raw Git bypass.
4. Require `accept-native --artifacts ... --candidate` before tagging. Its
   existing matrix and provenance owners verify the approved artifact signer,
   current signed HEAD, locked inputs, and supplied bytes. Candidate mode rejects
   a selected or same-version local tag; `verify-artifacts` and publication keep
   their signed-tag path. Do not create a temporary tag or a second verifier.

## Risks / Trade-offs

- **Vendor configuration drift** → A focused contract test checks the isolated
  policy before invocation; real-client acceptance exercises the installed
  Hermes version and authenticated local test endpoint.
- **A different failure shares the old generic message** → Preserve the
  failing full-client evidence, make cause categories observable, and require a
  new full client run rather than treating a focused pass as release proof.
- **Unpublished local tag remains visible** → Disclose it outside the release
  Changelog, validate clean remote-equivalent refs, and wait for ETHOS' native
  exact-OID abandon path rather than publishing or reusing it.
- **Tagless acceptance masks an invalid release tag** → Require explicit
  candidate selection and reject a same-version local tag before native tests.

## Migration Plan

No user data migration is needed. Prove the focused regression, full native and
real-client journeys, then build and qualify a new signed candidate on macOS,
Linux, and Windows before dual-peer publication. Keep the installed 0.3.1
executable and client projections unchanged until a separately verified
credential-entrypoint cutover and bounded package upgrade.

Complete the source, candidate, and performance tasks before archiving this
Change through ETHOS. Only then mint the new signed stable tag, publish the same
object and assets to both peers, and verify each peer's required macOS, Linux,
and Windows tag CI and asset digests. Before changing the Homebrew link, verify
the already-owned stable credential entrypoint; then prove installed upgrade,
rollback, forward upgrade, and live Client Binding continuity without prompts
or service interruption. Finally reconcile exact owned release outputs,
temporary services, proposal refs, and Work Lane state. Disclose the local-only
`v0.3.2` history until ETHOS provides native exact-OID abandonment. These are
post-archive delivery obligations, not tasks that block their own archive.

## Local Qualification

The focused adapter test failed before the disposable-home opt-out, then passed
after it. A separate categorical-error assertion failed against the former
generic message, then passed after the adapter change. The installed Hermes CLI
passed `go test -tags=client_acceptance ./tools/release -run
'^TestNativeClientJourney/hermes$' -count=1 -v` against a disposable source-built
program and local authenticated endpoint, including Account rename and
finalization. This is focused local evidence, not a signed release or full
client-matrix claim.
