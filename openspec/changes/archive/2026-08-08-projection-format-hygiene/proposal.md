# Projection format hygiene

## Why

OpenSpec projection added a trailing blank line that violates AIGW's canonical
text-layout contract and blocks the archive-head proof.

## What changes

Normalize the projected specification to one terminal newline. No product,
workflow, release, or runtime behavior changes.
