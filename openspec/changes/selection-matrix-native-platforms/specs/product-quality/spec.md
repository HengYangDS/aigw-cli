## ADDED Requirements

### Requirement: The selection tests run on every native platform of each Forge

The exhaustive selection tests SHALL run in every full local proof, and in each Forge's pipeline that verifies a pushed
commit, a merge or pull request, or a tag, once on every native platform that the Forge verifies: in the quality job on
that job's platform, and in the native job of every other platform. A failure of these tests on any platform SHALL fail
that pipeline.

#### Scenario: A proof runs without a remote

- **WHEN** a full proof runs on a host with no Forge configured
- **THEN** it SHALL run the exhaustive selection tests

#### Scenario: A Forge verifies a commit

- **WHEN** a Forge's pipeline verifies a pushed commit, a merge or pull request, or a tag on its macOS, Linux and
  Windows hosts
- **THEN** each of those hosts SHALL run the exhaustive selection tests exactly once in that pipeline
- **AND** the same platforms SHALL run them on the other Forge, generated from the same pipeline declaration

#### Scenario: A native job qualifies every quality tool

- **WHEN** a manual native job qualifies every repository quality tool on its host
- **THEN** that job SHALL run the exhaustive selection tests exactly once

#### Scenario: A native job accepts only prebuilt artifacts

- **WHEN** a native job accepts prebuilt release artifacts without source tests
- **THEN** it SHALL NOT run the exhaustive selection tests
