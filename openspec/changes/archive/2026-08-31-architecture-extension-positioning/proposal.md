## Why

AIGW's implementation already separates local configuration authority from API
traffic, but its public contract does not state how mature gateways compose with
that boundary or how a new Provider or Client enters without parallel semantics.
Making that contract explicit prevents unnecessary framework adoption and
provider-specific branching.

## What Changes

- Define general gateways and Codex Responses Proxy as optional Account
  endpoints rather than AIGW dependencies.
- Classify extensions as Account data, Account authentication, Client Adapter,
  or an independent data-plane concern.
- Require a mature dependency to remove more owned complexity than it adds.
- Keep the guidance in the existing architecture and Adapter admission owners;
  do not create another runtime abstraction or comparison document.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `product-control-plane`: make data-plane composition and the Provider/Client
  extension boundary explicit product requirements.

## Impact

This changes only the existing product-control-plane specification,
`docs/architecture/authority-and-projection-boundary.md`, and
`docs/governance/adapter-admission.md`. It adds no runtime dependency, Provider,
Client, compatibility path, or release surface.
