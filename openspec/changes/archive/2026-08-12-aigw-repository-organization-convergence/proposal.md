## Why

AIGW's current release identity is assembled from tags and CI environment rather than one repository-owned source, while accepted `dev` and release `main` are not locally converged. This obscures the product version and weakens local-first release reasoning.

## What Changes

- **BREAKING** Make a tracked repository version source authoritative for CLI, artifact, changelog, and release validation.
- Make local `dev` to `main` closeout a governed exact-head operation.
- Align repository configuration, editor normalization, quality entry points, and release documentation around one source graph.
- Remove duplicate or inferred version paths after proof.
- Keep Codex/Claude control-plane boundaries, Proxy independence, Forge independence, and external identity inputs unchanged.

## Capabilities

### New Capabilities

- `repository-organization`: one portable contract for version, branch lifecycle, quality entry points, and documentation ownership.

### Modified Capabilities

- `product-control-plane`: release metadata remains provider-neutral and does not acquire data-plane or workstation ownership.

## Impact

Affects Go release metadata/checks, changelog, tracked configuration, documentation, OpenSpec, and ETHOS proof inputs. Does not modify Codex session data, Proxy runtime state, credentials, or private signing inputs.
