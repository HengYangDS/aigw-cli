# Design

## Context

See `proposal.md` for the motivation. Git selects a template from
`--template`, then `GIT_TEMPLATE_DIR`, then `init.templateDir`, then its
built-in default, and copies the template's hooks, `config` and `info/exclude`
into each new repository; an empty selection copies nothing. A global
`core.hooksPath` applies to every repository, including the ref update that
`git init` performs. The 2026-08-23 fixture policy makes fixtures own their
identity, signing and hook directory after initialization, so it cannot undo
template content or stop a global hook that runs before or outside that
policy.

On 2026-10-15 the default Go suite ran with every public HTTP(S) and gRPC
request refused through an unreachable proxy, with loopback exempt. Only
`TestNativeDependencyEvidenceRetainsFindings` failed. Clients that ignore proxy
settings would not be detected this way. The same suite, run under a hostile
host template and a global hook directory whose `reference-transaction` hook
rejects every update, failed 80 tests in five packages through that hook alone.

## Goals / Non-Goals

**Goals:** the offline behavior gates observe only controlled fixtures, and
native dependency qualification keeps exercising the production scanner
invocation.

**Non-Goals:** a stand-in for the OSV or deps.dev services, host network
changes, changes to the ETHOS profile, and host system-level Git
configuration.

## Decisions

### An owned global Git configuration for each test process

`tools/gitfixture` gives each test process an owned global configuration. It
includes the host's top-level global files, which Git itself lists with
`--show-scope --no-includes`, and then names an empty owned hook directory;
`GIT_TEMPLATE_DIR` names an empty owned template. Fixture-local configuration
and command-line options still take precedence, so Forge peers keep their own
hooks. Every test package that creates repositories runs through
`gitfixture.Run` from its `TestMain`.

One shared owner replaces seven copies of the same policy, so the architecture
policy declares `tools/gitfixture` as a repository-tool package with no
product imports. Dropping the host's global configuration altogether would
also drop runner settings such as `safe.directory`. Injecting `core.hooksPath`
through `GIT_CONFIG_COUNT` would override fixture-owned hook directories.
Passing `--template=` and a hook directory at each call site would repeat the
rule at dozens of sites.

### Live dependency qualification declares network access

OSV Scanner's offline mode turns off the deps.dev license lookup that
production scans rely on, so a controlled fixture would need test-only scanner
flags. The live test therefore moves behind the `dependency_acceptance` tag and
runs as the `dependency-acceptance` quality-graph gate, following
`client_acceptance` and `native_resource_acceptance`. `go run ./tools/ci
quality` is the `go-static-analysis` proof gate, which declares
`network_policy = "required"`, and `mise run check` runs the same graph. The
controlled-runner tests in `tools/release/construction` stay in the offline
suite.

## Risks / Trade-offs

- [A future package creates repositories without `gitfixture.Run`] → running
  the default suite under a hostile host template and hook directory is the
  acceptance check for every package.
- [A host sets hooks in its system configuration or through Git environment
  variables] → those remain outside this boundary; the owned global hook
  directory still overrides a system-level one.
- [Windows resolves global files or quoted paths differently] → Git reports
  the global files itself, and the regressions run in native Windows CI before
  the change is published.
- [The live qualification leaves each platform's coverage run] → it still runs
  wherever the quality graph runs, including full-quality native jobs.
