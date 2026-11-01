## Context

The command-surface-contracts design gave each renamed flag one release in
which its earlier spelling worked, hidden and with a warning on standard error,
and removed the earlier spelling in the release after. AIGW 0.5.0 was that
release. Its hidden spellings are bound to the same value as their
replacements, refused together with them, annotated with their replacement for
the warning, and rewritten to their replacement in a preview's apply command.

## Goals / Non-Goals

**Goals:** `setup --model`, `add --model`, `install --target` and
`uninstall --target` fail as unknown flags, and no code remains that exists only
for hidden spellings.

**Non-Goals:** renaming anything else, changing `--model` in `route add` or
`--target` in `client enable`, or changing a JSON document.

## Decisions

- **Cobra's own refusal.** A removed spelling fails with Cobra's
  `unknown flag` error, rendered as every other flag error is, before the
  command runs. Keeping a table of removed spellings to name each replacement
  in the error was rejected: it is the machinery this Change retires, and 0.5.0's
  warning and CHANGELOG already named the replacements.
- **Retire the mechanism with the spellings.** The replaced-by annotation, the
  warning before each command, the mutual-exclusion pairs and the apply-command
  mapping have no other use, so they go. A later rename can bring the
  mechanism back from history.
- **Journeys keep their baselines' flags.** The release journeys pass
  `--target` only to a baseline program, which accepts it up to 0.5.0, and
  `--path` to the candidate; the next journeys start from v0.5.0.
- **Version 0.6.0.** Removing flags breaks scripts that still pass them, so
  the release is a minor version, as 0.5.0 was.

## Risks / Trade-offs

- A script that skipped 0.5.0 and still passes an earlier spelling stops at
  the unknown flag → CHANGELOG's 0.6.0 section names each removed spelling and
  its replacement, and the refusal happens before any Token read or write.
- Cobra's error does not name the replacement → the error points to the
  command's help, which shows `--upstream-model` and `--path`.
