## ADDED Requirements

### Requirement: Repair previews name each projection surface

Each client adapter SHALL declare the surface identity of every projection it
plans, and repair previews SHALL report that identity for each projection in
human and JSON output.

#### Scenario: Preview repair for every admitted client

- **WHEN** `aigw repair --dry-run --json` plans projections for Claude, Claude
  Desktop, Codex and Hermes
- **THEN** their `surface_id` values SHALL be `claude-settings`,
  `claude-desktop-config-library`, the identity of the Codex target's home and
  `hermes-home-default`
- **AND** no projection SHALL report the surface of another client.
