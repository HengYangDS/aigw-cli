# Git Fixture Hermeticity

## Why

Repository and Forge tests create temporary Git repositories. Those fixtures
must not inherit workstation signing, credential, or hook policy because host
configuration can block commits or silently redirect bare-repository hooks.

## What changes

- centralize complete unsigned-repository policy for changelog fixtures;
- bind bare Forge remotes to their own hook directory;
- exercise both boundaries with hostile global Git configuration.

## Non-goals

- no production Git, signing, Forge, or release behavior changes;
- no timeout increase, retry loop, or host-specific bypass.
