## Why

`aigw doctor` reports one check for each Codex configuration file that the
Codex binding projects into, and labels it with that file's position in the
binding, such as `Codex configuration target 1`. The output does not say which
file that is, so an operator cannot connect the check to a file, and with
several Codex homes the rows differ only by a number.

Doctor deliberately keeps private paths out of diagnostics, which operators
copy into reports. A path relative to the home directory, such as
`~/.codex/config.toml`, identifies the file without exposing the user name or
another private location.

## What Changes

- A Codex target check about a file under the user's home directory is labeled
  with its home-relative path, for example
  `Codex configuration ~/.codex/config.toml`, and its JSON check gains a
  `target` field with the same home-relative path.
- A Codex target outside the home directory is not named. Its human label keeps
  the position and says `(outside the home directory)`, and its JSON check has
  no `target`.
- The client adapter's local check carries the file it inspected, so doctor
  does not reconstruct it from configuration.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `product-quality`: diagnostics name the native file they inspect when it lies
  under the home directory, and never expose a private path.

## Impact

`internal/client` checks, the Codex adapter's inspection, doctor's labels and
JSON checks, and their tests change. The `target` field is additive; check
names, details, fixes and exit status do not change. No configuration, client
file, credential or projection changes.
