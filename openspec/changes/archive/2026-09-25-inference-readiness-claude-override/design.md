# Design

## Context

A model-free catalogue request cannot detect refusal of the selected wire
model. Readiness therefore needs an explicit inference scope alongside the
endpoint-only scope. Claude settings inspection must also distinguish a proven
model-only native preference from an edit to AIGW's managed connection.

A Claude Code model alias is not an AIGW wire-model identifier. A native
`opus[1m]` request can resolve to a different concrete model, so copying the
alias into `Route.UpstreamModel` would produce a false AIGW inference test.
AIGW owns the selected Route, endpoint, and credential helper; Claude Code
owns its later native model preference.

## Goals and non-goals

The command must report what was actually observed: endpoint authentication,
one exact Route-model inference, or only a local projection. It must preserve a
sidecar-proven Claude preference and its exact settings bytes. It does not add
request proxying, automatic route failover, client-alias resolution, or a claim
about provider retention and future availability.

## Decisions

### Keep the team manifest curated and labels derived

The shipped manifest declares three Accounts, stable lower-case canonical
Model and Route IDs, exact provider `upstream_model` spellings, and per-client
Recommendations. It is not a copy of three provider catalogues. A provider
Route requires successful authenticated inference on its declared client and
protocol; Account symmetry does not require a full Route matrix. DMXAPI CC,
SSVIP, and CDX variants are distinct Routes to one canonical Model, never
additional logical Models. A Model identity with no admitted Route is not a
claim of provider availability. MiniMax M3 has exact-wire text inference on
AIHubMix and UCloud; its DMXAPI candidate timed out and is not admitted.
DeepSeek V4.1 Flash replaces V4 Pro 0813 after exact-wire inference on all
three Accounts and the vendor's stronger general-model positioning. Doubao
Seed 2.1 Pro 260628 has three exact-wire text Routes; newer vendor versions
without Account acceptance remain outside the manifest.
xAI now positions Grok 4.7 as its general flagship; all three Accounts
completed exact-wire Responses calls, so it replaces Grok 4.6 with no Account
coverage loss. Existing explicit local bindings remain outside this manifest
change's authority.
Xiaomi's documented general recommendation is `mimo-v2.6-pro`. Its exact
Responses wire ID returned completed text on AIHubMix and UCloud on September
25; no DMXAPI MiMo Route is admitted.
Additional vendor models are admitted only on AIHubMix after primary-vendor
positioning and one complete text call on their declared protocol: ERNIE 5.1
uses Responses; Mistral Large 3, Nemotron 3 Ultra, Solar Pro 4, Mercury 2.5,
LongCat 2.0, HY3, Step 3.7 Flash, and Ling 3.0 Flash use the now-qualified
Chat Completions endpoint. The `-free` Nemotron wire ID is one Route for its
base canonical Model. Preview MAI Thinking 1 and Step 5, and Command A+ with
HTTP 400, are not admitted.
Meta Muse Spark 1.3 has one AIHubMix Route. Although an earlier 128-token
Responses request completed once, repeated short `ping` calls at that cap
were incomplete. A 512-token explicit short-answer request completed with
text; the bounded probe budget is raised to 512 and still rejects incomplete
HTTP 200 results.
Unrelated Spark IDs on DMXAPI do not establish a Meta Route.
UCloud GLM 5.3's Responses path returned HTTP 200 without a standard
Responses output; Chat Completions returned assistant text at 512 tokens, so
that Route declares Chat Completions only.
Opus 5.5 Routes for all three Accounts and GPT-6 Sol Routes for AIHubMix and
UCloud passed selected-protocol requests on September 25. DMXAPI GPT-6 Sol
previously passed through a locally configured Proxy endpoint and once through
the manifest's direct Responses endpoint, but repeated September 25 direct
requests returned HTTP 503 while UCloud completed the same model. Release
0.3.1 therefore omitted the direct Route. Two later noninteractive direct
Responses probes on September 25 at 10:13 and 10:17 UTC returned HTTP 200,
completed assistant text, and no network error with the exact
`gpt-6-sol` wire ID. The next candidate restores that Route as explicitly
selectable as a setup alternative after AIHubMix Sol, without changing an
existing local Proxy endpoint or Client Binding. Two successes establish
current availability, not a reliability promise. AIHubMix and DMXAPI Luna
passed direct Responses requests and are admitted alongside UCloud Luna. The
team preference is DMXAPI Opus 5.5 for Claude clients, and UCloud GPT-6 Sol
for Codex and Hermes. The existing AIHubMix Sol alternative retains its order;
DMXAPI Sol follows it. DMXAPI Luna remains a separate manual choice. These
Recommendations do not rewrite existing Client Bindings or provide automatic
failover.

