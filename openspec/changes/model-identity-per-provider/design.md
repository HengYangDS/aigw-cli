## Context

A model's visible name comes from `domain.modelLabel` (`Provider · Family`, or an explicit label) everywhere AIGW
shows it: its own output, Claude Desktop's model list, the Codex provider's name. Codex's picker is the exception: it
shows the catalog entry's `display_name`, which AIGW's alias copies from the family entry
(`modelcatalog.Document.Project`). The team file's curated-line test reads lines across the whole file
(`TestTeamManifestKeepsOneFamilyPerCuratedLine`).

## Goals / Non-Goals

**Goals:**

- No two models that a client lists under one provider share a visible name, in AIGW or in Codex's picker.
- The line rule decides each provider's models by what that provider lists and qualifies.

**Non-Goals:**

- Guessing a family from a model ID: a variant still joins its family only through `--family` or a team file.
- A provider's own model ID: AIGW keeps it exactly.

## Decisions

### A variant's derived label names its suffix

- **Rests on:** DMXAPI's channels end with the family ID plus `-cdx`, `-cc` or `-ssvip`; Suhe.AI's begin with
  `claude-code-` or `codex-` before it. The team file already labels every DMXAPI channel explicitly
  (`DMXAPI · GPT-6.2 Sol · CDX`).
- **Considered:** require an explicit label for every variant; derive from the ID's difference to its family.
- **Chosen because:** the derivation gives a variant that `aigw use` adds a distinct name at once. The rest of the ID
  stays as written (`cdx`): a rule that guesses which suffix is an acronym would be one nobody can predict; the team
  file's explicit labels keep their own spelling (`CDX`). A model whose ID neither begins nor ends with its family's
  keeps the plain derivation; the uniqueness check catches any collision left.
- **Applies to:** `domain.modelLabel`; `ExportTeamFile` omits a stored label equal to the new derivation.

### Labels are unique per provider

- **Rests on:** Claude Desktop lists one provider's models; Codex and Hermes show one provider at a time; AIGW's own
  output prefixes the provider.
- **Considered:** unique across the whole configuration; unique per provider.
- **Chosen because:** across providers the provider part already differs. Case is ignored, as for provider IDs.
- **Applies to:** `Config.Validate`.

### Codex's alias names the model, not the family

- **Rests on:** Codex shows `display_name`; the bundled family entry already holds the family's name.
- **Considered:** keep copying; append the suffix to the bundled name; use AIGW's label without its provider.
- **Chosen because:** AIGW's label is the one name the operator sees elsewhere for that model. When the table already
  shows that name, ignoring case and of any visibility, the model's ID follows it in parentheses; a table that shows
  that too gets no alias, as a model whose family the table lacks gets none. The verifier takes the name AIGW writes and
  asks the real client whether another entry shows it.
- **Applies to:** `Document.Project` takes the display name; `catalogcheck` compares all fields but `slug` and
  `display_name`.

### The line rule is per provider

- **Rests on:** the owner's words of 2026-11-18: 「模型规则肯定是 by provider 的，因为不同供应商上新速度不一样」.
- **Considered:** one family per line across the file; one per line on each provider.
- **Chosen because:** a provider that has not listed a successor keeps a model that works there.
- **Applies to:** `TestTeamManifestKeepsOneFamilyPerCuratedLine` and the specification.

## Risks / Trade-offs

- **An operator's configuration with two models of one label** fails to load after the upgrade. Every command loads
  the configuration first, so the refusal names both models and the label, and its fix is a `label` in the
  configuration file. A team file that would clash with a model only the configuration holds names
  `aigw model edit <model> --label`, which works on the configuration as it is. A copy of the owner's configuration
  loads under this release.
- **A family that 0.8.1's `--family` created** keeps the model's ID as its label, and no command relabels a family;
  applying a team file that names the family, or a `label` in the configuration file, corrects it. The owner's
  `gpt-6.2-sol` is corrected by the team file.
- **A team file with one family per line per provider** can show two families of one line in a client that lists
  several providers; each carries its provider's name.
