## ADDED Requirements

### Requirement: CI steps run one native command

Each projected CI step SHALL run one native command or toolchain-owner task.
A step MAY keep a shell program only while it runs before the locked toolchain
exists or after it may have failed, as operating-system glue or an official
installer, and only while the retained-step record names its Forge file, its
jobs and the SHA-256 of its exact text. Native acceptance SHALL derive its
arguments from the job's declared inputs in one implementation for every
platform.

#### Scenario: A native job receives manual inputs

- **WHEN** a manual pipeline supplies candidate, baseline, input-package,
  diagnostic or performance inputs to a native job
- **THEN** the CI tool SHALL map them to the same acceptance arguments on
  every platform
- **AND** diagnostic or performance inputs outside a manual pipeline SHALL fail
  before acceptance starts.

#### Scenario: A retained step changes

- **WHEN** the text of a retained shell step changes without its record
- **THEN** repository governance SHALL report the step as inline shell until
  the record names the new text.

#### Scenario: A step needs a sequence of commands

- **WHEN** a step would run more than one command
- **THEN** the sequence SHALL move into a native command or toolchain task, and
  the step SHALL invoke only that command.

#### Scenario: A native job receives only artifact inputs

- **WHEN** a GitLab native job selects existing artifacts without full quality
  or a lock refresh
- **THEN** the Forge configuration SHALL select the artifact tool closure from
  the same inputs, without a shell condition
- **AND** the native command SHALL skip the source bootstrap and source
  qualification.
