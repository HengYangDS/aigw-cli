# ADR-0001: Separate AIGW Control Plane from Proxy Data Plane

- Status: accepted
- Date: 2026-07-14

## Context

Codex third-party Responses compatibility needs a local data-plane adapter,
while account selection, credentials, and multi-client configuration need a
control plane. Overlapping writers cause drift and ambiguous rollback.

## Decision

AIGW owns the canonical account manifest and marked provider projections across
Codex targets. Any explicitly selected compatibility service owns its outbound
request transformation and local lifecycle. AIGW has no transport-service
lifecycle API, and an external service never gains authority over an
AIGW-marked provider block. Codex CLI and Codex Desktop share the Codex Home
configuration, while Codex retains authority over existing conversation model
selection, transcripts, and Desktop-only GUI settings.

## Consequence

Projection recovery happens through AIGW's all-target transaction. Transport
recovery remains entirely within the selected service's product boundary.
Neither path edits historical conversations.
