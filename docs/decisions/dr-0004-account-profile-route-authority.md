# DR-0004: Use Account, Profile, and Route as the Configuration Authority

- Status: accepted
- Date: 2026-08-07
- Last amended: 2026-09-13

## Context

Provider endpoints, credentials, client model choices, and active selections
have different lifecycles. Encoding them in client adapters or inferring them
from provider and model names creates duplicated policy, hidden fallback, and a
high cost for adding a provider.

## Decision

An Account owns provider endpoints and one logical Token boundary. A Profile
owns one explicit `account + client + model` choice. Each client's Route selects
one compatible Profile before execution; no global default spans clients.
These entities are the
configuration SSOT; model and provider names remain transparent values.

An imported recommendation is not a Route selection. It remains data in the
same configuration owner until setup or synchronization can fill an unselected
client. Explicit selections are preserved when their credentials are temporarily
unavailable. Keeping these meanings separate avoids both silent reselection and
a recommendation that prevents another connected Account from becoming usable.

Client adapters project this desired state but do not redefine it. Provider
diagnostics are optional leaf capabilities and cannot create a Profile, Route,
or hidden provider fallback.

## Consequences

Adding an ordinary provider changes configuration data rather than branching
client or routing logic. Switching a Route never copies a Token into client
files, and traffic is never retried through an unselected provider.

## Active redesign

The active [engineering-reference Change](../../openspec/changes/engineering-reference-convergence/design.md#9-redesign-intent-before-extending-adapters) revisits this decision: reusable Profiles and one explicit client binding replace client-bound model duplication and parallel Route/Adapter selection. This is an implementation target, not a claim that the new schema has shipped. Existing installed configuration stays on its accepted schema until migration and native candidate acceptance pass.

## Revisit Trigger

Revisit if AIGW adopts a different canonical domain model that preserves the
same explicit endpoint, credential, client-choice, and route boundaries.
