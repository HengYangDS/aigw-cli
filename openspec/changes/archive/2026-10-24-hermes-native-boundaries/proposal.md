# Hermes Native Boundaries

## Why

AIGW 0.4.14 deletes a Hermes verification home once no process holds its
gateway lock. Codex's independent qualification blocked that criterion (BLOCK
`505ad363`): a released lock does not prove that the gateway process exited.
Hermes 0.22.5 confirms it. On a graceful exit the gateway releases its pid file
and `gateway.lock` first, then writes its exit records and drains up to one
second of logs into the home before the process ends, so that tail can
recreate files in a home that verification just deleted.

Separately, a Hermes CLI session freezes the profile configuration it was
created with. After `aigw use` selects another Route or a forwarding endpoint,
only new Hermes sessions use it, and AIGW does not say so.

## What Changes

- Hermes keeps `gateway_state.json`, with the gateway's process identifier, in
  the home after the gateway stops. A verification home counts as free of its
  gateway only when its gateway lock is free and that recorded process has
  exited. The bounded wait, the single native stop and the home's remover all
  use this one check, so the remover keeps a home, and names it, while the
  recorded process still runs.
- The client registry records that existing Hermes sessions keep the route they
  were created with. When `aigw use`, or its preview, changes the Hermes
  projection, human output adds an "Existing sessions" row: they keep their
  route, and new sessions use the selection.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `product-control-plane`: gains the requirement "Hermes verification waits for
  its gateway process to exit".
- `route-client-selection`: gains the requirement "Hermes selection says
  existing sessions keep their route".

## Impact

`aigw verify --for hermes` may wait up to the existing bounds for the gateway's
exit tail, normally about a second longer. `aigw use --for hermes` prints one
more row when the projection changes; its JSON output is unchanged. No vendor
source, operator Hermes home, credentials or sessions change, and no signal is
sent to any process.
