## REMOVED Requirements

### Requirement: Synchronization adopts a Hermes or Codex model choice

**Reason**: `client-chosen-model` owns a model chosen in a client, for every command that writes the client's files
and not for `aigw sync` alone.

**Migration**: None for a configuration or a client file. A script that ran the Fix of a synchronization that found
no added model for the client's choice now receives the command that restores the recorded selection; the command
that adds the chosen model is named beside it.
