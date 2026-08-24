## Why

Native Windows verification currently applies a POSIX `0600` assertion to the
newly created Codex configuration file. Windows does not represent that
owner-only policy through `os.FileMode`, so the test rejects the intended
portable behavior after the product transaction itself succeeds.

The same accepted-object publication updates peer `main` and `dev` atomically,
but only the `main` event receives a visible pipeline. The product should retain
one expensive verification graph per product commit while exposing a bounded
`dev` confirmation that the peer refs name the same already-verified object.

Repository Markdown also lacked a semantic format gate. A malformed table could
therefore remain invisible to both the formatter and table-specific lint rules,
and the documented requirement that every canonical document be reachable from
the documentation map had no executable proof.

## What Changes

- Keep the reversible Codex configuration journey platform-neutral.
- Assert POSIX owner-only configuration mode only on hosts that implement that
  representation.
- Add one lightweight `dev` push projection that verifies exact equality with
  the peer `main` object, without rerunning the complete product graph.
- Add locked Markdown format and lint gates, reject malformed table structure
  before parser-based lint, and prove canonical documentation is reachable from
  the declared reader entrypoints.
- Generate both Forge workflows from the existing CUE authority.

## Capabilities

### Modified Capabilities

- `product-quality`: distinguish portable behavior from POSIX permission
  representation and expose non-duplicated accepted-ref evidence.

## Impact

This change affects native acceptance tests, CI topology, generated Forge
workflows, Markdown presentation, documentation navigation, and their canonical
quality contract. It changes no credentials, client state, provider behavior,
release payload, or runtime configuration.
