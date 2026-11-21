## ADDED Requirements

### Requirement: A model declares its context window

A model MAY declare `context_window`, the number of tokens that AIGW tells a client to use as the model's context
window. A model that declares none SHALL have 1,000,000. Only `aigw model add`, `aigw model edit` and a team file
SHALL set it, to a whole number above zero. `aigw.toml` and `aigw config export` SHALL hold the field only for a
model that declares it, and `aigw model list` and `aigw model show` SHALL report the window of every model.

#### Scenario: A model declares no window

- **WHEN** a model declares no `context_window`
- **THEN** `aigw model list --json` and `aigw model show --json` SHALL report `context_window` 1,000,000 for it
- **AND** `aigw.toml` and `aigw config export` SHALL hold no `context_window` for it.

#### Scenario: A team file declares a lower window

- **WHEN** a team file declares `context_window = 256000` for a model and `aigw setup --from` applies it
- **THEN** the configuration and every report SHALL give that model 256,000
- **AND** human output of `aigw model list` SHALL show a window that differs from the default.

#### Scenario: An operator sets a window and returns to the default

- **WHEN** `aigw model edit <provider/model> --context-window 512000` runs
- **THEN** the model SHALL declare 512,000
- **AND** `aigw model edit <provider/model> --default-context-window` SHALL remove the declaration
- **AND** `aigw model add <provider/model> --context-window 512000` SHALL add the model with that window.

#### Scenario: A window is not a whole number above zero

- **WHEN** `--context-window` is given zero, a negative number or text that is no whole number, or a team file
  declares such a value, or `--context-window` and `--default-context-window` are given together
- **THEN** the command SHALL be refused before it changes the configuration or a client's file.
