## MODIFIED Requirements

### Requirement: Native Go events retain one coverage attempt

The existing coverage command SHALL optionally emit original Go test JSON on stdout and coverage summaries on
stderr, and SHALL optionally write a JUnit XML test report and an LCOV line report. Tests, retained coverage
counters and both reports SHALL originate from one native invocation with the selected package inventory,
tags, race option, atomic instrumentation and threshold policy unchanged. The exact-HEAD proof SHALL judge
Go behavior from those reports and SHALL NOT run the Go tests a second time.

#### Scenario: Machine-readable coverage succeeds

- **WHEN** coverage selects `--json` and an explicit profile destination
- **THEN** stdout SHALL contain only the native test event bytes
- **AND** package and aggregate summaries SHALL remain separately observable
- **AND** the retained profile SHALL contain the same attempt's counters.

#### Scenario: The native test attempt fails

- **WHEN** the selected native tests fail after emitting events and a profile
- **THEN** their event bytes and retained diagnostic counters SHALL remain observable
- **AND** the command SHALL fail without qualifying partial coverage
- **AND** temporary profile cleanup SHALL remain required.

#### Scenario: Test and line reports are requested

- **WHEN** coverage selects a JUnit destination and an LCOV destination
- **THEN** the JUnit report SHALL hold one test case for each test result of that attempt, with its failure or
  skip
- **AND** a package that fails without a failing test SHALL appear as a test case with an error
- **AND** the LCOV report SHALL hold one record for each selected production source file by repository path,
  with the lines of that attempt's statement blocks and their counts, and no lines for a file in which the
  attempt's build instrumented no statement
- **AND** the LCOV report SHALL hold no branch or function records
- **AND** a profile file outside the selected inventory SHALL fail the command
- **AND** a reader SHALL never observe a partially written report.

#### Scenario: The exact-HEAD proof judges Go behavior

- **WHEN** the exact-HEAD proof evaluates the behavior of the Go sources
- **THEN** it SHALL read the reports written by the coverage gate's own attempt
- **AND** no other proof gate SHALL run the Go tests.