An ordinary Route has no stored label. Presentation derives `Account · Model`
from the declared Account and Model labels; only channel-specific or deliberate
user overrides retain a Route label. Native manifest export omits a redundant
derived label even when an older local configuration stored it. Route and Model
IDs remain independent of the provider's exact-case wire ID. Setup needs no
Token to import the catalogue, activates whichever one compatible Account is
connected, and preserves every explicit Client Binding. The manifest contains
no proxy endpoint.

### Keep empty activation distinct from diagnostic success

An imported catalogue may have valid Routes while every client remains
deferred. `check` has no subject in that state, so an empty loop cannot prove
health and must return a deferred nonzero result without probing. `status`
and `doctor` name the zero enabled-client scope. `doctor` may still pass its
local configuration diagnostics, but its JSON and human result must state
that no client is active. `sync` cannot use `check` as the continuation after
an empty selection. For the read-only environment backend, the continuation
names one compatible Account variable and leaves client activation to a later
`sync`. No new Keychain observation of unselected Accounts is admitted.

### One explicit diagnostic scope

`diagnostics.Scope` is an argument to `Probe` and `ProbeBounded`, and
`Result` carries the scope actually attempted. The selected scope determines
the request and its deadline; a credential rejection terminates after one
request rather than entering a second recovery loop.
`check` selects inference by default, or endpoint when `--endpoint-only`
is present. A client-native authentication Route remains local configuration
evidence. A proven Claude native model preference selects endpoint scope for
that client even under the command default; the result names that exception
and continues to `aigw verify --for claude`.

The alternative of reusing `endpoint_checked` after an inference request
would make one state mean two kinds of evidence. Inferring scope from the
caller instead of carrying it on `Result` would let rendering overclaim after
a fallback or construction error.

### Build the exact request in the credential owner

`internal/credential` adds a model-carrying request beside the existing
catalogue request. It requires a nonempty `Runtime.Model`, uses the selected
protocol and endpoint, does not stream, and caps both requested output and the
body read. All three protocols request at most 512 output tokens with the
short prompt `Reply with exactly pong.`; Responses also sets `store: false`.
Endpoint-only probes keep a five-second attempt limit, while inference gets
one 60-second attempt after a Gemini 3.1 Pro response completed in 48 seconds.
AIGW stores no response conversation. For protocols without a no-storage
switch, it makes no provider-retention claim. The existing endpoint join and
authentication header conventions remain the wire authority.

A catalogue diff is insufficient: the incident model was listed while its
distributor refused inference. Constructing a malformed request or receiving
an unsupported-parameter response must fail visibly; it must never fall back
to a green catalogue result. Inference checks use one attempt and do not retry
unchanged authorization or model-availability failures.

### Recognize only a proven Claude model preference

`internal/claude` inspects its existing sidecar without writing. If the
recorded managed hash matches after substituting only the prior Route's model,
it returns a native-model-override observation. Any endpoint, helper, or
managed-credential edit still fails as an ownership conflict. The inspection
does not adopt the current model as a Route model or change the sidecar.
`internal/client` carries the observation to readiness. Status and doctor
report local connection readiness and the native verification continuation;
check may authenticate the endpoint but may not claim inference for the
native alias. Synchronization leaves a proven native preference untouched
while the Route model and Account/endpoint credential scope remain the same.
An AIGW-owned helper path update changes the helper but not that preference;
a changed model or credential scope projects the selected Route model.
External edits to the helper or other managed connection fields still fail
the sidecar guard.

