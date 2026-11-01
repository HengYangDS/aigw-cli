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
`--target` in `client enable`, or adding or removing a JSON document field.

## Decisions

- **Cobra's own refusal.** A removed spelling fails with Cobra's
  `unknown flag` error, rendered as every other flag error is, before the
  command runs. Keeping a table of removed spellings to name each replacement
  in the error was rejected: it is the machinery this Change retires, and 0.5.0's
  warning and CHANGELOG already named the replacements.
- **A flag error names the command's help.** Cobra's flag errors, an unknown
  flag, a flag without its value or a value that does not parse, named
  top-level help or `aigw check`, neither of which lists the command's flags.
  The flag error function now appends the refusing command's help, as
  flag-group and required-flag failures already do, so a script that passes
  `setup --model` is sent to the help that shows `--upstream-model`.
- **Retire the mechanism with the spellings.** The replaced-by annotation, the
  warning before each command, the mutual-exclusion pairs and the apply-command
  mapping have no other use, so they go. A later rename can bring the
  mechanism back from history.
- **Journeys pass `--path` to every program.** The release journeys told the
  baseline apart from the candidate only to pass `--target` to the baseline.
  A baseline built from this source no longer accepts `--target`, and v0.5.0,
  the predecessor of 0.6.0, accepts `--path`, so the journeys pass `--path` to
  both and the split goes. A predecessor before 0.5.0 fails a journey at its
  first step, the install.
- **Version 0.6.0.** Removing flags breaks scripts that still pass them, so
  the release is a minor version, as 0.5.0 was.

## Risks / Trade-offs

- A script that skipped 0.5.0 and still passes an earlier spelling stops at
  the unknown flag → CHANGELOG's 0.6.0 section names each removed spelling and
  its replacement, and the refusal happens before any Token read or write.
- Cobra's error does not name the replacement → the error names the command's
  help, which shows `--upstream-model` and `--path`.
- Scripts that match a flag error's exact text or its `next_action` see the
  command's help where they saw `aigw --help` or `aigw check` → the CHANGELOG's
  0.6.0 section records the change.
