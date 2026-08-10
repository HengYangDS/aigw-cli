## Why

The accepted tree passes its native source gate, but closure is still false:
two canonical OpenSpec purposes are placeholders, the locked Go graph has
stable updates, hosted platform and publication evidence is stale, and absorbed
Work Lanes remain visible. These are one release-quality closeout, not new
product scope.

## What Changes

- Replace placeholder specification purposes with concise authority statements.
- Verify every direct Go dependency is current, and leave transitive selection
  with the resolver instead of pinning modules the main build does not need.
- Re-run exact native and hosted verification, then publish independently to
  GitLab and GitHub.
- Install and verify the released product before governed lane retirement.

## Capabilities

### Modified Capabilities

- `product-control-plane`: make terminal quality, publication, runtime, and
  repository-lifecycle closure explicit and reproducible.

## Impact

- **Authority:** repository source and locks remain authoritative; Forges own
  only their independent publication planes.
- **Breaking changes:** no user-facing compatibility surface is retained or
  introduced.
- **Reuse:** the Go resolver and existing repository-native tools own dependency
  and release mechanics.
- **Non-goals:** no Proxy, Workstation, JetBrains, Codex history, or session
  ownership enters AIGW.
