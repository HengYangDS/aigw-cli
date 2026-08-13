# Migrate the repository Commitment schema

## Why

The accepted repository Commitment carries retired permission fields, while a
tracked marker keeps obsolete private state inside the source tree. The current
ETHOS runtime therefore cannot compile a governed plan or reset local state.

## What changes

- migrate the repository Commitment to the current schema;
- retire the tracked private-state marker;
- add the canonical OpenSpec entrypoint;
- preserve AIGW product behavior and repository authority.

## Non-goals

- no provider, client projection, release, or runtime behavior changes;
- no compatibility layer or duplicate authority surface.