Native-client evidence is time-bound. When the host no longer carries the
observed alias, exercise a real request in an owned isolated Claude context
with noninteractive credentials or leave that acceptance open. Never modify
the user's model selection or sidecar to recreate an earlier observation.

The rejected alternatives are silently projecting the old Route default,
inventing a provider Route from a client alias, or treating the changed model
as a changed AIGW credential. Each would confuse a separate owner.

### Project one truthful result

`internal/readiness` maps a healthy endpoint observation to
`endpoint_checked` and a healthy inference observation to
`inference_checked`. Existing typed failures keep their current degraded,
invalid, and unavailable classes. JSON exposes the performed
`diagnostic_scope` and whether Claude has a proven native model preference;
human output conveys the same fact and the same next action. Neither surface
treats `check_passed` as real-client proof. Status and doctor remain
non-inference commands.

## Dependency order

1. Finalize the scoped evidence and Claude preference contracts in this
   Change.
2. Add request construction and protocol tests in the credential owner.
3. Pass scope through diagnostics and readiness, including the distributor
   refusal regression.
4. Add read-only Claude preference inspection and carry the observation through
   the client Adapter.
5. Select scope and render matching human/JSON results in check, status, and
   doctor; update terminal guidance.
6. Reconcile the three-provider team catalogue against current public IDs and
   prior inference evidence. Keep GPT Astra/Sol/Luna and Claude Fable 5.1,
   Opus 5.5, and Sonnet 5 as the requested logical identities; an unverified
   identity has no Route. Retain one previously qualified general Model per
   other vendor globally, with separately evidenced Account Routes. Map
   DMXAPI channel variants to their base Model while preserving exact wire
   IDs; do not retain a second general Model to hide lost Account coverage.
   A public ID and successful inference establish route availability, not a
   model's relative general strength. Replacing a retained general Model also
   requires current primary-vendor general-purpose positioning and reviewed
   Account coverage. A class-relative Flash claim cannot rank it above Pro.
7. Reconcile team-owned local Models and Routes through AIGW's configuration
   owner. The before/after plan must preserve Accounts, Tokens, client
   selections, projections, native preferences, and foreign content. Public
   catalogue listings and an unauthenticated 401 are not inference evidence;
   a Keychain UI requirement stops dependent live probes.
8. Bind the final candidate team manifest to the actual installed local
   profile through public export, then read back explicit Client Bindings and
   native projections. Check global vendor cardinality, exact Account Route
   semantics, and per-Route inference; review any lost Account coverage before
   replacing a vendor's one general Model.
9. Project the one CUE product-native matrix into both Forges. Runner selectors
   remain peer-specific, but neither a capability list nor another peer's result
   may shrink a required gate. Preserve GitLab's current Darwin control selector;
   qualify Linux Docker and the on-demand Windows ARM64 runner on the exact
   candidate review before archive. Validate the tag graph before archive;
   observe the actual tag jobs after archive before claiming GitLab release
   acceptance. A paused Windows runner is not a green gate. GitLab
   release-assets verifies published signed bytes after its own quality,
   version, and native jobs; it neither signs nor notarizes them. Stable
   signing and Apple notarization belong to the release publication owner,
   not another CI lane.
   Peer independence means no AIGW source, policy, evidence, or asset comes
   from the sibling product peer. GHCR images and GitHub-hosted third-party
   tool archives are separate locked supply-chain dependencies; this change
   does not claim survival of a global GitHub distribution outage.
   Select exactly one native `notarytool` authentication mode: a validated
   Keychain profile or an existing protected App Store Connect API key file,
   key ID, and issuer when required. Never retry a failed profile through a
   guessed credential, prompt, or silent fallback. The same accepted Apple log
   must bind the uploaded ZIP and both final macOS executables in either mode.
10. Before archive, verify focused behavior, candidate bytes, exact-HEAD
    quality and native gates, live routes, and peer review. After archive,
    verify signed publication, installation, rollback, and owned cleanup.
    `tasks.md` tracks only the pre-archive implementation obligations.

The existing OpenSpec gate now treats every native finding as a failed project
check, even when the official validator labels it informational. Five overlong
canonical requirement bodies were condensed or split without dropping their
scenarios; this uses the existing validator rather than another specification
parser or waiver list.

