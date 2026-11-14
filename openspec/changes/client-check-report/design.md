## Context

A client-depth check runs the real client in a private home and treats an explicit native warning or error as an
unqualified verification. Until now every such failure had one sentence, and the client's output was dropped at
the adapter. That was deliberate: the output can hold a credential, a path or a provider's error body, and a
check's output is pasted into issues.

## Goals / Non-Goals

**Goals:**

- Whoever meets such a failure, a person or an agent, learns what to change, or can read what the client said, on
  request.
- AIGW's own message stays safe to paste and carries none of the client's sentences.
- One owner for the rule across Codex and Claude Code.

**Non-Goals:**

- Deciding that some client reports are harmless. A deprecation notice still stops the check; the owner of the
  configuration removes the key or accepts the stop.
- Showing a client's output of a successful session, or without a request.
- Showing the output of a client whose Token an external helper supplies.

## Decisions

### The message, the remedy and the report are three things

`verification.Diagnostic` returns an error with AIGW's message, a remedy where AIGW can name one, and the
client's output. The message and the remedy never carry the client's sentences, which the existing leak tests
keep checking. The check command reads the report and the remedy through methods of the error, so it imports
nothing from the adapter layer.

Alternative considered: put a redacted excerpt into the message. Rejected, because the default output would then
carry client text that nobody asked for, and a redaction miss would reach every paste.

### A deprecated key is named by grammar

A client that calls a configuration key deprecated writes it in backticks before "is deprecated". AIGW reads up
to three such keys and names them in its message and in the remedy "Remove … from the client's configuration".
The grammar is a path of lowercase names that start with a letter: a name or a table in brackets, then at least
one dot and name, each name at most 32 characters and the path at most 64. A key path is configuration
vocabulary, not a sentence, and it is what the reader must find. A single word, a capital and a name that starts
with a digit are not named, because a client can quote a value there, such as a credential that it calls
deprecated; that case keeps the general message, and the report shows the client's line.

### The report is bounded and redacted, and the reader names its secrets

At most four distinct lines of at most 300 characters. Lines that carry a warning marker come first, judged on
the whole line; without one, the first lines are taken, which is the case of a Codex error item. ANSI sequences,
control characters and format characters leave. The redaction owner removes the credential forms that it knows,
and the check adds the provider's Token that it read for the endpoint depth, which catches a Token that the
client prints bare. The home directory is written as `~` where it stands as a whole path prefix.

The redaction owner gains the forms that a client's output is likely to hold: an `Authorization` header with its
scheme word, a cookie line, a URL's user and password, a `key` query parameter and a bare `sk-` key. They are
forms, not a proof: a credential that no form describes and that is not the provider's Token stays visible in
output that the caller asked for.

### Only a request shows it, in either output

`--verbose` already means "show what the check knows". The failing client's row gains its report lines there, and
the JSON document gains them as `failure.client_report`: an agent that runs the check needs the client's words as
much as a person does, and reads the document. Without `--verbose` neither output carries them. The document's
`schema_version` becomes 4, as the rule for a changed field requires.

### The Fix is never the same check

The rule that every failure names one fix forbids a command that would fail the same way, and the same check
with `--verbose` fails the same way. So the Fix of such a failure is prose, which the document carries as
`next_detail`: the remedy when AIGW names one; else how to read the report; and, once the output shows the
report, to resolve what the client reported.

### An external credential helper withholds the report

For a client that reads its Token through an external helper, the runner already replaces the client's output:
the helper's output can stand in it. The adapters withhold the report in that case, for the client's event
stream too. Hermes's verification treats its output the same way and keeps its messages.

### Codex's memory work is off in the private home

The session's command line sets `features.memories=false`. Dropping the key from the copied configuration would
leave the client's default in force, which a later Codex release may turn on. Measured with Codex 0.169.0 on one
host: with the override a first turn sends two requests, both for the verified model; without it, five or six
more for another model, each ending early.

## Risks / Trade-offs

- **A report line can still hold something private** that no form describes, such as a host name → it appears
  only on request, in the caller's own output, bounded to four lines.
- **A key-shaped secret in backticks before "is deprecated" would be named in the default message** → the
  grammar admits only lowercase dotted names of bounded length; the residual case needs a client that writes a
  secret in exactly that place.
- **A client changes its wording**, and the deprecated key is no longer named → the message falls back to the
  general sentence, and the report still shows the client's words.
- **Another background feature of a client sends its own requests** → only the observed one is turned off; the
  next one is found the same way, by counting a session's requests.
