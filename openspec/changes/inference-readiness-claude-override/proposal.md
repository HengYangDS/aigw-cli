# Inference-scoped readiness and Claude model preferences

## Why

`aigw check` currently authenticates through a model catalogue request that
does not carry the selected Route's model. A catalogue can list a model while
inference returns `503 ... no available channel (distributor)`; the 2026-09-23
incident had exactly that shape. The existing `ModelUnavailable` diagnostic
cannot observe that refusal through the current request path.

Claude Code can also change only its native model preference after AIGW projects
an endpoint, credential helper, and default model. The recorded AIGW sidecar can
prove that the connection is unchanged, yet read-only inspection currently calls
the client invalid. On 2026-09-24, a native `opus[1m]` request succeeded while
the AIGW default Route remained Fable. Reprojecting that Route would erase the
user's selected model.

## What Changes

- Add one bounded, model-carrying inference request for each admitted
  Account-Token protocol. Keep the existing catalogue request as the explicit
  endpoint-only observation.
- Make `check` use inference scope by default and add `--endpoint-only`.
  Report the performed diagnostic scope and use `inference_checked` only when
  the selected Route's exact wire model was accepted by the endpoint.
- Recognize a model-only Claude Code preference when the attributed sidecar
  proves the helper, endpoint, and managed credential fields unchanged. Preserve
  its exact bytes. Because a native alias need not be the Route's wire model,
  use endpoint scope for that client and direct the operator to
  `aigw verify --for claude` for native-model proof.
- Keep authentication failures and other refused inference observations bounded
  and classified without mutating configuration or client files.
- Curate the shipped team manifest as Account, canonical Model, provider Route,
  and per-client Recommendation declarations. Keep stable IDs lower-case and
  provider wire IDs exact; derive ordinary Route names from Account and Model
  labels while retaining explicit channel distinctions. A provider catalogue
  listing alone does not admit a Route.
- Project one CUE-owned native CI matrix to both independent Forges. Each peer
  must execute its own required jobs; runner availability cannot silently
  shrink the product gate.
- Resolve overlong OpenSpec requirements at their semantic owners and require
  zero native validation findings, including informational advice, in the
  existing quality gate.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `cli-readiness`: distinguish endpoint, inference, and native-model-override
  evidence in human and JSON checks.
- `product-control-plane`: bind each authenticated check to its actual Route,
  protocol, model, and performed scope.
- `projection-format`: accept a sidecar-proven Claude model preference during
  read-only inspection without adopting connection or credential edits.
- `ci-diagnostics` and `product-quality`: require the complete native matrix
  on each selected Forge without a parallel CI authority.

## Impact

Affected owners are `internal/credential`, `internal/diagnostics`,
`internal/readiness`, `internal/configuration`, `internal/claude`,
`internal/client`, the affected CLI presentation packages, and the CUE CI
projection. The terminal experience documentation and native acceptance
journeys change. An ordinary `check` incurs one capped inference
request per enabled Account-Token client whose exact wire model AIGW owns;
`--endpoint-only` retains the previous request scope.

## Non-Goals

AIGW remains a local configuration control plane. It does not proxy requests,
manage upstream distributors, perform per-request failover, resolve Claude Code
aliases, or alter Codex conversation state. The probe makes no promise about
future model availability or a provider's retention policy. AIGW does not
persist the probe response; it requests no storage only where the wire protocol
offers that control.
