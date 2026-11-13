## Why

Node.js 26.13.1 'Lithium', the 26 LTS line's security release of 2026-11-11, fixes 16 vulnerabilities, seven of them
rated High: CVE-2026-123863, CVE-2026-123864, CVE-2026-123865, CVE-2026-123866, CVE-2026-123872, CVE-2026-123874 and
CVE-2026-126051, mostly in http2 and tls. The repository runs its Markdown, OpenSpec and audit tooling on Node 26.13.0,
pinned in `.config/mise/config.toml` and `.config/mise/mise.lock`.

## What Changes

- Advance Node to 26.13.1 in the Mise configuration and in the lock for all six platforms.
- Mise 2026.11.7 and the other routine updates stay with the next supply refresh, because the Windows runner's Mise is
  pinned by digest and needs a guest window.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- None.

## Impact

`.config/mise/config.toml` and `.config/mise/mise.lock`. Node is development tooling and is not part of the
released program. No behavior or format change.
