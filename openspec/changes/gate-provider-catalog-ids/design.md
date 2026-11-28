## Context

ETHOS beta.12 (runtime `3e6b10f2`) resolves a gate provider through its packaged catalog
`ethos/contracts/gate-providers.toml`: `[providers]` maps each ID to a `module:callable`, and `[legacy]` maps each
former `module:callable` string to its ID. A legacy name resolves to the same callable and adds one
`gate_provider_legacy_name` note per gate run (`ethos/adapters/gates/verification.py`, `_legacy_notes`). The proof of
AIGW's dev head `36d2ab18` on 2026-11-28 reported that note for all eleven gates: three from AIGW's profile and eight
from ETHOS's own `ethos/resources/system/gates.toml`.

## Goals / Non-Goals

**Goals:** AIGW's profile names every gate provider by its catalog ID.

**Non-Goals:** ETHOS's default gates, whose names their owner changes; any change to what a gate runs or judges.

## Decisions

- **Name the IDs that the legacy table gives.** `ethos.adapters.gates.code_quality:behavior_report` maps to
  `provider.code-behavior` and `ethos.adapters.gates.code_quality:static_report` to `provider.code-static`; both IDs
  map in `[providers]` to the same callables, so each gate keeps its verifier. Keeping the legacy names was rejected:
  they work only while ETHOS ships the legacy table.
- **The test identifies the behaviour gates by the ID.** It already requires both behaviour gates to be found, so a
  profile and a test that disagree fail it rather than pass unchecked.

## Risks / Trade-offs

- An ETHOS runtime older than the catalog would not resolve the IDs. AIGW's runtime selector names beta.12, and its
  rollback beta.11 is not checked here; a rollback to it would be qualified on its own before use.
