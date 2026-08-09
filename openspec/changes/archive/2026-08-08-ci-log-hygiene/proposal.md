# Clean hosted CI diagnostics

## Why

Successful hosted verification still emits Git's default-branch initialization
hint. The warning obscures actionable diagnostics and makes a green run look
partially unhealthy.

## What changes

- declare `main` as Git's process-scoped default branch in GitHub workflows;
- enforce the declaration in the existing Go workflow contract;
- preserve all verification, provenance, and publication gates.

## Non-goals

- no product, provider, release, runner, or Forge-topology behavior changes;
- no global Git configuration and no diagnostic suppression.
