# Complete the publication topology

## Why

The accepted tree still uses a retired flat publication declaration. Current
publication admission therefore cannot recognize GitLab and GitHub as two
independent peers, even though the repository already owns the required CI,
verification, installation, and release surfaces.

## What changes

- declare the existing local verification and installation commands;
- declare GitLab and GitHub as independent publication peers;
- remove the retired flat remote fields instead of keeping a compatibility
  plane.

## Out of scope

- new wrappers, workflows, installers, identity defaults, or Forge coupling;
- changing provider credentials, remote history, or release assets.
