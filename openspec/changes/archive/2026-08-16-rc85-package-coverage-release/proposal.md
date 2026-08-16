# Publish AIGW v0.1.0-rc.85

## Why

The accepted product tree now enforces statement and branch coverage strictly
above 95 percent at both aggregate and package scopes. The contract is proven
locally but has no published AIGW version.

## What Changes

- Advance the release identity to `0.1.0-rc.85`.
- Record the stricter quantitative quality contract in the Changelog.
- Make the same product tree available for independent GitLab and GitHub
  publication after each Forge passes its own release gates.

## Boundaries

This release changes no provider endpoint, credential, client projection,
Proxy lifecycle, JetBrains state, Codex conversation data, or model metadata.
It introduces no compatibility surface.
