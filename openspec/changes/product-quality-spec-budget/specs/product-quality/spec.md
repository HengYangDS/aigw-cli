## MODIFIED Requirements

### Requirement: Hang bounds tolerate a shared CI host

Every bound that a test or the coverage run uses to stop a hang SHALL leave room for a native CI host three times slower than a quiet one; each package run SHALL use the limit that the coverage policy declares, and an interruption test SHALL count only an interruption that lands after its target has started.

#### Scenario: A shared host runs the suite slowly

- **WHEN** a native job shares its CI host and its packages run three times slower than on a quiet host
- **THEN** no package SHALL reach the policy's run limit, and no hang bound SHALL stop a test that is still making progress.

#### Scenario: An interruption lands before the work it targets

- **WHEN** a test interrupts a native preflight to prove cleanup, and the interruption lands before the workload has started
- **THEN** the test SHALL repeat it with a longer window, and every attempt SHALL still prove that the processes it observed exited within its cleanup bound.

#### Scenario: A process hangs

- **WHEN** a selected child test or a package stops making progress
- **THEN** its bound SHALL stop it and fail the run with Go's diagnostics before the job's own time budget ends.

#### Scenario: The coverage policy omits or misstates the package limit

- **WHEN** the coverage policy's package limit is missing, unparsable or not positive
- **THEN** the coverage run SHALL refuse the policy before it runs any test.
