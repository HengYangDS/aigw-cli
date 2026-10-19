# Credential Reader Continuity

## Why

A client keeps calling the versioned credential reader it loaded, so a
configuration written by a newer AIGW must stay readable by every earlier
reader. AIGW 0.4.4 added the `tool_search` Route capability to the canonical
configuration that every reader validates strictly. Once a Route declares it,
as the 0.4.5 team manifest does, every schema-six reader from 0.4.3 or earlier
refuses that configuration and returns no Token. A Codex session still calling
a 0.4.3 reader gets only "Cannot read AIGW configuration", which names neither
the reader's version nor the stage that failed.

## What Changes

- Keep the canonical configuration in the form every published schema-six
  reader accepts. Route capabilities outside the published schema-six
  vocabulary, today only `tool_search`, move to a separate configuration
  component that credential readers never read. The full program reads both,
  accepts the capability at its earlier canonical location, and moves it out
  at the next configuration write, including `aigw sync`.
- Apply a moved capability only while its Route keeps the Account, endpoint,
  upstream model and interface that qualified it, and drop it otherwise.
- Carry the component through commits, compensation, backups and verified
  checkpoints.
- When a credential reader cannot use the configuration, report the reader's
  AIGW version and the stage that failed. A read failure also names the
  operating-system operation and error, never the path. After a parse or
  validation failure the fix names `aigw sync` and a client reload.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `secret-storage`: gains the requirements "Configuration evolution preserves
  retained credential readers", "Stored capabilities follow their recorded
  Route" and "Credential configuration failures name the reader and stage".

## Impact

The configuration schema version, the reader's projection contract and its
Account, endpoint and fingerprint checks stay unchanged. Shipped reader bytes
are never rewritten. A predecessor program after a rollback no longer sees
`tool_search`, so its own synchronization turns off Claude Desktop tool search
until re-upgrade. Nothing is removed or renamed. Client configuration changes
only through the usual `aigw sync`.

Out of scope: rewriting client history or cached client commands, the Codex
default-model conflict, the GitLab catch-up and changes to native client
behavior.
