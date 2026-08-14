# Check controlled Markdown links

## Why

The link gate expands a hidden-file glob across `.git/ethos/runtime`, so
third-party npm documentation can fail AIGW source verification. Private Git
runtime projections are not repository source.

## What changes

- derive the Markdown input set from Git-tracked files;
- run Lychee with that exact cross-platform path list;
- keep every tracked Markdown file in the existing source gate.

## Out of scope

- link-policy weakening or compatibility behavior;
- ETHOS runtime contents or lifecycle changes.
