## Why

AIGW 0.5.0 renamed four flags and kept each earlier spelling for one release,
hidden and with a warning on standard error: `setup --model` and `add --model`
became `--upstream-model`, and `install --target` and `uninstall --target`
became `--path`. That release is out, so the earlier spellings and the
machinery that only they use can go, as the command-surface-contracts design
decided.

## What Changes

- **BREAKING:** `setup --model`, `add --model`, `install --target` and
  `uninstall --target` are removed. Each now fails as an unknown flag before
  the command reads a Token or writes anything; scripts use `--upstream-model`
  and `--path`.
- The warning that named a hidden spelling's replacement, and the mapping that
  wrote a preview's apply command in the replacement's spelling, are removed
  with them.
- A flag error, such as an unknown flag or a flag without its value, names the
  help of the command that refused it, which lists `--upstream-model` and
  `--path`. It named top-level help or `aigw check` before.
- The release that carries the removal is 0.6.0.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `command-line`: a renamed flag's earlier spelling is refused in the release
  after the one that hid it, and a flag error names the command's help.

## Impact

`setup`, `add`, `install` and `uninstall` lose one flag each, and the CLI loses
the replaced-spelling warning. A flag error names the help of the command that
refused the flag, in text and, when `--json` comes before that flag, in the
JSON failure document's `error` and `next_action`; the credential helper keeps
its own refusal. No JSON document gains or loses a field, and configuration,
credentials and client files do not change. The release journeys read each
program's path flag from its own help, so they still start from any published
release. Out of scope: other flags, and `--model` and `--target` where they
keep their meaning, as in `route add` and `client enable`.
