# Authority and Projection Boundary

Status: canonical.

AIGW resolves a canonical local account/profile/route configuration and projects
only its marked Codex provider selection to every configured target. It does not
start a background service or carry request traffic.

```text
AIGW configuration -> all-target preparation -> atomic projection commit
                                           -> rollback on any failure
Codex Desktop      -> existing conversation model selection and transcripts
Codex DMX Proxy    -> local Responses compatibility and listener lifecycle
```

A projection comprises `config.toml` plus its `.aigw-state.json` sidecar. The
transaction captures byte-exact pre-state for both, including the absence of a
sidecar. It validates all targets before the first write; on failure it restores
every affected config and sidecar. The narrowly defined exact-truncation repair
remains part of preparation and does not weaken foreign-edit conflict checks.

`aigw sync --dry-run --json` exposes target/action only. It does not expose
credentials or config bodies and does not mutate files, bind native
authentication, restart clients, or modify conversation state.
