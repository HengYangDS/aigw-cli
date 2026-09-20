# Text Layout Policy

Repository-wide authoring and formatting policy for contributors. Product
interaction belongs to [Terminal experience](../experience/terminal-experience.md).

## Scope

The byte invariants apply to every tracked text file, including root metadata,
source, documentation, configuration, locks and generated CI projections.
Disposable build output and binary content are not text-policy inputs.

Markdown authors separate distinct semantic paragraphs with one blank line.
Headings, lists, tables, and fenced blocks also retain one blank line from
adjacent prose. Formatter wrapping within one semantic paragraph does not create
a new paragraph; Prettier owns that mechanical wrapping.

## Enforced byte invariants

- UTF-8 text uses LF line endings.
- Non-empty text ends with a newline.
- Lines contain no trailing spaces or tabs.

Git's [text attributes](https://git-scm.com/docs/gitattributes#_text) establish
one LF checkout policy for automatically detected text, including extensions
not individually listed. The locked EditorConfig Checker enforces the byte
invariants from [EditorConfig policy](../../.editorconfig); native formatters
retain ownership of layout. Editor defaults are inherited once, with Go's tab
indentation as an override. No repository-specific parser duplicates those
mature responsibilities.

## Format ownership

| Concern                 | Owner                           | What it proves                                                                                  |
| ----------------------- | ------------------------------- | ----------------------------------------------------------------------------------------------- |
| Source layout           | The locked language formatter   | Canonical whitespace and syntax layout                                                          |
| Current Markdown layout | Locked Prettier                 | Stable tables, lists, fences and wrapping policy                                                |
| Markdown structure      | Locked markdownlint             | Heading progression, sibling uniqueness, list structure, language-tagged fences and link syntax |
| Diagram syntax          | Locked Mermaid validator        | Grammar and diagnostics, not visual quality or hosted rendering                                 |
| Explicit links          | Locked lychee and Git inventory | Tracked local targets and heading anchors; external reachability requires an online run         |
| OpenSpec document shape | Locked OpenSpec validator       | Official Change and specification structure                                                     |

Do not duplicate formatter rules in a prose table or a custom parser. For
example, OpenSpec's official top-level sections need not be rewritten as H1
headings, and repeated scenario names under different requirements are valid.
The Markdown policy checks duplicate sibling headings, not repeated words
across unrelated sections.

Native heading checks reject decorative trailing punctuation and standalone
emphasis that hides a section from heading navigation. Question headings and
OpenSpec's `**Goals:**` / `**Non-Goals:**` labels remain valid. These are
structural constraints, not a custom prose parser or a ban on emphasized text.

Local links include their fragment targets. The native link command enables
anchor checking; an existing Markdown file with a missing heading is a broken
link. It also admits local targets against the checkout's Git index, including
directories containing tracked content. An ignored or untracked file on the
author's machine is not a shared target; symlinks must resolve inside that
tracked content. Stage new linked files before checking their source links.
Source checks remain offline, so they do not establish external URL
availability or rendered browser behavior.

The repository quality graph exposes one format check through the existing
CI owner. That owner streams the exact checkout-bound Git inventory to the
locked, checkout-local Prettier API declared by
[the npm package manifest](../../package.json). Git ignores filter untracked
output, not tracked source; [formatter scope](../../.prettierignore) excludes
only immutable OpenSpec history. A directory named `archive` elsewhere remains
current source.

The adapter applies native defaults without discovering local or parent
Prettier configuration. EditorConfig owns editor defaults and its independent
byte checks, not Prettier options. Missing local dependencies, a scope with no
supported files or differing format fail without writing source or falling
back to another installation.

Markdown lint validates its one rule file against schemas shipped with the
locked package before checking source. It does not discover nested or parent
configuration, accept inline suppression, or ignore warnings. Current style
checks exclude only official OpenSpec archive history. Mermaid syntax checks
still include archived diagrams; that does not make their historical design
current. Both adapters receive exact file inventories on standard input,
preserve source bytes and require their checkout-local dependencies.

OpenSpec owns document validation. The CI consumer admits its result only when
the findings report identifies the requested checkout, covers `all` items in a
nonempty scope, and records consistent successful totals with no findings.
Exit status zero alone is insufficient: an empty project or a report discovered
in a parent directory does not validate this checkout. This result admission
does not parse OpenSpec documents or prove implementation completeness.

