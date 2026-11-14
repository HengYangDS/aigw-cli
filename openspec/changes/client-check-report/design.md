## Context

A client-depth check runs the real client in a private home and treats an explicit native warning or error as an
unqualified verification. Until now every such failure had one sentence, and the client's output was dropped at
the adapter. That was deliberate: the output can hold a credential, a path or a provider's error body, and a
check's output is pasted into issues.

## Goals / Non-Goals

**Goals:**

- Whoever meets such a failure, a person or an agent, can read what the client said, on request.
- AIGW's own message stays safe to paste and names the fix where it can be read without the client's sentences.
- One owner for the rule across Codex, Claude and Hermes.

**Non-Goals:**

- Deciding that some client reports are harmless. A deprecation notice still stops the check; the owner of the
  configuration removes the key or accepts the stop.
- Showing a client's output of a successful session, or without a request.

## Decisions

### The message and the report are two things

`verification.Diagnostic` returns an error whose message is AIGW's classification and whose report is the
client's own lines. The message never carries the client's sentences, which the existing leak tests keep
checking. The report is read through a method of the error, `ClientReport`, so the check command needs no import
of the adapter layer.

Alternative considered: put a redacted excerpt into the message. Rejected, because the default output would then
carry client text that nobody asked for, and a redaction miss would reach every paste.

### A deprecated key is named by grammar

A client that calls a configuration key deprecated writes it in backticks before "is deprecated". AIGW reads up
to three such keys and names them in its message. The grammar is a path: a name or a table in brackets, then at
least one dot and name, each name at most 32 characters and the path at most 64. A key path is configuration
vocabulary, not a sentence, and it is what the reader must find. A single word in backticks is not named, because
a client can quote a value there, such as a credential that it calls deprecated; that case keeps the general
message, and the report shows the client's line.

### The report is bounded and redacted

At most four distinct lines, each at most 300 characters. Lines that carry a warning marker come first; without
one, the first lines are taken, which is the case of a Codex error item. ANSI sequences and control characters
leave, credential assignments and bearer values are redacted by the existing redaction owner, and the home
directory is written as `~`.

### Only a request shows it, in either output

`--verbose` already means "show what the check knows". The failing client's row gains its report lines there, and
the JSON document gains them as `failure.client_report`: an agent that runs the check needs the client's words as
much as a person does, and reads the document. Without `--verbose` neither output carries them. The fix of such
a failure is the same check with `--verbose`, because reading the report is the next action.

### Codex's memory work is off in the private home

The session's command line sets `features.memories=false`. Dropping the key from the copied configuration would
leave the client's default in force, which a later Codex release may turn on. Measured with Codex 0.169.0 on one
host: with the override a first turn sends two requests, both for the verified model; without it, five or six
more for another model, each ending early.

## Risks / Trade-offs

- **A report line can still hold something private** that no pattern knows, such as a host name → it appears only
  on request, in the caller's own output, bounded to four lines.
- **A client changes its wording**, and the deprecated key is no longer named → the message falls back to the
  general sentence, and the report still shows the client's words.
- **Another background feature of a client sends its own requests** → only the observed one is turned off; the
  next one is found the same way, by counting a session's requests.
