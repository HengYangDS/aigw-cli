## Context

See [proposal](proposal.md). Readiness already resolves every enabled client
Route and owns the concise health result, but Token availability was checked
only for the default Route.

## Goals / Non-Goals

**Goals:** fail closed before an enabled client is called ready when its selected
Account Token is unavailable.

**Non-Goals:** probe every endpoint, add a new diagnostic command, change Token
storage, or introduce provider-specific behavior.

## Decisions

The existing readiness loop remains the sole owner. Immediately after resolving
each enabled client Route, it checks that Route's Account through the existing
secret-store interface and uses the existing Token recovery helper. Endpoint
stability probing remains limited to the default Route so `check` stays bounded;
full per-client protocol requests remain owned by `verify`.

This is preferred over a new aggregate readiness model or a second diagnostic
pass because both would duplicate Route resolution and create parallel health
semantics.

## Risks / Trade-offs

- A missing non-default Token now makes `check` fail where it previously passed
  incorrectly. This is the intended fail-closed behavior.
- The command still performs one bounded gateway probe. Full model-path proof
  remains an explicit `aigw verify` operation.
