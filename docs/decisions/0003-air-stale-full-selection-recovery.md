# ADR-0003: Recover Only the Verified Stale Air Full-Selection Residue

- Status: accepted
- Date: 2026-07-18
- Owner: Yang HENG

## Context

Air's default authority is JetBrains AI. AIGW may stage only an explicit,
reversible namespaced fallback for Air; it does not own Air's persistent
provider selection, credentials, process lifecycle, conversations, JSONL,
SQLite state, or model metadata.

One observed corruption shape contains a complete AIGW-marked full selection
and a recognized AIGW fallback sidecar whose managed-block hash does not match
the full-selection block. The sidecar has no original provider or model value,
so ordinary restore cannot prove or reconstruct the former JetBrains
selection. A historical backup is not evidence of the current JetBrains
baseline.

This decision does not change the private GitHub Free provenance boundary in
ADR-0002. It requires no paid GitHub feature, repository-visibility change,
or rewrite of a provider-native tag or Release.

## Decision

Provide the narrowly admitted command:

```text
aigw route recover air --dry-run --json
aigw route recover air --confirm-host-idle
```

Recovery is admitted only when discovery identifies a present JetBrains-owned
Air configuration and all of the following hold:

1. the sidecar is AIGW-owned and declares `namespaced-fallback`;
2. the Air file has exactly one complete, generated AIGW full-selection block
   with AIGW-marked top-level selection lines and no fallback block;
3. the full-selection block has the generated provider shape and the sidecar
   hash does not match it; and
4. the sidecar has no stored original provider or model selection.

Every other state fails closed, including ordinary fallback, a full-selection
sidecar, foreign or incomplete sidecar data, duplicate markers, a changed
provider block, or retained original-selection values. A clean Air file with
no sidecar and no AIGW selection, markers, or provider tables is an idempotent
`already-external` result rather than an error; it authorizes no write. AIGW
residue without an attributable sidecar still fails closed without exposing
the configuration path.

The command removes only the AIGW-marked top-level selections, full-selection
provider block, mismatched sidecar, and Air's explicit AIGW target membership.
It returns Air to an unselected external baseline. It must not invent a
JetBrains `model` or `model_provider` value, read or change credentials,
start/stop/reload a native client, or access sessions or conversation state.

The dry run is credential-free and lock-free. Apply requires
`--confirm-host-idle`, which is an operator attestation rather than a process
probe. The adapter transaction rolls back config and sidecar artifacts on a
write failure; the CLI restores its control-plane configuration snapshot if
adapter recovery fails.

`aigw route doctor` reports `recoverable-stale-full-selection` and recommends
the recovery preview only for this exact condition. Other route-ownership
conflicts continue to recommend the ordinary read-only repair preview.

## Consequences

The recovery command is a corruption repair, not an Air-provider selection
feature. A later Air UI session remains the authority for proving JetBrains
login, endpoint behavior, billing, and a user-visible reply. After recovery,
Air is no longer an AIGW Codex target; a future AIGW fallback requires an
explicit `aigw route fallback air` request.

## Evidence

The adapter and CLI regression suites cover admission, rejection, rollback,
read-only preview, idleness confirmation, target removal, and route-doctor
guidance. Repository gates in `CONTRIBUTING.md` verify source, formatting,
governance, documentation, and provider-projection contracts.

## Revisit Trigger

Revisit this decision only if a recoverable original Air selection becomes
available, Air's authority changes, or a product requirement explicitly asks
AIGW to manage Air's persistent provider selection.
