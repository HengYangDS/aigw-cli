# DR-0004: Use Account, Profile, and Route as the Configuration Authority

- Status: accepted
- Date: 2026-08-07

## Context

Provider endpoints, credentials, client model choices, and active selections
have different lifecycles. Encoding them in client adapters or inferring them
from provider and model names creates duplicated policy, hidden fallback, and a
high cost for adding a provider.

## Decision

An Account owns provider endpoints and one logical Token boundary. A Profile
owns one explicit `account + client + model` choice. A Route selects a default
or client-specific Profile before client execution. These entities are the
configuration SSOT; model and provider names remain transparent values.

Client adapters project this desired state but do not redefine it. Provider
diagnostics are optional leaf capabilities and cannot create a Profile, Route,
or hidden provider fallback.

## Consequences

Adding an ordinary provider changes configuration data rather than branching
client or routing logic. Switching a Route never copies a Token into client
files, and traffic is never retried through an unselected provider.

## Revisit Trigger

Revisit if AIGW adopts a different canonical domain model that preserves the
same explicit endpoint, credential, client-choice, and route boundaries.
