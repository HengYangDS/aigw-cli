# Output Identity Names

## Why

An audit of AIGW's human output after the group-alignment release found that
it names clients, protocols and projection surfaces inconsistently:

- Claude Desktop appears under three names: `Claude Desktop` in status and
  doctor, `claude-desktop` in the sync preview, route list and selection
  preview, and `Claude-desktop` in check, test, verify, migration, account and
  credential messages. Two helpers capitalize the client identifier instead of
  reading the label the client registry already declares.
- Help for `aigw check`, `aigw test` and `aigw verify` lists the `--for` values
  as labels such as `Claude Desktop`, which the flag refuses; it admits only
  identifiers such as `claude-desktop`.
- `aigw repair --dry-run` reports `claude-settings` as the surface of every
  projection except Codex's, so its human and JSON preview misname Claude
  Desktop's configuration library and Hermes's configuration file, and an
  explicit Codex home loses its identifier suffix.
- The migration and selection output print protocol identifiers such as
  `openai_responses` where the rest of the output uses protocol labels.

## What Changes

- Human output names each client by its registry label and each protocol by
  its protocol label. The two identifier-capitalizing helpers are removed.
- Help lists the identifiers that each `--for` flag accepts.
- Each client adapter declares the surface identity of every projection it
  plans, and repair previews report that identity. The surface registry gains
  `claude-settings` and takes over Hermes's `hermes-home-default` from its
  adapter.
- The migration preview shows Client Bindings and Recommendations in their own
  sections and counts native targets in words that agree with the number.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `product-quality`: gains the requirement "Human output names clients and
  protocols by their labels".
- `product-control-plane`: gains the requirement "Repair previews name each
  projection surface".

## Impact

Human output and help text change. JSON keeps its fields and identifiers,
except that `aigw repair --dry-run --json` corrects `surface_id` for Claude
Desktop (`claude-desktop-config-library`), Hermes (`hermes-home-default`, the
identity its discovery already reports) and explicit Codex homes (their full
identifier); Claude and the default Codex home keep their values. Flags,
accepted values, exit statuses and copyable commands are unchanged.
