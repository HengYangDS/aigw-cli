# Correct the rc.85 release date

## Why

The rc.85 source is ready, but its provider-native tags and Releases will be
created on 2026-08-17. The unpublished Changelog heading still records
2026-08-16, so publishing it unchanged would make the release chronicle false.

## What changes

- record 2026-08-17 as the rc.85 release date;
- preserve the release version, source, behavior, and artifact contract;
- require exact-head proof and refreshed Forge projections before tagging.

## Boundaries

This change does not alter product behavior, dependencies, CI, release assets,
provider configuration, or any previously published tag or Release.
