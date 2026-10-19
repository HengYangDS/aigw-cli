# Design

## Context

AIGW projects each client's credential command as a content-addressed
versioned reader. A client keeps invoking the reader it loaded until it
reloads its configuration, so readers from several releases can be live at
once, and AIGW never deletes or rewrites them. Each reader reads the canonical
configuration and the client-forwarding component, decodes them strictly and
validates the whole configuration, including every Route capability against
its build's vocabulary. Display order already lives in a separate catalogue
component for the same reason.

Schema six published eight capabilities from 0.3.3 through 0.4.3. AIGW 0.4.4
added `tool_search` to the vocabulary and to the canonical file. A
secret-free reproduction on October 19 showed every retained reader from
0.3.3 to 0.4.3 refusing the operator's configuration, while all six retained
readers accepted the same file without its three `tool_search` declarations
(`build/evidence/credential-reader-compat-20261019/` in the accepted root).

## Goals / Non-Goals

**Goals:** every published schema-six reader keeps returning its Token after a
newer AIGW writes the configuration; the newer program keeps the capabilities
it qualified; a reader that still fails explains itself.

**Non-Goals:** changing shipped reader bytes, the schema version or the
Account, endpoint and fingerprint checks; making predecessor programs aware of
newer capabilities; rewriting client history or cached client commands.

## Decisions

### The canonical file keeps the published schema-six vocabulary

AIGW writes only text, reasoning, streaming, tools, structured output,
continuation, compaction and multimodal input into Route interfaces of the
canonical file. A schema version bump would make every retained reader refuse
the file, the opposite of the goal. Relaxing validation helps only future
readers. Moving every capability would also hide the published ones from
predecessor programs that understand them, although only the unpublished one
breaks readers.

### Newer capabilities live in an identity-bound component

`config.capabilities.toml` holds, per Route and interface, only capabilities
outside that vocabulary, beside the forwarding and catalogue components. Each
entry records a digest of the Route's Account, endpoint, upstream model and
interface. An entry whose Route no longer matches does not apply and is
dropped at the next write. It is not an error, because a stale entry can only
add a capability, and failing every command over it would leave no repair
path inside AIGW. The component is decoded strictly, so an unknown capability
there fails the full program's configuration read explicitly.

### Credential readers never read the component

Credential lookup keeps reading only the canonical and forwarding files. A
damaged capability component therefore cannot stop Token delivery, as a
damaged catalogue already cannot.

### The earlier location migrates on the next write

Decoding still accepts `tool_search` in the canonical file, where 0.4.4 and
0.4.5 wrote it, and merges it with the component. Encoding separates the two,
so the next commit moves it, and `aigw sync` always commits. Declared
capabilities, and therefore the projected Desktop profile, stay the same.

### Recovery state carries the component

Snapshots, backups, compensation and equality include the component in the
same way as the catalogue. A verified checkpoint stores only the published
vocabulary, as it already omits display order, and merges the current
component when read; the identity binding keeps that merge to Routes that
still match.

### The credential diagnostic names reader and stage

The configuration problem's evidence names the reader's AIGW version and the
failed stage. A read failure adds the operating-system operation and error.
Path and content stay out, because the helper's stderr reaches client logs.
After a parse or validation failure the fix points to `aigw sync` with the
installed AIGW and a client reload, which replaces a stale cached reader; a
read failure points to read access and `aigw doctor`. Old readers keep their
old message.

## Risks / Trade-offs

- [A predecessor program after rollback cannot see `tool_search`] → its own
  synchronization turns off Desktop tool search; re-upgrade restores it while
  the Route still matches. An earlier reader's Token matters more.
- [A predecessor cannot express removing `tool_search`] → a removal made there
  is undone after re-upgrade; import the manifest again with the successor.
- [A later release adds another capability] → it lands in the component
  without breaking schema-six readers, but this release refuses the component
  until then; rolling back to it needs its own compatibility decision.

## Migration Plan

After archive the Change rolls out through the existing release workflow:

1. Release 0.4.6 on both peers.
2. Qualify upgrade, rollback and re-upgrade between the published 0.4.5 and
   0.4.6 portable programs in an isolated home with the environment Token
   backend: the configuration keeps `tool_search` for 0.4.6, each published
   schema-six reader returns the fixture Token after every transition, and
   0.4.5 exports the configuration after rollback.
3. Upgrade the operator installation through Homebrew, run `aigw sync`, and
   confirm that the canonical file no longer declares `tool_search` and the
   Desktop profile keeps tool search.
4. Run every retained reader against a copy of the real configuration with the
   environment backend, then each enabled client's exact projected helper
   through the native store with its output captured, never printed.

A Codex session that still calls the 0.4.3 reader starts receiving its Token
after step 3. Observing that inside the original thread is unproved, because
GUI access to Codex stays denied.

## Open Questions

- None.