### Credential entrypoint during package replacement

The retained-command unlink test fails with shell status 127 when the command
names the Homebrew-managed CLI link. Before-and-after checks cannot prove
continuous credential delivery across that interval.

- The Cask CLI link cannot be the helper entrypoint because Homebrew unlinks it.
- Four client-specific fallbacks duplicate path and failure policy.
- A proxy, daemon, or launcher adds an unrelated runtime.
- One AIGW-owned executable at the platform-native data path is the candidate:
  the package manager leaves it alone, and the existing `credential` command
  still reads the one selected Token backend.

The selected candidate copies the verified current AIGW executable only when
an enabled default Account-Token binding needs it. Its path is stable across
routine CLI upgrades; ordinary sync never replaces a working copy merely
because the CLI version changed. It is a derived credential entrypoint, not a
second CLI installation or Token store. Creation, provenance, no-op behavior,
ownership, exact rollback, and eventual removal need native tests on each OS.
An update to the credential reader itself is a separate admitted transition.

The complete desired Client Binding set, not only the selected projection,
owns helper retention. After successful projection withdrawal of the last
default Account-Token consumer, AIGW removes its intact copy and receipt;
another default consumer retains both. An incomplete projection rollback
never triggers removal. A cleanup failure after successful projection is
reported as a committed transition with incomplete cleanup, so a later `sync`
can retry. Dry-run names either installation or removal without writing.
An enabled explicit credential command resolving to the same executable also
retains it; this does not authorize AIGW to create an external helper.
Explicit client disable revokes that binding's credential command; it is not
equivalent to an unchanged active binding during a package-manager upgrade.

Already-running clients may have cached the old Brew path. Rewriting their
configuration cannot prove they adopted a new command. The first host cutover
therefore retains the existing CLI link until those legacy callers are proved
absent or migrated; otherwise it stops before Brew unlink. AIGW cannot promise
continuity for an arbitrary external `brew upgrade` that bypasses this
precondition. No live Keychain item or client is changed to make a test pass.

## Risks and mitigations

- A protocol or gateway rejects a minimal output field or `store: false`:
  classify that request as unusable, test the exact shipped provider route,
  and adjust only the owning request shape with evidence.
- A transient 503 changes after the probe: report a time-bound observation,
  never a promise of future availability or an automatic failover decision.
- Default checks consume a small metered request: document the cost and keep
  `--endpoint-only` for operators who need the catalogue scope.
- Claude changes a model alias to another wire model: retain the native alias
  as client-owned and require native verification; never infer an AIGW Route
  from the alias.
- A platform's native client or credential store behaves differently: require
  the existing multi-platform native acceptance and explicit selected-route
  journey before distribution.
- A public catalogue advertises a new ID that cannot be called without
  interaction: record it as a candidate, keep it out of selectable Routes, and
  resume only through an authorized noninteractive credential path.

## Post-archive delivery acceptance

After all Change tasks and final source admission pass, archive the Change
before creating a stable tag. The release owner then:

1. Publishes one signed stable release with identical archives and checksums to
   GitHub and GitLab. It verifies exact refs and tag jobs, both Release records,
   every asset byte, macOS Developer ID signing and accepted notarization.
   Apple authentication uses exactly one validated noninteractive mode; mixed
   or incomplete inputs fail without prompting or fallback.
2. Publishes and installs the matching Homebrew Cask, verifies the installed
   executable, UCloud Routes, Claude preference, both check scopes, and the
   original client credential calls continuously through upgrade, bounded
   rollback, and re-upgrade.
3. Preserves recovery material and foreign state, removes only proved
   disposable residue, and checks the exact worktree, refs, lease, and artifact
   inventory before ETHOS closeout. A failed delivery effect does not turn an
   archived Change into a successful release.

## Migration and rollback

No configuration schema or credential migration is required. Existing
sidecars remain authoritative for connection ownership, and the new
inspection is read-only. Keep the installed stable release until the
candidate passes exact-HEAD, packaged/native, and selected-route acceptance.
Rolling back the program restores the previous check semantics without
rewriting user settings or Codex session state.
