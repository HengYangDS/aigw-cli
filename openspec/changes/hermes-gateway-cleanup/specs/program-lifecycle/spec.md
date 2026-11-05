## MODIFIED Requirements

### Requirement: Failure recovery retains owned-resource cleanup outcomes

Credential writes, explicit client verification and coverage execution SHALL
retain their original failure when resource cleanup also fails. Cleanup SHALL
name the exact owned resource and SHALL NOT silently report success.

#### Scenario: A credential replacement fails before publication

- **WHEN** replacement cannot rename its staging file and staging removal also fails
- **THEN** the original credential SHALL remain unchanged and both failure causes
  SHALL be observable with the exact retained staging name
- **AND** once replacement commits, cleanup SHALL NOT remove a subsequently
  recreated file at the old staging name.

#### Scenario: Client verification creates additional output

- **WHEN** a client verification command writes into its working directory
- **THEN** that directory SHALL be private to the verification operation and
  reclaimed on success or failure
- **AND** cleanup failure SHALL retain the invocation error and exact workspace.

#### Scenario: A client verification launched a gateway that is still starting

- **WHEN** a client verification ends while a gateway it launched into its
  private home has not yet locked or recorded itself
- **THEN** cleanup SHALL keep the home until that gateway's own records show it
  ended
- **AND** if its bounds pass first, cleanup SHALL retain the home and name it.

#### Scenario: Coverage measurement succeeds but route cleanup fails

- **WHEN** a coverage command cannot remove its owned temporary route
- **THEN** its exit status SHALL indicate failure and its diagnostic SHALL name
  that file without suppressing any preceding test failure.

#### Scenario: A guarded file write fails during staging

- **WHEN** writing, setting permissions or syncing an owned temporary file fails
- **THEN** the commit helper SHALL close its handle exactly once and retain any
  close failure alongside the original error
- **AND** the writer SHALL remove only its failed staging file and report any
  cleanup failure without modifying unrelated files
- **AND** after a successful rename, it SHALL relinquish the old staging name.
