## ADDED Requirements

### Requirement: An outside edit to managed Codex settings has one recovery action

When AIGW refuses because AIGW-managed Codex settings were changed outside AIGW,
every surface that reports the refusal, including errors, `check`, `status` and
`doctor`, SHALL give the same next action: undo the outside edit, then run
`aigw sync`. AIGW SHALL NOT advise a command that refuses on the same state,
overwrite the edit, or relax the guard.

#### Scenario: Retry budgets were edited outside AIGW

- **WHEN** the operator changed managed Codex retry budgets outside AIGW
- **THEN** `use --dry-run`, `sync --dry-run`, `sync`, `check`, `status` and
  `doctor` SHALL report the same next action
- **AND** none of them SHALL advise `aigw check` or `aigw sync` alone
- **AND** client files and owned state SHALL stay unchanged.

#### Scenario: The outside edit is undone

- **WHEN** the operator restores AIGW's values and runs `aigw sync`
- **THEN** synchronization SHALL succeed and `check` SHALL report no drift.
