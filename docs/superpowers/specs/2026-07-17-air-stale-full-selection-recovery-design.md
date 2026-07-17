# Air stale full-selection recovery design

## Decision

Add one explicit recovery command for a narrowly identifiable Air corruption
shape. The command removes only an AIGW-owned full-selection projection and
its mismatched fallback sidecar, then removes Air from AIGW's configured Codex
targets. It does not write a JetBrains model selection, authenticate a client,
start or stop Air, or read or modify conversations, JSONL, or SQLite state.

This is a local AIGW behavior repair. It does not require GitHub Pro, a public
repository, paid GitHub features, or platform-enforced immutable release tags.

## Context

The approved authority split is:

- standalone Codex CLI and later/new ChatGPT Desktop defaults: AIGW;
- PyCharm, Junie CLI, and Air's persistent default: JetBrains AI;
- Air AIGW use: an explicit, reversible namespaced fallback only.

The live Air configuration has an invalid mixed state:

1. a complete AIGW full-selection block, including AIGW-marked top-level
   `model_provider` and `model` selections;
2. an AIGW sidecar marked as `namespaced-fallback` rather than
   `full-selection`;
3. a sidecar block hash that does not match the complete full-selection block;
4. no stored original provider or model from which a full-selection restore
   could safely reconstruct the former Air selection.

Existing `route restore air`, `route fallback air`, `repair --dry-run`, and
`sync --dry-run` must continue to fail closed for this state. Restoring a
historical DMX backup is explicitly out of scope because it is not proof of a
JetBrains AI baseline.

## Command contract

Add:

```text
aigw route recover air --dry-run --json
aigw route recover air --confirm-host-idle
```

`recover` accepts only `air`. Its default is a read-only preview. The apply
form requires `--confirm-host-idle`, which is an operator attestation and must
not probe, start, stop, reload, or restart Air.

The secret-free preview reports:

- `surface_id: "jetbrains-air-codex"`;
- `authority: "jetbrains-ai"`;
- `projection_mode: "none"`;
- `action: "recover-stale-full-selection"`;
- whether AIGW's configuration will remove the explicit Air fallback target.

It must not render configuration bodies, paths, endpoints, credentials,
session metadata, or backup contents.

## Admission and refusal rules

Recovery is admitted only when every condition below is true:

1. discovery identifies a present, JetBrains-owned Air configuration that
   admits a manual fallback;
2. a recognized `aigw-cli` sidecar is present and declares
   `namespaced-fallback`;
3. the current configuration contains exactly one complete AIGW
   full-selection marker and no AIGW fallback markers;
4. the top-level `model_provider` selection is exactly
   `"aigw" # managed by AIGW`;
5. an optional top-level `model` selection, if present, is marked as managed
   by AIGW;
6. the AIGW provider block has the exact generated shape: one AIGW table,
   AIGW-labelled name, non-empty base URL, Responses wire API, OpenAI auth
   requirement, and the matching end marker;
7. the sidecar's managed-block hash does not match that full-selection block;
8. the sidecar contains no original provider or model values.

Every other state fails closed. In particular, recovery refuses a normal
fallback, a foreign or incomplete sidecar, a changed provider block, duplicate
markers, an AIGW fallback block, or a full-selection sidecar that could be
restored through the ordinary reconciliation path.

## Recovery semantics

The adapter builds a synthetic *legacy full-selection* restoration state from
the verified current full-selection block. It uses the existing
full-selection-removal algorithm to remove only:

- the AIGW-marked `model_provider` line;
- the AIGW-marked `model` line when present;
- the AIGW managed-provider marker and provider table; and
- the mismatched AIGW sidecar.

All remaining Air configuration bytes remain under Air/JetBrains ownership.
Because the missing original selection is unrecoverable, the resulting file is
an external, unselected Air baseline rather than a fabricated
`model_provider = "jetbrains"` setting. A later Air UI session is still the
authority for proving JetBrains authentication and user-visible behavior.

The CLI removes Air from the AIGW Codex adapter target list in the same
transactional command. This prevents a later generic `aigw sync` from
re-staging an AIGW fallback without an explicit `aigw route fallback air`
request.

## Atomicity and rollback

The adapter recovery prepares a two-artifact transaction: the Air config and
its sidecar. It uses the existing snapshot, compare-before-write, atomic write,
remove-if-unchanged, and rollback primitives. The CLI snapshots the AIGW
control-plane config before removing Air from the target list; if adapter
recovery fails, it restores that control-plane snapshot exactly.

No account token is read, bound, rotated, or deleted. No native client process
is executed.

## Diagnostics and documentation

`aigw route doctor` recognizes this exact state as
`recoverable-stale-full-selection` and directs the operator to
`aigw route recover air --dry-run`. Other ownership conflicts keep their
existing fail-closed guidance.

README, security guidance, terminal experience contract, and changelog describe
the recovery as a narrow corruption repair, not a way to choose Air's default
provider or bypass JetBrains ownership.

## Verification

Tests must prove all of the following:

1. the preview is read-only, secret-free, performs no authentication, and does
   not take the mutation lock;
2. the admitted mixed state previews and applies successfully, removes only
   AIGW-managed full-selection content and the sidecar, and removes Air from
   AIGW's target list;
3. apply requires `--confirm-host-idle` and does not execute a client;
4. changed/foreign/duplicate markers, a normal fallback, a full-selection
   sidecar, or original-provider data are rejected without writes;
5. an injected adapter write failure restores the AIGW control-plane config;
6. `route doctor` gives the recovery command only for the exact recoverable
   shape;
7. after recovery, `route doctor` reports Air as externally JetBrains-owned and
   `aigw sync --dry-run` has no implicit Air fallback action.

Full repository quality gates and GitLab/GitHub provider-projection contracts
remain required before publication. Runtime JetBrains login and a visible Air
reply are deliberately separate, user-visible acceptance evidence.
