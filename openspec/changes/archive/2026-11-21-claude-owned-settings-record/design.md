## Context

AIGW keeps a record beside each client file that it writes. For Claude Code's `settings.json` the record
(`settings.json.aigw-state.json`) holds what the owned settings were before AIGW and a digest of what AIGW wrote.
The published 0.8.3 decodes it with unknown fields refused, and computes the digest over `apiKeyHelper`, `model` and
four environment keys. A reader's report of the 0.8.3 source, with file and line for each statement, is kept in the
main checkout at `build/evidence/release-090-rollback-20261121/predecessor-readers.md`, beside the failed
acceptance's output.

## Goals / Non-Goals

**Goals:** 0.8.3 synchronizes after a rollback from the next release with Claude Code enabled; the next release
finds its map again afterwards; a later release can own one more settings key without repeating this.

**Non-Goals:** making 0.8.3 maintain or withdraw a map that it does not know; changing `aigw self rollback`.

## Decisions

### The first record keeps the fields and the digest that 0.8.3 reads

- **Rests on:** the acceptance's failure of 2026-11-21 (the unknown field), and the reader's report: 0.8.3 would
  refuse a second time, as an outside edit, once the map entered the digest, even with the field gone; it leaves a
  settings key that it does not manage in place and rewrites the file re-indented.
- **Considered:** a version 2 of the record, which 0.8.3 refuses as "incomplete or not owned"; carrying the map's
  earlier value inside the record's map of environment values, the one place that the strict decode does not
  constrain, which would hide one fact inside another's field; refusing `aigw self rollback` until Claude Code is
  disabled, which the accepted requirement on a supported release pair does not allow.
- **Chosen because:** the record's shape is a published contract with a strict reader, and nothing that 0.8.3 must
  read can change.
- **Applies to:** `originalSettings`, `settingsState` and `managedSettingsHash` in `internal/adapter/claude`.

### A newly owned settings key has a record of its own, read tolerantly

- **Rests on:** the same report: 0.8.3 lists the files that it reads, copies and removes by fixed names, so it
  never touches another file beside the settings. The digest of the written value has to survive 0.8.3's
  re-indentation, so it is taken over the value's canonical JSON, not its bytes.
- **Considered:** no record, deriving "AIGW wrote this map" from the configuration's previous selection, which
  cannot tell AIGW's map of an older selection from the user's after 0.8.3 changed the selection; a record with one
  fixed field, strictly decoded, which is the defect again at the next owned key.
- **Chosen because:** a record of entries by settings key, whose reader keeps an entry that it does not know and
  leaves that key alone, lets a later release add an owned key while this release stays a valid predecessor. An
  entry's own shape is fixed once released; a new need is a new entry.
- **Applies to:** a new record `settings.json.aigw-owned-keys.json` (`version`, `writer_id`, `keys`; each entry
  holds `original` and `written_sha256`), planned, written, restored and withdrawn with the first record.

### Ownership of the map begins when AIGW first needs one

- **Rests on:** the reader's report: with the map in the first record's digest, an upgrade from 0.8.3 refused
  settings in which the user had a map of their own.
- **Considered:** owning the key from the first projection, deleting a user's map while AIGW needs none; refusing a
  user's map.
- **Chosen because:** an entry exists exactly while AIGW's map is in the file. Taking over records the user's
  value for the withdrawal; with no map to write and no entry, the key is not AIGW's and stays as it is.
- **Applies to:** the projection and the withdrawal of `modelOverrides`; the cases are the table below.

| Entry   | AIGW's map | The file's value                    | Projection                                        |
| ------- | ---------- | ----------------------------------- | ------------------------------------------------- |
| none    | none       | any                                 | leaves the key                                    |
| none    | some       | any                                 | records the value as the original, writes the map |
| present | some       | what AIGW wrote, or the map to come | writes the map, records its digest                |
| present | none       | what AIGW wrote                     | restores the original, removes the entry          |
| present | any        | anything else                       | refuses as managed drift, writes nothing          |

A withdrawal follows the fourth and fifth rows, also when the first record is gone because 0.8.3 withdrew the client
after a rollback: the entry alone says what to restore.

### The release journey writes a map, and a test holds the first record to what 0.8.3 reads

- **Rests on:** the report: the journey's only Claude model is its own family, so no map was written, and the
  hosted jobs build their predecessor from the same source; only the field, never the digest, could have failed.
- **Considered:** a hosted job against the published predecessor by default, which is its own change.
- **Chosen because:** the acceptance that found this must exercise the map, and a unit test that decodes the written
  record as 0.8.3 does, with the digest's inputs that 0.8.3 uses, fails in seconds where the acceptance takes a
  ticket.
- **Applies to:** the journey's manifest and assertions under `tools/release`; a test in
  `internal/adapter/claude`.

## Risks / Trade-offs

- While 0.8.3 is active after a rollback it neither updates nor removes the map. A selection of another provider
  made with 0.8.3 leaves the map naming the earlier provider's IDs until the newer program synchronizes again, and
  a withdrawal made with 0.8.3 leaves the map and its record; the newer program's next projection, withdrawal or
  uninstall removes both. The changelog says so.
- After such a withdrawal by 0.8.3 the settings file may hold only the map; when the newer program restores it and
  nothing else remains, it writes an empty settings file, because the fact of whether the file existed before AIGW
  went with the first record.
- Claude Desktop: 0.8.3 accepts the newer profile and writes its own entries, without the model behind an entry,
  until the newer program synchronizes again (the report, question 3). Nothing refuses.

## Migration Plan

None: no published program wrote the field that this change removes. A record written by an unreleased build of
`dev` is not read.

## Open Questions

- A hosted job that runs the lifecycle against the published predecessor by default would have found this before
  the release lane. Owner: the AIGW and Proxy session; next action: a Change of its own after 0.9.0; review
  2026-11-28.
