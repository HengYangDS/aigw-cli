## Context

The owner's curation of 2026-11-12 named families: Claude Fable 5.2, Opus 5.5, Sonnet 5.5 and Haiku 5.5; GPT-6.1 Sol
and Terra; every Gemini 4 model. On 2026-11-13 the owner restated it by product line, with the instruction that a
newer model of a line replaces the older one.

## Goals / Non-Goals

**Goals:**

- The shipped team file holds the newest generally available model of each named line, and each other
  vendor's single strongest one.
- A newer model of a line replaces the older one, and a test fails while a line holds two families.
- UCloud's Claude Fable 5.2 enters without turning off Claude Desktop's tool search on UCloud.

**Non-Goals:**

- Changing the file's recommendations or any client default.
- A new release: the team file is a repository file that `aigw setup --from` applies.

## Decisions

### The lines and their newest models

Each vendor's own documentation, read on 2026-11-13, names the lines:

- Anthropic lists Fable 5.2, Opus 5.5, Sonnet 5.5 and Haiku 5.5 as its latest models. Claude Mythos 5.2 is limited to
  verified organizations, so it stays out under the rule for models that are not generally available.
- OpenAI's GPT-6 guide names GPT-6 Astra for the most demanding work, GPT-6.1 Sol, GPT-6.1 Terra and GPT-6 Luna for
  focused, high-volume tasks. Fast, Ultrafast and pro are modes of these models, not lines.
- Google's model page lists three Gemini 4 lines: Carbon, Argon and Hydrogen. It calls Gemini 3.8 Flash its
  previous-generation Flash and lists 3.1 Pro as a preview, so neither is a current line.

The owner guessed Gemini had two lines, Argon and Carbon. The file follows the owner's rule and Google's page, so
Hydrogen enters; removing it is a one-line change if the owner names only two.

### A test of the rule

`TestTeamManifestKeepsOneFamilyPerCuratedLine` reads each curated family's line from its ID (`claude-<line>-…`,
`gpt-<version>-<line>`, `gemini-<version>-<line>`). It fails while one line holds two families, and it pins the
eleven lines, so a line cannot leave or join without a test change. A test cannot decide which model is newest,
which stays a review of the vendor's notes.

### Qualification

Every new provider model passed the three Hermes turns of 2026-11-12 on the stock Hermes v0.23.1 that CI pins: an
isolated `read_file` turn, a tool-free resumed recall, and a resumed `read_file` of a second file.

- AIHubMix's Responses relay returned empty output three times on Hydrogen's resumed tool turn, so AIHubMix serves
  Hydrogen over Chat Completions, which passed.
- DMXAPI's Gemini 4 Carbon missed the 240 s bound again and stays out.
- The supplied stock Hermes lacks the Anthropic SDK until `hermes pm install --extra anthropic` runs; the first Fable
  attempt failed on that alone.

### UCloud's Fable 5.2 and Claude Desktop

Desktop enables host tool search for a provider only when every Claude model it lists declares `tool_search`. A
direct request in Desktop's tool-search shape found a deferred `mcp__` tool through `tool_reference` blocks on
UCloud's Fable 5.2, as it did on UCloud's Opus 5.5. So Fable 5.2 declares `tool_search`, and the Desktop test now
includes it.

## Risks / Trade-offs

- The file's recommendations do not change; GPT-6 Astra is not made a default for any client. That stays the owner's
  choice.
- Claude Desktop's own tool search with UCloud's Fable 5.2 is proven at the request level, not inside a running
  Desktop session.
