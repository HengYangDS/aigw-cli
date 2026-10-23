## Why

A Renovate `dryRun=full` candidate calculation with the repository policy on
GitHub `dev` (`9185e204`, 2026-10-23) admits seven tool updates. Node 26.12.1
is a security release that fixes five High-severity CVEs in TLS, DNS and brotli
handling, and uv 0.13.5 rejects certificate chains containing the compromised
eMudhra intermediate keys through rustls-webpki 0.103.16. GoReleaser 2.19.0 is
built with Go 1.27.3 and updated `golang.org/x` modules. npm 12.2.1, gh
2.103.0, the Renovate image 44.178.5 and mise-action v5.2.2 are the current
stable releases.

The same calculation could not reach the Go module proxy or Docker Hub, so it
audited no Go module and not the Mise image.

## What Changes

- Advance Node to 26.12.1, npm to 12.2.1, uv to 0.13.5, gh to 2.103.0,
  GoReleaser to 2.19.0 and the Renovate image to 44.178.5 in the Mise
  declaration, and regenerate their lock entries with `mise lock`.
- Advance mise-action to v5.2.2 in the CUE authority and regenerate both Forge
  projections.
- Hold the Windows native client pins. Claude Code 2.1.304 is admissible, but
  Codex 0.164.0 passes the release age only on 2026-10-26, so both move together
  in one native client journey.
- Hold the Go modules and the Mise image as unaudited until their datasources
  answer.
- The supply-chain requirement gains a scenario: a dependency whose datasource
  the candidate calculation cannot reach stays unaudited, not current.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `product-control-plane`: the latest-stable supply-chain requirement gains a
  scenario for a candidate calculation that cannot reach a dependency's
  datasource.

## Impact

Developer, CI and release tool declarations, the Mise lock and both Forge
projections change. AIGW programs, installed AIGW, user configuration and
credentials do not change; the next release is packaged with GoReleaser 2.19.0.
