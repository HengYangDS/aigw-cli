## ADDED Requirements

### Requirement: Hang bounds tolerate a shared CI host

Every time bound that a test or the coverage run uses to stop a hang SHALL
leave room for a native CI host that runs the suite at least three times
slower than a quiet host. Each package run SHALL have an explicit limit that
the coverage policy declares. A test that proves cleanup after an interruption
SHALL count only an interruption that lands after the process chain it targets
has started.

#### Scenario: A shared host runs the suite slowly

- **WHEN** a native job shares its CI host with other jobs and its packages run
  three times slower than on a quiet host
- **THEN** no package SHALL reach its run limit, and no hang bound SHALL stop a
  test that is still making progress
- **AND** each package's run limit SHALL be the value that the coverage policy
  declares, not the Go default.

#### Scenario: An interruption lands before the work it targets

- **WHEN** a test interrupts a native preflight to prove that its processes are
  reclaimed, and the interruption lands before the workload has started
- **THEN** the test SHALL repeat the interruption with a longer window instead
  of failing
- **AND** every attempt SHALL still prove that the processes it observed exited
  within the attempt's cleanup bound.

#### Scenario: A process hangs

- **WHEN** a selected child test or a package stops making progress
- **THEN** its bound SHALL stop it and fail the run with Go's diagnostics
  before the job's own time budget ends.

#### Scenario: The coverage policy omits or misstates the package limit

- **WHEN** the coverage policy's package limit is missing, unparsable or not
  positive
- **THEN** the coverage run SHALL refuse the policy before it runs any test.
