# Refresh the stable supply chain

## Why

AIGW releases select a reproducible dependency graph, but reproducibility must
not preserve versions after newer stable releases are available. The selected application, test, and declared tool closure has stable updates,
so the next release candidate must refresh that closure and re-run the complete
product proof.

## What changes

- update the selected Go module graph to current stable versions;
- publish the verified result as `v0.1.0-rc.86` on independent Forge planes;
- reinstall the signed portable release and verify the current control plane.

## Reuse and removals

Go modules remain the sole dependency owner. No compatibility layer, second
resolver, provider conditional, or new product surface is introduced.

## Non-goals

- no Account, Profile, Route, credential, or client-projection behavior change;
- no Proxy, Workstation, ETHOS, JetBrains, or conversation-state authority;
- no requirement that GitLab and GitHub share commit identities.