## Shared references and navigation

A source or operation reference must be usable by the document's audience.
Link a repository authority to its tracked owner, and an external claim to
its primary source or published artifact. A local verification log can support
work in progress, but it is not a shared source citation in current product or
research documentation. Preserve necessary findings in their semantic owner;
publish required raw evidence with the corresponding CI run or release.
Do not commit disposable output merely to make a citation resolve.

When a document introduces a configuration, specification, decision, procedure,
or code owner as an authority or next step, provide a descriptive link at that
point. Repeated mention within the same explanation need not repeat the link.
Examples, runtime paths, filename conventions and command syntax remain literal
when they do not refer to a repository artifact. Their prerequisites and the
procedure that creates them must still be clear.

Review navigation from the reader's task: entry point, prerequisites, action,
verification and recovery. Every canonical document needs an incoming path from
the documentation map or a semantic register. Link syntax and reachability can
be checked automatically; whether a missing link prevents a reader from acting
still requires semantic review.

## Content and rendering

English is the canonical language for repository-authored documentation,
including root guides, design and research documents, and active OpenSpec
artifacts. Conversation language does not change this convention. Preserve
external quotations, proper names, and verbatim diagnostic evidence in their
original form when accuracy requires it; explain their relevance in English.
Translate a document in place rather than adding an unrequested bilingual copy.
This convention does not authorize rewriting immutable archives or third-party
license text.

A document has one audience, purpose and canonical owner. Organize its headings
by that purpose; use lists for alternatives or sequences, tables for comparable
attributes, and diagrams for relationships or behavior that benefit from a
visual explanation. Remove a diagram or table when it repeats nearby prose
without helping a reader decide or act.

- **Diagrams:** name the actors and edge meanings, keep direction consistent,
  and describe the diagram for nonvisual readers. Configuration files do not
  send requests; a success-only flow does not prove failure recovery.
- **Tables:** use comparable rows, clear headers and concise cells. Large prose
  cells belong in sections; verify the rendered width as well as source alignment.
- **Code blocks:** identify the language and platform, distinguish executable
  commands from output, and state prerequisites. Examples must preserve the
  documented credential, ownership and failure boundaries.
- **Lists and headings:** express semantic nesting and actual execution order.
  Keep peer items at the same level and make link targets unambiguous.
- **Content and links:** state supported behavior precisely, link to its owner,
  and remove duplicated explanations or stale claims. A valid URL alone does
  not prove that a reader can find the information they need.

Render changed diagrams and pages, inspect wide and narrow layouts, and compare
behavioral diagrams and command examples against their implementation. Syntax,
formatting and link checks do not establish semantic accuracy or visual quality.
The [documentation index](../README.md) owns navigation. Historical OpenSpec
archives remain immutable; current instructions must not depend on obsolete
archived guidance.

## Terminal help

Commands and explanations are distinct fields, not strings joined by manual
spaces or shell-comment markers. Use the existing presentation renderer for
aligned rows; it owns terminal-cell measurement, ANSI-aware widths and compact
layout. Pass related rows as one group: its longest label sets the description
column, and a multiline or over-width row switches the whole group to stacked
layout. Derive the executable name from Cobra command metadata. A narrow terminal
places the explanation below its command instead of truncating either field.
Standalone executable examples remain command output, not annotated rows.
Preserve their characters, quoted whitespace and explicit line breaks; leave
visual soft wrapping to the terminal. A logical command line may exceed the
terminal width. Do not insert hard line breaks or shell-specific continuation
syntax to fit it. Usage grammar with placeholders is explanatory text, not an
executable example, and follows the width-aware text layout.

Verify wide, narrow, colored and plain output through the real help renderer.
Assert description-column alignment, complete meaning and bounded display width,
rather than copying source padding into expected strings. Existing Lip Gloss
and ANSI utilities own display measurements; another layout framework or an
independent help template is unnecessary. pflag owns option grammar, while the
same width-aware renderer owns its final display. Long labels, unbroken values,
multiline explanatory content and indentation must fit the available terminal
cells without dropping text; color must not change measured layout. Copyable
commands follow the byte-preservation contract above. Routine machine output,
credential stdout and interactive input prompts retain their separate contracts.

## Generated configuration

AIGW-generated TOML remains deterministic and readable, but its serializer is
the output owner. The repository-wide text checker does not duplicate serializer
formatting policy.
