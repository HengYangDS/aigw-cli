# Project CI from actual Forge capability

## Why

AIGW requires native Windows evidence, but GitLab currently has no admitted
Windows executor. Emitting a GitLab Windows job with `allow_failure` confuses a
product requirement with one Forge's temporary capacity and leaves a pending or
non-authoritative job in the pipeline.

## Outcome

- keep one CUE authority for the complete product evidence model;
- declare native executor capacity independently for each Forge;
- project GitLab onto its available macOS and Linux runners;
- retain required macOS, Linux, and Windows evidence on GitHub.

## Boundary

This change does not weaken Windows support, infer native behavior from
cross-compilation, add another CI owner, or couple either Forge to the other.
