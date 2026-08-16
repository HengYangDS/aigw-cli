# DR-0001: Separate AIGW Control Plane from Transport Data Planes

- Status: accepted
- Date: 2026-07-14
- Last amended: 2026-08-16

## Context

Codex third-party Responses compatibility needs a local data-plane adapter,
while account selection, credentials, and multi-client configuration need a
control plane. Overlapping writers cause drift and ambiguous rollback.

## Decision

AIGW owns the canonical account manifest and marked provider projections across
Codex targets. Any explicitly selected compatibility service owns its outbound
request transformation and local lifecycle. AIGW has no transport-service
lifecycle API, and an external service never gains authority over an
AIGW-marked provider block. Codex CLI and Codex Desktop share the native Codex Home configuration, while
Codex retains authority over existing conversation model selection, transcripts,
and Desktop-only GUI settings. Foreign applications and integrations remain
independent; AIGW neither depends on, configures, nor verifies them.

## Consequences

Projection recovery happens through AIGW's all-target transaction. Transport
recovery remains entirely within the selected service's product boundary.
Neither path edits historical conversations.

## Alternatives Considered

- **Mandatory all-in-one AI gateway:** rejected because local client use would
  depend on a traffic process, combining configuration, policy, and transport
  failure domains.
- **Client-specific switcher as the canonical store:** rejected because it
  couples Account and Token authority to one client's private layout.
- **Provider-specific code for every compatible endpoint:** rejected because it
  turns data admission into branching code without a protocol need.

Adjacent gateways and client switchers remain useful references, but their
feature breadth is not AIGW's product objective. AIGW optimizes for a small
authority surface, native-client configuration, explicit transactions, and
independent endpoint products.

## Revisit Trigger

Revisit only if AIGW intentionally becomes a transport runtime or an external
transport service becomes an admitted AIGW configuration owner.
