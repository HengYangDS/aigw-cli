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
- The release that carries the removal is 0.6.0.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `command-line`: a renamed flag's earlier spelling is refused in the release
  after the one that hid it.

## Impact

`setup`, `add`, `install` and `uninstall` lose one flag each, and the CLI loses
the replaced-spelling warning. Documents, configuration, credentials and client
files do not change. Release journeys start from a baseline of 0.5.0 or
earlier, whose own steps keep that release's flags. Out of scope: other flags,
and `--model` and `--target` where they keep their meaning, as in `route add`
and `client enable`.
