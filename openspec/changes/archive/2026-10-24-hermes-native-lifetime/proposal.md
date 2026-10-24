# Hermes Native Lifetime

## Why

Codex's independent qualification of AIGW 0.4.13 found three defects around
the native Hermes client, and this host confirms the first:

- `aigw verify --for hermes` deletes its isolated Hermes home as soon as the
  probe returns, although the one-shot chat auto-started a detached gateway
  for that home. The gateway keeps running for its 10-minute idle period,
  recreates state in the deleted home and, in Codex's reproduction, logged
  `FATAL: state.db was replaced underneath the gateway`. Each of this host's
  four recent Hermes verifications left such a recreated home behind. The
  gateway also shared the operator's host lock directory.
- The shared native diagnostic classifier recognizes warning words but not
  warning symbols, so Hermes's `⚠ install out of sync …` on a zero exit is
  reported as completed verification.
- The release journey continues its Hermes session with creation options
  that Hermes 0.22.5 refuses on resume (exit 2) before any inference.

## What Changes

- The verification home's configuration gives the auto-started gateway a
  2-second idle exit, and the gateway keeps its host locks inside that home.
- After the probes, verification waits for the gateway's per-home lock to be
  free. If the gateway has not exited by the idle bound, verification asks it
  once to stop through `hermes gateway stop` with closed input and waits
  again. The home is deleted only after the lock is free; otherwise it is
  retained and verification reports which home it kept.
- The home's remover itself refuses to delete a home whose gateway lock is
  held.
- A diagnostic line that begins with an explicit warning or error symbol,
  `⚠`, `✗`, `✖`, `✘` or `❌`, counts as a native warning; check marks and
  ordinary session and progress lines do not.
- The release journey creates its Hermes session once with its policy, then
  resumes it by the stored session identifier without creation options, and
  stops the journey's gateway before deleting the journey home.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `product-control-plane`: gains the requirement "Hermes verification
  releases its gateway before deleting its home".
- `product-quality`: gains the requirements "Native warning symbols are
  diagnostics" and "The Hermes release journey resumes its session natively".

## Impact

`aigw verify --for hermes` may take a few seconds longer while its gateway
exits, and it now fails with a named retained home instead of deleting a home
under a live gateway. Codex and Claude verification, CI tooling and release
evidence that use the shared classifier also treat line-leading warning
symbols as warnings. No vendor source, operator Hermes home, credentials or
sessions change.
