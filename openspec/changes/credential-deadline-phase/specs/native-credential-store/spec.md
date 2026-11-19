## ADDED Requirements

### Requirement: A native credential deadline names where the operation stood

A native credential operation that reaches its deadline SHALL report, without Token or item data, the deadline and
which steps ended within it: its subprocess was created, and its subprocess was started inside the process group or
Job that owns it. It SHALL give the time from the operation's start at which each of those steps ended and, when
known, the time at which the step that missed the deadline ended. The failure SHALL remain a deadline to every caller
that tests for one.

#### Scenario: The subprocess is not created in time

- **WHEN** the deadline ends before the operating system has created the credential subprocess
- **THEN** the failure SHALL say that the subprocess was not created within the deadline
- **AND** when its creation ended later, the failure SHALL give that time.

#### Scenario: The subprocess is created and not started in time

- **WHEN** the subprocess was created within the deadline and the deadline ends before it was started
- **THEN** the failure SHALL give the time at which it was created
- **AND** it SHALL say that the subprocess was not started within the deadline
- **AND** when its start ended later, the failure SHALL give that time.

#### Scenario: The subprocess is started and does not answer in time

- **WHEN** the subprocess was started within the deadline and the deadline ends before its answer
- **THEN** the failure SHALL give the times at which it was created and started
- **AND** it SHALL say that the subprocess did not answer within the deadline.

#### Scenario: The caller ends the operation

- **WHEN** the operation's caller cancels it before its deadline
- **THEN** the failure SHALL be the cancellation and SHALL NOT be reported as a deadline.

#### Scenario: The credential command shows where its read stood

- **WHEN** a client's credential command reaches the deadline of its Token read
- **THEN** its problem SHALL show where the operation stood as its evidence
- **AND** its Fix SHALL name the credential service only when the subprocess was started within the deadline.
