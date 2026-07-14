# ADR-0001: Separate AIGW Control Plane from Proxy Data Plane

- Status: accepted
- Date: 2026-07-14

## Context

Codex third-party Responses compatibility needs a local data-plane adapter,
while account selection, credentials, and multi-client configuration need a
control plane. Overlapping writers cause drift and ambiguous rollback.

## Decision

AIGW owns the canonical account manifest and marked provider projections across
Codex targets. Codex DMX Proxy owns outbound request sanitization and its local
service lifecycle. AIGW has no proxy lifecycle API; the proxy never rewrites an
AIGW-marked provider block. Existing Codex conversation model selection and
transcripts remain under Codex Desktop authority.

## Consequence

Projection recovery happens through AIGW's all-target transaction. Transport
recovery happens through the proxy's manifest-verified deployment. Neither path
edits historical conversations.
