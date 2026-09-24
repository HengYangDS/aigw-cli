# Design

## Context

See proposal.md for the two readiness failures. Today
`credential.ValidateRuntime` makes a model-free catalogue request, and
`diagnostics.ProbeStable` can therefore classify a channel refusal only if a
future caller somehow supplies one. Claude settings inspection requires an
already-converged projection even though the settings owner can prove a
model-only native preference and safely reconcile an explicit Route later.

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

### One explicit diagnostic scope

`diagnostics.Scope` is an argument to `Probe` and `ProbeStable`, and
`Result` carries the scope actually attempted. Scope is independent of retry
policy: it answers what request was sent, while policy bounds how it was sent.
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
body read. Anthropic Messages uses `max_tokens: 1`; OpenAI Responses uses
`max_output_tokens: 16` and `store: false`; Chat Completions uses
`max_tokens: 1`. AIGW stores no response
conversation. For protocols without a no-storage switch, it makes no
provider-retention claim. The existing endpoint join and authentication
header conventions remain the wire authority.

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
native alias. Explicit `aigw use` and synchronization retain their existing
guarded projection behavior.

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
9. Project ordinary GitLab verification, tag-version validation, and published
   artifact verification to the disposable macOS CI runner. The authorized
   macOS host alone performs Developer ID signing and Apple notarization;
   those operations are separate from the GitLab jobs. Verify the generated
   pipeline and each relevant job's actual runner identity and exact source
   SHA. An older successful VM job does not qualify the current candidate.
10. Verify focused behavior, exact HEAD quality and native gates, selected
    released bytes, live routes, publication, installation, rollback, and owned
    cleanup in that order. tasks.md is the only progress ledger.

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

## Migration and rollback

No configuration schema or credential migration is required. Existing
sidecars remain authoritative for connection ownership, and the new
inspection is read-only. Keep the installed stable release until the
candidate passes exact-HEAD, packaged/native, and selected-route acceptance.
Publish one signed stable release through the existing Forge and Homebrew
workflow, test installed behavior and rollback, then retire only the owned
Lane. Rolling back the program restores the previous check semantics without
rewriting user settings or Codex session state.
