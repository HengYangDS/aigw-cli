# Proposal

## Why

Hermes 0.21.6 keeps an installation's committed dependency environment and
managed tools under the data root that `HERMES_HOME` selects. AIGW verifies
Hermes in a fresh disposable home, which has neither. The version probe exits
with "no dependency environment is committed for this install", so
`aigw verify --for hermes` fails although the installed client and the selected
Route work.

## What Changes

- Reuse the configured Hermes home's installed dependency environment and
  managed tool store from the disposable verification home. The projected
  verification configuration stays isolated from the operator's file.
- Run every verification probe with Hermes' documented probe policy that
  refuses on-demand installation, so verification cannot synchronize,
  rebuild, or republish the shared installation state.
- Fail closed when the host cannot expose that state to the disposable home.
  Do not provision another environment, retry, or disclose private paths.
- Leave the operator's Hermes configuration, credentials, plugins, sessions,
  and launchers unchanged. Do not modify Hermes.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `product-control-plane`: Hermes verification must reuse the installation
  state of the configured Hermes home instead of requiring a provisioned
  disposable home, and must not change that state.

## Impact

The Hermes adapter's verification and its focused tests change. No dependency,
configuration owner, credential path, or client configuration write is added.

AIGW still owns only its disposable verification home. Hermes owns its
configuration, installation state, plugins, and launchers; verification reads
the installation state and changes none of them. The reuse stance follows the
per-task home layout that Hermes' own end-to-end tests exercise and the probe
policy its environment reference documents, rather than a wrapper, a copied
environment, or a vendor patch.

Non-goals: verification through a Hermes profile, loading operator plugins
during verification, Windows reparse-point alternatives to symbolic links, and
the release real-client fixture's own disposable Hermes home.
