# Authority and Projection Boundary

Status: canonical.

AIGW resolves a canonical local account/profile/route configuration and
projects only to host surfaces that its ownership contract admits. It does not
start a background service or carry request traffic.

```text
AIGW configuration -> admitted-target preparation -> guarded commit
                                               -> compensating rollback on failure
Codex Desktop      -> existing conversation model selection and transcripts
JetBrains AI       -> PyCharm, Air default selection, and Junie CLI authority
Codex DMX Proxy    -> local Responses compatibility and listener lifecycle
```

A host surface has one of the following contracts:

| Surface | Authority | AIGW projection contract |
| --- | --- | --- |
| Ordinary standalone Codex CLI | AIGW | Full provider/model selection is admitted. |
| ChatGPT Desktop | Desktop for existing conversations | AIGW may provide default configuration for later/new work, but never edits session metadata, JSONL, SQLite, a selected conversation model, or a transcript. |
| PyCharm Codex | JetBrains AI | Excluded from AIGW target adoption and generic reconciliation. |
| JetBrains Air | JetBrains AI | Default selection and exact host mirrors are external. Only `aigw route fallback air` may stage a reversible, namespaced AIGW block. |
| Junie CLI | JetBrains AI | Classified for route diagnosis only; it is never a Codex target. |

A projection comprises `config.toml` plus its `.aigw-state.json` sidecar. A
sidecar records its projection mode, writer ID, and transaction ID. A legacy
sidecar with none of those fields is adoptable only as a full-selection legacy
projection; partial, foreign, and mode-mismatched attribution fails closed.

Each transaction captures byte-exact pre-state for both artifacts, including
the absence of a sidecar. It validates every admitted target before the first
write. Each commit performs a guarded preimage check; this is best-effort
in-process coordination, not a cross-process or crash-safe compare-and-swap.
On a write failure, AIGW compensates in reverse order only while an artifact
still equals this transaction's postimage, so it does not overwrite a newer
writer. The narrowly defined exact-truncation repair remains part of
preparation and does not weaken foreign-edit conflict checks.

`aigw sync --dry-run --json` exposes a secret-free target/action plan. Generic
setup and repair discover only the ordinary standalone Codex CLI. An already
staged Air fallback remains reconcilable as its namespaced mode, but generic
commands never turn an Air or PyCharm default into an AIGW full selection.
Dry-run does not mutate files, bind native authentication, restart clients, or
modify conversation state.

`aigw repair --dry-run --json` is the transition preview for legacy target
membership. It computes the desired repair state without enabling a Claude
shim, binding native Codex authentication, writing configuration, or taking a
mutation lock. Its output identifies planned Codex actions by stable surface ID
rather than by configuration path, so an operator can verify restoration of a
legacy PyCharm/Air full selection before applying `aigw repair`. The preview is
not a host-idleness proof and does not authorize an apply while a host has
active work.

`aigw route doctor` is a separate local observer. It reports host policy,
sidecar attribution, and selection conflicts without running a client,
contacting a provider, reading credentials, exposing paths or configuration
bodies, or claiming session, endpoint, terminal, or billing proof. Junie is
always reported as not probed. On a route-ownership conflict it recommends the
repair preview, not an Air fallback restore that could be invalid while Air is
still selected to AIGW at the top level.

Equal AIGW bytes or markers do not transfer ownership. Air is an
`external-host-mirror` only when its exact managed projection matches a current
standalone full-selection projection with a recognized, hash-matching sidecar.
Without that positive reference proof, a sidecar-absent exact generated full
selection is `orphaned-exact-full-selection`; partial, duplicate, fallback,
foreign, and changed shapes remain fail closed. Route doctor gives a host
mirror no mutation guidance, keeps ADR-0003's sidecar-mismatch recovery
separate, and recommends `recover-orphan` only for the exact orphan.

`aigw route attest air` observes bounded forwarding records from one fresh Air
log generation. This ephemeral evidence does not grant authority or prove
process lifecycle, authentication, billing, terminal outcome, or a visible
reply. It reads no headers, bodies, prompts, responses, credentials, or
conversation state.

Exact-orphan recovery binds immutable snapshots of the Air config and sidecar
and the standalone config and sidecar. Its private journal and quarantine retain
the byte-exact preimage while guarded removal leaves an unset external
baseline. Settlement updates only that journal and quarantine; it never writes
Air or invents a JetBrains selection.

Air fallback never changes the top-level `model` or `model_provider` keys. It
adds/removes only an AIGW-owned namespaced suffix, preserving the original Air
bytes when restored. Its apply and restore commands require
`--confirm-host-idle`; that flag is an operator attestation, not a process
probe or lifecycle action.

When a selected endpoint targets a loopback host, AIGW reports it as an
external loopback compatibility layer in `status` and `check`. The diagnostic
does not expose the endpoint URL or port, infer a particular proxy product, or
claim transport ownership. Codex requests use the selected listener, so its
availability remains a runtime prerequisite for that route. Transport start,
stop, configuration, manifest, health diagnosis, and watchdog ownership remain
outside AIGW.
