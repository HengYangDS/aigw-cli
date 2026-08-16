# Publish AIGW v0.1.0-rc.83

## Why

The accepted source now infers the client from an explicitly selected
single-client profile. The fix is verified on the accepted tree but has no
published AIGW version.

## What Changes

- Advance the release identity to `0.1.0-rc.83`.
- Record the profile-client inference fix in the changelog.
- Publish the same product tree independently through GitLab and GitHub after
  native macOS, Linux, and Windows acceptance.

## Boundaries

This release changes no provider endpoint, credential, Proxy lifecycle,
JetBrains state, Codex conversation data, or model metadata. It introduces no
compatibility surface and no new product behavior beyond the accepted fix.
