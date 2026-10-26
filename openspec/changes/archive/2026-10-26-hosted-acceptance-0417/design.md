# Design

## Context

GitHub jobs install Mise through `jdx/mise-action` with `version` set to the
CUE `miseVersion`, which is derived from the `miseImage` tag. GitLab Linux
jobs run that image, and GitLab Windows ARM64 jobs refuse any runner-owned Mise
whose version or executable digests differ from the CUE pins. Contributors run
their own Mise, which `.config/mise/config.toml` holds at `min_version`.

Change `codex-connection-retries` (`268b0e43`) taught the mirror failure test
Mise 2026.10.19's diagnostic for an asset it cannot confirm. The pin stayed at
2026.10.15 and the floor at 2026.10.14, so the same test expects behavior that
only some supported Mise releases have.

## Goals / Non-Goals

**Goals:**

- Hosted native jobs pass on the 0.4.17 head.
- The declared Mise is the one that local proofs and hosted jobs run.
- Windows coverage stays above its floor.

**Non-Goals:**

- Lowering the coverage floor.
- Weakening the mirror test.
- Changing product behavior.

## Decisions

### Advance the declaration instead of tolerating two Mise behaviors

The pin, the floor and the runner-owned Windows Mise move together to 2026.10.19.

Making the test accept either diagnostic was rejected, for two reasons. It
would hide which release a proof ran. It would also keep 2026.10.15, which
lacks 2026.10.19's protections against hostile mirrors and archives.

The identities come from the official release:

- Docker Hub tag `2026.10.19-debian`: `sha256:9b8995ab…a21`.
- Windows ARM64 zip: verified against `SHASUMS256.txt`.
- Its `mise.exe` and `mise-shim.exe`: `50596450…7240` and `a7b0a3c9…29d1`.

The 2026.10.15 values were recomputed the same way and matched the current
pins.

### Cover the new paths, not the floor

The tests exercise behavior that the 0.4.17 changes introduced and nothing
verified:

- the human preview of declared retry budgets;
- each refusal and selection of the release Git peer;
- rollback admission and retry decoding failures.

They pass against the current code; the hosted coverage report is what they
fix. Folding error branches to save statements was rejected, because it would
hide the paths from review.

## Risks / Trade-offs

- **Stricter Mise.** 2026.10.19 refuses some inputs that 2026.10.15 accepted,
  such as a mirror without Node `.sig` files. Local proofs already ran on
  2026.10.19; GitLab's peer mirror is first exercised at the GitLab catch-up.
- **Contributor floor.** A contributor on an older Mise must upgrade before
  running the repository's tasks.

## Migration Plan

Publish the landed head to the 0.4.17 proposal so its checks rerun. Before the
next GitLab job, replace the runner-owned Windows ARM64 Mise with the verified
2026.10.19 executable and shim. GitLab is unreachable now, so the replacement
waits for the GitLab catch-up; that catch-up is owned by this session and
starts when the user reports GitLab back.

## Open Questions

- None.
