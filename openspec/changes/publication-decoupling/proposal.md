## Why

The Client Projection Provider adds drawing-only data, tests and quality
dependencies to AIGW. The user requires source repositories to contribute no
extra implementation merely to produce architecture diagrams.

## What Changes

- Transfer exact authored Claim Model and editorial inputs to the independent
  architecture-publications owner.
- Remove the Provider, installed drawing tests, unused carrier classes and
  documentation links.
- Retain the native Node text-quality test and all AIGW product checks.

## Capabilities

- `repository-organization`: keep unrelated external consumers out of the
  product's implementation and qualification dependency graph.

## Impact

Only drawing-specific carriers and their callers change. AIGW accounts, routes,
credentials, client synchronization, installation, coverage and release behavior
remain unchanged. Accepted architecture deliveries and source history remain.
