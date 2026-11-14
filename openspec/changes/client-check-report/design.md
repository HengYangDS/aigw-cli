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
- Showing the output of a session whose credential AIGW does not hold.
- A proof of redaction. The rules are forms; what makes a report safe to ask for is the session's credential,
  which is removed by its value.

## Decisions

### The message, the remedy and the report are three things

`verification.Diagnostic` returns an error with AIGW's message, a remedy where AIGW can name one, and the
client's output. The message and the remedy never carry the client's sentences, which the existing leak tests
keep checking. The check command reads the report and the remedy through an interface of its own, so it does
not import the verification package. The error keeps the output as the client wrote it, behind a pointer, and
shows it through no other way: no formatting verb prints it, also not `%#v` of an error that wraps it or of a
value that holds it.

Alternative considered: put a redacted excerpt into the message. Rejected, because the default output would then
carry client text that nobody asked for, and a redaction miss would reach every paste.

### A deprecated key is named by grammar

Codex writes a deprecated configuration key in backticks before "is deprecated", as a table in brackets and a
name: `[features].multi_agent`. AIGW reads up to three such keys and names them in its message and in the remedy
"Remove … from the client's configuration". The grammar is that form only: a table in brackets, then at least
one dot and name, in lowercase names that start with a letter, each at most 32 characters and the whole at most 64. A key is configuration vocabulary, not a sentence, and it is what the reader must find.

A path without a table is not named: a provider's error can quote a field of its own request that way, and
"remove it from the client's configuration" would then be false. A single word, a capital and a name that starts
with a digit are not named either, because a client can quote a value there. Each of those cases keeps the
general message.

### A report needs the session's credential

A client can print its credential in a form that no rule describes. So the report is given only to a reader who
names the session's credential, and that value is taken out first. The check names the Token that its endpoint
depth read from AIGW's store, which is the Token of the session. A client that authenticates itself, or whose
Token an external helper supplies, has no such Token in the check, and so no report, with or without
`--verbose`. One rule covers both cases, in the one place that builds a report.

Alternative considered: the adapters withhold the report for an external helper. Rejected after review: the
withheld failure lost its remedy and fell back to `aigw check` as its Fix, and a client that authenticates
itself was not covered.

### The report is bounded, and each line is redacted once

At most four distinct lines of at most 300 characters. When any line carries a warning marker, only the marked
lines are reported; without one, the first lines, which is the case of a Codex error item. Each line first loses
its control and format characters and is then judged, before its redaction, which can take the marker; one
redaction then removes the credential and the credential forms. One pass, because a form that cuts a part of the
credential would hide the rest from a second one; per line, so that no rule reaches the next line. The home
directory is written as `~` where it stands as a whole path prefix.

The redaction owner gains the forms that a client's output is likely to hold: an `Authorization` or a cookie
header to the end of its line, whatever its scheme, and the scheme word with its credential in an assignment
such as `Authorization=Basic …`; a URL's user, with or without a password; a `key`, `sig` or
`signature` query parameter; and a bare `sk-` key of at least 20 further characters, which a model or a
directory name is not. The header rule can take the rest of a one-line document with it; that is the price of
not knowing where a scheme's parameters end. They are forms, not a proof: another credential than the session's
that no form describes stays visible in output that the caller asked for.

### Only a request shows it, in either output

`--verbose` already means "show what the check knows". The failing client's row gains its report lines there, and
the JSON document gains them as `failure.client_report`: an agent that runs the check needs the client's words as
much as a person does, and reads the document. Without `--verbose` neither output carries them. The document's
`schema_version` becomes 4, as the rule for a changed field requires.

In the text the client's lines stand after AIGW's problem and Fix, as quotes: where a quote wraps, it continues
two columns deeper than a detail begins. A client's line that holds the words "Fix:" can therefore not read as
AIGW's own Fix.

### The Fix is one sentence

No `aigw` command repairs a key in the client's own configuration, and the same check with `--verbose` fails the
same way, which the rule that every failure names one fix forbids. `aigw check`, the earlier Fix, does not fail
the same way: at its default depth it starts no client, passes, and names the failing check as the deeper one.
So the Fix of such a failure is a sentence, which the document carries as `next_detail`: the remedy when AIGW
names one; else how to read the report; once the output shows the report, to resolve what the client reported;
and without a report, to run the client itself with the same model. The rule gains a third kind of Fix: a
sentence on what to change or how to learn it.

### The summary of a failed request reads the client's own text

`Failure` answers a request that the client itself failed with one of AIGW's fixed sentences, which it picks by
markers in the client's output. It read them in redacted text; with a header rule that takes the rest of a line,
a marker after a header would be lost. It now reads the client's own text, and needs no secret for that, so the
Claude adapter no longer passes the Token along.

Hermes's verification treats its output as an external helper's and keeps its messages.

### Codex's memory work is off in the private home

The session's command line sets `features.memories=false`. Dropping the key from the copied configuration would
leave the client's default in force, which a later Codex release may turn on. Measured by hand on 2026-11-14
with Codex 0.169.0 on one host, not by a test: with the override a first turn sends two requests, both for the
verified model; without it, five or six more for another model, each ending early. The test holds only that the
plan of each turn carries the override.

## Risks / Trade-offs

- **A report line can still hold something private** that no form describes, such as a host name → it appears
  only on request, in the caller's own output, bounded to four lines.
- **A key-shaped secret in backticks before "is deprecated" would be named in the default message** → the
  grammar admits only a table in brackets with lowercase names of bounded length; the residual case needs a
  client that writes a secret in exactly that form and place.
- **A Token that is an ordinary word**, such as the placeholder of a local provider → that word leaves the
  report's lines too. A real Token is not a word of a client's sentences.
- **Codex deprecates a key that AIGW itself sets for the session** → the remedy would name a key that the
  operator's file does not hold. AIGW sets one such key today, and the release that meets the notice changes
  what AIGW sets.
- **A client changes its wording**, and the deprecated key is no longer named → the message falls back to the
  general sentence, and the report still shows the client's words.
- **Another background feature of a client sends its own requests** → only the observed one is turned off; the
  next one is found the same way, by counting a session's requests.
