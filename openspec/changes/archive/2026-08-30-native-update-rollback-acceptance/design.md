## Context

See `proposal.md` for motivation. The repository already owns one portable
lifecycle implementation in `internal/upgrade`, one public `aigw update`
command, one public `aigw uninstall` command, portable archive builders, and one
native product journey invoked by `tools/ci native`. The missing evidence is a
public-command journey over released-artifact shapes, not a missing lifecycle
implementation.

## Goals / Non-Goals

**Goals:**

- Prove the same reversible program lifecycle on macOS, Linux, and Windows.
- Exercise real portable archives and checksum manifests through the installed
  public executable.
- Preserve the existing configuration and credential retention contract.
- Make lifecycle residue observable and require its terminal absence.

**Non-Goals:**

- Add a lifecycle service, script runner, test framework, update protocol, or
  platform-specific implementation.
- Contact either Forge or require a network during native acceptance.
- Couple AIGW to Codex Responses Proxy or another external endpoint product.
- Re-test every setup scenario inside the lifecycle sequence.

## Decisions

### Extend the existing native journey

The new scenario belongs in `tools/ci/native_journey_test.go`, which already
builds and executes the native product and is already selected by every native
CI job. This keeps one cross-platform acceptance command and one fixture model.

Alternative rejected: create a second release-lifecycle binary or shell script.
That would duplicate command orchestration and introduce another platform
surface.

### Build two real portable archive generations locally

The fixture will build small old and new AIGW executables from the same source
with distinct injected versions, package each in the canonical release archive
shape, and generate a standard checksum manifest. The installed old program
will consume the newer archive using the existing explicit candidate input.
This proves extraction, checksum validation, atomic replacement, and rollback
without making native CI depend on a Forge or network.

Alternative rejected: replace bytes directly through `internal/upgrade` tests.
Those tests remain valuable but do not prove the public shipped command.

### Assert state after every transition

The journey observes `aigw --version` after install, update, rollback, and
forward recovery. It also records configuration and an environment-backed
credential before the lifecycle transition, then rechecks them afterwards.
After uninstall, the portable source executable invokes the same public
uninstall command against the installed target. This avoids pretending that a
running Windows image can synchronously delete itself while still proving the
public product boundary. The journey then requires absence of the installed
executable, the one rollback sibling, and lifecycle staging residue while
retained user state remains.

Alternative rejected: assert only command exit status. A zero exit code cannot
prove that the intended executable generation is active or that cleanup
converged.

### Reuse platform-neutral product paths

All orchestration remains Go and uses `filepath`, repository archive helpers,
and the existing updater. No POSIX shell, PowerShell, launch service, or
platform-specific installation branch is added.

## Risks / Trade-offs

- **Archive creation could diverge from release packaging.** → Reuse canonical
  archive naming/layout and the standard checksum format already enforced by
  release tests.
- **The journey increases native job duration.** → Build two binaries once and
  use one bounded lifecycle scenario; avoid duplicate source or full-suite
  execution.
- **Environment credentials are not persistent files.** → Use them here to prove
  lifecycle preservation without triggering host credential UI; the existing
  opt-in native Keyring scenario continues to prove each platform's system
  store separately.
