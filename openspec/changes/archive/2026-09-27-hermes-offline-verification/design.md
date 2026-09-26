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
5. The real-client acceptance fixture also calls Hermes `--version` directly.
   Set the same native update opt-out in its disposable `HERMES_HOME` before that
   preflight, preserve it through projection, and use the existing bounded
   process runner for fixture-owned client commands. Do not edit the user's
   Hermes home or add a second version-probe path.

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
- **The real-client preflight reaches GitHub before model inference** → Assert
  its isolated opt-out before invocation, then reject update-status output or
  cache creation from the installed Hermes CLI.

## Migration Plan

No user data migration is needed. Prove the focused regression, full native and
real-client journeys, then build and qualify a new signed candidate on macOS,
Linux, and Windows before dual-peer publication. Keep the installed 0.3.1
executable and client projections unchanged until a separately verified
credential-entrypoint cutover and bounded package upgrade.

Complete the source, candidate, and performance tasks before archiving this
Change through ETHOS. Archive changes the tracked source tree, so the qualified
pre-archive candidate and its Apple submission cannot authorize the release.
One explicitly selected peer's exact-object review CI admits the pre-archive
candidate; unavailable peers do not block local source acceptance. Change tasks
end at implementation and candidate acceptance. Each peer's accepted-ref and
tag CI, published assets, installation, and lane retirement remain binding
post-archive obligations in the canonical specifications and this design.
From the archived, signed commit, build one new Developer ID matrix; verify its
exact source, real clients, predecessor transition, performance, and a new Apple
`Accepted` submission before minting the stable tag at that same commit. Then
verify the signed tag against those unchanged bytes, publish them to both peers,
and check each peer's macOS, Linux, and Windows tag CI and asset digests. Any
source change after final construction restarts this exact-byte qualification.

Before changing the Homebrew link, verify the already-owned stable credential
entrypoint; then prove installed upgrade, rollback, forward upgrade, and live
Client Binding continuity without prompts or service interruption. Finally
reconcile exact owned release outputs, temporary services, proposal refs, and
Work Lane state. Disclose the local-only `v0.3.2` history until ETHOS provides
native exact-OID abandonment. These are post-archive delivery obligations, not
tasks that block their own archive.

## Local Qualification

The focused adapter test failed before the disposable-home opt-out, then passed
after it. A separate categorical-error assertion failed against the former
generic message, then passed after the adapter change. The installed Hermes CLI
passed `go test -tags=client_acceptance ./tools/release -run
'^TestNativeClientJourney/hermes$' -count=1 -v` against a disposable source-built
program and local authenticated endpoint, including Account rename and
finalization. This is focused local evidence, not a signed release or full
client-matrix claim.

The signed `538f2d69` pre-archive matrix passed `accept-native` in explicit
`--artifacts --candidate --clients` mode with a product-verified published
0.3.1 predecessor.

Apple submission `3de53c2f-ddd5-47d7-be5d-4de5a4a250a3` returned `Accepted`;
`verify-macos-distribution` then passed for its exact two-architecture ZIP,
certificate, and submission ID.

The first performance run exposed a test-fixture error: the published 0.3.1
predecessor requires `aigw use --for claude` in non-interactive mode. After
removing that unnecessary variant split, the tracked `TestNativePerformance`
passed with 24 forty-sample blocks, 12 pooled rows, four peak-memory records,
and all candidate budgets met. These results qualify that pre-archive candidate
only; the archived source requires new final bytes and acceptance.

The real-client fixture's Hermes preflight initially lacked the native update
opt-out and failed a focused policy assertion. With `updates.check: false`
applied before the direct version call, the installed Hermes CLI completed the
isolated client journey under a blocked external proxy without update-status
output or an update-check cache. Fixture-owned commands now use the same
deadline-bounded process runner as product verification.
