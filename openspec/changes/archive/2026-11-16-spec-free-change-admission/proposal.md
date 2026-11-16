## Why

AIGW's OpenSpec gate fails on every finding of the validator, whatever its level. Since OpenSpec 2.0 a
Change that changes no specification declares `skip_specs: true` in its metadata, and the validator answers
with a finding of its own: read on 2026-11-16 with the bundled 2.0.0, such a Change is `valid` and carries one
issue at level `INFO`, path `file`, which begins `skip_specs is set in .openspec.yaml` and says that the
Change declares no change of behavior at the level of a specification. The gate fails on that
acknowledgement, so every Change of this repository must carry a delta.

A Change that changes no behavior then has to add text. Fifteen archived Changes modified the requirement
"Latest stable repository-owned supply chain", which holds 17 scenarios at `9a3cf734`, and a fix of a defect
against a scenario that is already accepted has nothing true to add.

## What Changes

- A Change that declares `skip_specs: true` names, in its proposal, the accepted requirement and scenario
  that already state its behavior, or says that it changes no behavior of the product; the repository's
  OpenSpec rules require it.
- The OpenSpec gate passes an active Change that declares `skip_specs: true`, holds no delta and has the
  validator's acknowledgement of that declaration as its only finding. The gate's summary names each such
  Change.
- Every other finding fails the gate as before: another finding of that Change, any other `INFO`, and the
  same acknowledgement for a Change whose metadata does not declare `skip_specs: true`.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `product-quality`: "Warnings are owned failures" excepts the acknowledgement of a declared `skip_specs` and
  gains the scenario "A Change declares that it changes no specification".

## Impact

`tools/ci`: the OpenSpec gate and its tests. No product code, no released behavior.

- **Authority boundary:** OpenSpec decides what a valid Change without a delta is, and refuses the
  declaration when a delta exists. The gate admits only what the Change's own metadata in the checkout
  declares; it adds no severity policy of its own.
- **Reuse:** the gate's existing reader of the validator's report.
- **Non-goals:** no admission of another `INFO` finding; no change to ETHOS's validation, which already
  admits such a Change; no rewrite of a requirement that earlier Changes extended.
