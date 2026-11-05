## Why

When `NO_COLOR` or `AIGW_ACCESSIBLE` is set, AIGW renders plain accessible
prompts, and its Token prompt then reads an ordinary terminal line. The
terminal echoes every Token character typed in `setup`, `rotate`, `use`,
account diagnostics and manifest import, which exposes the Token to the screen,
its scrollback and any screen share. The accessibility decision is also made
in two places: the CLI passes `NO_COLOR`, and the prompt reads
`AIGW_ACCESSIBLE` from the process environment itself.

## What Changes

- A Token prompt at an interactive terminal reads with echo disabled in every
  prompt mode. Token input that is not a terminal is read as before.
- The CLI decides accessibility once from its environment, and the prompt no
  longer reads the process environment.
- One hidden terminal reader serves every platform.

Non-goals: which variables select accessible prompts does not change, and
`--token-stdin` keeps its contract.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `secret-storage`: interactive Token input is never echoed.

## Impact

`internal/terminal` and the CLI's prompt wiring change, and a patch release
ships the fix. Configuration, credential stores and Token values do not
change.
