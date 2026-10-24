# Design

## Context

`configuration.ClientSpec` owns each client's identifier and display label, and
`configuration.ProtocolLabel` owns protocol labels. Two helpers capitalize
identifiers instead: `invocation.Title` in the command layer and `title` in
the credential validator, which turn `claude-desktop` into `Claude-desktop`.
Several renderers print identifiers directly. Help for check, test and verify
builds its `--for` choices from `AdmittedClientLabelUsage`, while their
argument validation admits only identifiers. The repair renderer assigns
`claude-settings` to every plan that is not Codex's, because the surface
registry names only Codex homes and the Desktop library, and it looks Codex
homes up again in discovery. The Hermes adapter keeps its own surface
identity, `hermes-home-default`, outside the registry.

## Goals / Non-Goals

**Goals:** one name for each client and protocol in human output; help that
lists the values a flag accepts; repair previews that identify the projected
surface.

**Non-Goals:** accepting labels as flag values; renaming identifiers, Accounts
or Routes; changing projection actions or JSON fields.

## Decisions

### The registry label is the human name

Human prose, row labels and problems name a client through one accessor,
`configuration.ClientLabel`, which reads `ClientSpec.Label`, and a protocol
through `ProtocolLabel`. Values a user types, such as flag choices and
commands, keep identifiers. Capitalizing an identifier was rejected because it
invents a third name that matches neither the label nor the value to type.

### Help lists what the flag accepts

Every `--for` usage line lists identifiers through `AdmittedClientUsage`, as
`aigw use --help` already does, and `AdmittedClientLabelUsage` is removed.
Accepting labels as aliases was rejected: it would add a second input
vocabulary to every command to repair a help-text defect.

### Adapters declare their surfaces

`client.ProjectionPlan` carries the surface identity of its target, set by the
adapter that plans it: Claude's settings, the Desktop library, the Hermes home
and the identity of each Codex target as its target reference already records
it. Repair reports that value instead of guessing, so it no longer needs
discovery to name a surface. The surface registry gains `claude-settings`,
which repair already reported for Claude, and takes over `hermes-home-default`,
which the Hermes adapter declared privately for its discovery.

### Migration groups its client rows

The migration preview lists the schema and counts, then Client Bindings and
Recommendations as sections of client rows. One row per recommended client
replaces the `client=route` list, which printed identifiers in prose.

## Risks / Trade-offs

- [Automation reads repair `surface_id`] → only incorrect values change;
  Claude and the default Codex home keep theirs.
- [Scripts parse human output] → JSON keeps identifiers and remains the stable
  interface.

## Migration Plan

None; the next release renders the new names.

## Open Questions

- None.
