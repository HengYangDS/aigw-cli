# Documentation Information Architecture

## Why

The documentation tree mixes content documents with container-named
`README.md` files and places experience, security, and evidence material under
broader governance or operations labels. Readers can reach most pages from the
root index, but the physical layout does not consistently express the semantic
owner of each document.

## What Changes

- Keep `docs/README.md` as the single documentation entry point.
- Replace subdirectory `README.md` carriers with semantic filenames.
- Make the Decision Record checker consume the semantic register name rather
  than preserving a container-name exception.
- Place product concepts, security architecture, terminal experience, text
  layout, release evidence, and the decision register under their owning
  information domains.
- Update every tracked link in the same atomic change.
- Keep small directories free of redirect-only local indexes; the root index
  and semantic registers provide navigation.

## Capabilities

### Modified Capabilities

- `repository-organization`: make documentation names, categories, and
  navigation reflect one semantic information architecture.

## Boundaries

- No product behavior, client projection, provider, release, or dependency
  changes.
- No duplicate compatibility paths or forwarding documents remain.
- Historical archived OpenSpec records are not rewritten.
