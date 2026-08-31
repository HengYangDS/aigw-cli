## Context

AIGW already owns route resolution, credential lookup, adapter checks, and provider diagnostics. Human `check` currently performs these operations inline and exits on the first failure; `status --json` exposes a different, non-probing view. The change must preserve read-only semantics and avoid a parallel readiness implementation.

## Goals / Non-Goals

**Goals:**

- Evaluate all admitted clients through one typed readiness result.
- Render that result as human text or JSON.
- Preserve actionable exit status and secret-free output.

**Non-Goals:**

- Changing route selection, credential storage, provider adapters, or proxy ownership.
- Making optional unselected catalogue entries required.

## Decisions

1. Keep `check` as the authoritative active-route probe and add a JSON renderer beside the existing human renderer.
2. Reuse the existing `routeStatus`/readiness data model where it covers the contract; add only fields needed to explain probe outcomes.
3. Return one complete document before deciding the process exit code, so automation receives diagnostics for all active clients rather than an opaque first error.
4. Keep `status` focused on static configuration facts; do not merge its semantics into `check`.

## Risks / Trade-offs

- Provider probes can be slow; the existing bounded diagnostic policy remains the single timeout owner.
- JSON consumers need a stable schema; acceptance tests will pin field names and secret-free guarantees.
