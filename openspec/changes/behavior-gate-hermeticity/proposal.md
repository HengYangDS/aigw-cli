# Behavior Gate Hermeticity

## Why

On 2026-10-14 host state decided the outcome of AIGW's offline behavior gates
twice. A Homebrew upgrade broke the pre-commit hook in the host's Git template;
`git init` had copied that hook into every release fixture, so under ETHOS's
tool-only `PATH` 60 fixture tests failed and three proofs were blocked. The
live OSV Scanner test also failed whenever the host's route to `api.osv.dev`
and `api.deps.dev` stalled, although both gates declare
`network_policy = "offline"`.

## What Changes

- Tests run under an owned global Git configuration that keeps the host's
  global settings but replaces its hook directory and template with empty
  owned directories, so host hooks cannot run in a fixture and a host template
  cannot add hooks, configuration or exclusions to one.
- The live dependency-scan qualification moves behind the
  `dependency_acceptance` build tag and runs as a quality-graph gate. The
  `go-static-analysis` proof gate, which declares network access, already runs
  that graph. Offline tests with controlled runners keep covering evidence
  normalization and nonblocking findings.

## Capabilities

`product-control-plane`'s "Deterministic local verification" requirement also
excludes host Git templates and hooks. Native qualification against public
services belongs to a quality gate that declares network access.

## Impact

Each Go test package that creates repositories owns that fixture policy
through `tools/gitfixture`, a new repository-tool package with no product
imports. The quality graph in `tools/ci/quality.go` owns which checks need
network evidence. `.ethos/profile.toml` is unchanged; its offline and network
declarations become true. The change reuses the existing build-tag acceptance
pattern and controlled dependency-scan runners, and adds no tool or registry.
Production, release, Forge and ETHOS behavior are unchanged, and nothing is
removed.

Non-goals: no change to the host network or proxy, no stand-in for the OSV and
deps.dev services, and no retry or longer timeout that would hide a network
failure.
