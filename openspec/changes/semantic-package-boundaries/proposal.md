# Semantic package boundaries

## Problem

Repository package and tool surfaces use concatenated implementation labels,
obscure ownership, split one CI responsibility across parallel commands, and
permit repository tooling to depend on product runtime owners.

## Change

Replace concatenated package and tool names with readable single-concept
owners, merge CI validation and execution into one command plane, and enforce
the intended dependency direction and direct package topology through the
architecture gate.

## Boundary

The cutover introduces no compatibility layer, forwarding package, alias, or
parallel invocation path. Command paths, imports, CI, docs, and release metadata
move atomically. Provider implementations remain extensible beneath their
existing domain owner; admitting a new top-level domain remains an explicit
architecture decision.
