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
| JetBrains Air | JetBrains AI | Default selection is external. Only `aigw route fallback air` may stage a reversible, namespaced AIGW block. |
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

`aigw route doctor` is a separate local observer. It reports host policy,
sidecar attribution, and selection conflicts without running a client,
contacting a provider, reading credentials, exposing paths or configuration
bodies, or claiming session, endpoint, terminal, or billing proof. Junie is
always reported as not probed.

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
