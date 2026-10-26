## ADDED Requirements

### Requirement: Diagnostics name the native file they inspect

When doctor reports an observation about one native client configuration file
under the user's home directory, human and JSON output SHALL name that file by
its path relative to the home directory. A file outside the home directory
SHALL NOT be named, and its human label SHALL say that it lies outside the home
directory. Diagnostics SHALL NOT expose an absolute or other private path.

#### Scenario: Codex projects into its default home

- **WHEN** doctor inspects a Codex binding whose target is
  `~/.codex/config.toml`
- **THEN** the human check is labeled `Codex configuration ~/.codex/config.toml`
- **AND** the JSON check keeps its name `codex:target-1` and carries
  `~/.codex/config.toml` as `target`.

#### Scenario: A Codex target lies outside the home directory

- **WHEN** a Codex binding's target lies outside the home directory
- **THEN** the human check is labeled by its position and says that it lies
  outside the home directory
- **AND** neither human nor JSON output contains the file's path.
