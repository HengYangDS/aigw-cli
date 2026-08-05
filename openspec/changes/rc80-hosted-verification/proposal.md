## Why

The rc.79 hosted matrix exposed two deterministic source defects. Evidence
verification treated a GitLab commit ID as portable across independent Forge
histories, while a Windows test borrowed the host Go executable as a Claude
launcher fixture. Both defects must be repaired forward as rc.80.

## What Changes

- Verify evidence by content tree in the current Forge history without fetching
  or depending on another Forge's commit object.
- Replace the host-tool lookup with a test-owned copy of the running test
  executable.
- Keep protected signing trust in Forge context, not product source.
- Advance release identity and Changelog forward to rc.80 only after proof.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `product-control-plane`: require provider-neutral tree verification in each
  Forge's own history and repository-controlled client fixtures on every native
  platform.

## Impact

Evidence verification, Windows launcher tests, version metadata, Changelog,
and release evidence change. AIGW routing, provider data, Proxy lifecycle, Codex
history, JSONL, SQLite, model metadata, and JetBrains applications do not.
