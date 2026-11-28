# client-programs Specification

## Purpose

Define which program each client uses: how the configuration declares it, by its path or by an editor's extensions
directory; how each operation resolves that declaration; when AIGW adopts a program that discovery finds; the client
commands that declare it; and how a program that is missing stops only its client.

## Requirements

### Requirement: A client declares its program by path or by extensions directory

A client section MAY declare `program`, the path of the client's program, or `extensions`, the path of an editor's
extensions directory, not both. Loading SHALL refuse a path that is not absolute and clean or holds a control
character, a `program` inside a version of the client's editor extension, and `extensions` for a client without one.
Loading SHALL NOT check whether the path exists; each operation decides that, so that a missing program stops only its
client.

#### Scenario: A client declares both

- **WHEN** `[clients.codex]` declares both `program` and `extensions`
- **THEN** loading SHALL fail, naming the client, without changing any file.

#### Scenario: A declared path is not absolute and clean

- **WHEN** a client declares `program = "bin/codex"`, or an `extensions` path that holds `..` or a control character
- **THEN** loading SHALL fail, naming the field, without changing any file.

#### Scenario: A program inside a version of an editor extension

- **WHEN** Codex declares `program = "<dir>/openai.chatgpt-<version>-darwin-arm64/bin/macos-aarch64/codex"`, as a
  renamed schema 8 `executable` may
- **THEN** every command SHALL stop when the file loads, naming the field, with the edit that replaces it by
  `extensions = "<dir>"` as its fix, without changing any file.

#### Scenario: A client without an editor extension declares one

- **WHEN** Hermes or Claude Desktop declares `extensions`
- **THEN** loading SHALL fail, naming the client and the field, without changing any file.

#### Scenario: A refused declaration is fixed in the file

- **WHEN** any of these refusals stops a command, `aigw client edit` included
- **THEN** it SHALL state why the declaration is refused and give the edit of the file as its fix, never a command,
  since every command loads the file first.

#### Scenario: A declared program is missing when the file loads

- **GIVEN** Codex declares a `program` that does not exist and Claude Code's program runs
- **WHEN** `aigw sync` runs
- **THEN** the configuration SHALL load and Claude Code SHALL be synchronized
- **AND** only Codex SHALL stop, as "A missing program stops its client" requires.

#### Scenario: A client declares neither

- **WHEN** a client section declares neither `program` nor `extensions`
- **THEN** loading SHALL accept it, and the client SHALL adopt the program that discovery finds, as "A configured
  client program is never replaced" requires.

### Requirement: A configured client program is never replaced

A client SHALL use the program that it declares, as `program` or through `extensions`, and only when it declares
neither the program that discovery finds on the PATH, or else at the client's fixed application path, which the
converging command then writes once as its `program` and names in its output. Only `aigw client enable`,
`aigw client edit`, a configuration rollback and `aigw self uninstall` SHALL change or remove a declaration, even when
the declared program is gone.

#### Scenario: The configured program is gone while discovery finds another

- **GIVEN** Codex is enabled with a declared `program` that is then removed, and discovery finds another `codex`
- **WHEN** `aigw sync` runs
- **THEN** the configuration SHALL keep the declared path
- **AND** AIGW SHALL NOT run the discovered program or write a file with it.

#### Scenario: No program is configured

- **WHEN** an enabled client declares neither `program` nor `extensions` and discovery finds its program on the PATH
- **THEN** `aigw sync` SHALL write the discovered path as the client's `program`, say so in its output and synchronize
  the client with it
- **AND** a later command SHALL treat that path as the client's declaration.

#### Scenario: Setup takes the configured program first

- **WHEN** `aigw setup` decides which clients are installed while the program that a client declares runs and
  discovery finds none
- **THEN** it SHALL count that client as installed.

#### Scenario: The client list shows the configured program

- **WHEN** `aigw client list` runs while a client's declared program is gone and discovery finds another
- **THEN** the client's row SHALL name the declared path and say that it is missing
- **AND** its JSON item SHALL carry that path as `program` with `installed` false.

#### Scenario: A client found only at its application path

- **WHEN** Claude Desktop declares neither, no program of it is on the PATH and its application bundle is at its fixed
  path
- **THEN** the command that converges Claude Desktop SHALL adopt the program in that bundle as its `program`.

#### Scenario: Discovery searches no extensions directory

- **WHEN** `aigw client enable codex` or `aigw sync` runs without a declaration and Codex's only program lies inside a
  version of an editor's extension, whether or not that directory is on the PATH
- **THEN** discovery SHALL NOT adopt it
- **AND** the missing-program repair that the command gives SHALL name `--extensions <dir>` beside `--program`.

### Requirement: An extensions directory resolves to the newest installed program

For a client that declares `extensions`, each operation SHALL take the directory's entries named for the client's
extension and a version, with this host's platform where an entry names one, drop each entry that the directory's
`.obsolete` file lists, and use the program inside the highest version by numeric dotted comparison when it is a
runnable regular file. AIGW SHALL NOT write the resolved path into `aigw.toml`.

#### Scenario: The highest installed version is used

- **GIVEN** Codex declares `extensions = "<dir>"` on an Apple silicon Mac, and `<dir>` holds
  `openai.chatgpt-26.5.1-darwin-arm64` and `openai.chatgpt-26.10.0-darwin-arm64`, each with its program
- **WHEN** `aigw sync` runs
- **THEN** Codex SHALL use `<dir>/openai.chatgpt-26.10.0-darwin-arm64/bin/macos-aarch64/codex`
- **AND** `aigw.toml` SHALL keep its `extensions` and gain no `program`.

#### Scenario: Codex's program on Linux

- **WHEN** Codex declares `extensions` on a Linux host with an x86-64 processor, and the highest entry of its
  extension is `openai.chatgpt-26.10.0-linux-x64`
- **THEN** Codex SHALL use `bin/linux-x86_64/codex` inside that entry
- **AND** on a Linux host with an ARM64 processor it SHALL use `bin/linux-aarch64/codex` inside the highest
  `linux-arm64` entry.

#### Scenario: An entry names no platform

- **WHEN** Claude Code declares `extensions` and the highest entry of its extension is `anthropic.claude-code-2.3.4`
- **THEN** Claude Code SHALL use `resources/native-binary/claude` inside that entry on any supported host.

#### Scenario: The editor installs a newer version

- **GIVEN** a sync used the program of the highest version of Codex's extension
- **WHEN** the editor installs a higher version and `aigw sync` runs
- **THEN** Codex SHALL use the new version's program while `aigw.toml` keeps its bytes
- **AND** `aigw client list` SHALL name the new program beside the declared directory.

#### Scenario: The editor marks a version obsolete

- **WHEN** the directory's `.obsolete` file lists the highest entry, whose directory is still on disk
- **THEN** the client SHALL use the program of the highest entry that `.obsolete` does not list.

#### Scenario: The directory has no obsolete file

- **WHEN** the directory holds no `.obsolete` file
- **THEN** no entry SHALL count as obsolete.

#### Scenario: The obsolete file cannot be read

- **WHEN** the directory's `.obsolete` file cannot be read or parsed
- **THEN** the client SHALL stop, as "A missing program stops its client" requires, naming that file
- **AND** AIGW SHALL write nothing for that client.

#### Scenario: No extension is installed

- **WHEN** the directory holds no entry of the client's extension, or only entries for another platform, such as
  `openai.chatgpt-26.10.0-linux-x64` on a Mac
- **THEN** the client SHALL stop, saying that the directory holds no installed extension of the client.

#### Scenario: The highest version lacks its program

- **WHEN** the highest remaining entry holds no runnable regular file where the client's program belongs, while a
  lower version holds one
- **THEN** the client SHALL stop, naming the missing program of the highest version
- **AND** AIGW SHALL NOT use the lower version's program.

### Requirement: Client commands declare a program or an extensions directory

`aigw client enable` and `aigw client edit` SHALL take `--program <path>` or `--extensions <dir>`, never both, and
enable SHALL require neither. A declaration SHALL replace the client's other one. A `--program` inside a versioned
directory of the client's extension SHALL be refused with the `--extensions` command that declares the same program,
and `--extensions` SHALL be refused for Claude Desktop and Hermes, which have no editor extension. A refusal SHALL
change nothing.

#### Scenario: A client is enabled without a program flag

- **WHEN** `aigw client enable codex` runs without `--program` or `--extensions` and discovery finds `codex` on the
  PATH
- **THEN** it SHALL enable Codex, write the found path as Codex's `program`, say so in its output and synchronize
  Codex with it
- **AND** when discovery finds none, it SHALL write none of Codex's files and exit non-zero with the missing-program
  repair.

#### Scenario: Both flags are given

- **WHEN** `aigw client edit codex --program <path> --extensions <dir>` runs
- **THEN** it SHALL refuse before any change, naming both flags.

#### Scenario: A program path names a version of the extension

- **WHEN** `aigw client edit codex --program <dir>/openai.chatgpt-26.10.0-darwin-arm64/bin/macos-aarch64/codex` runs
- **THEN** it SHALL refuse before any change
- **AND** its fix SHALL be `aigw client edit codex --extensions <dir>`.

#### Scenario: A client without an editor extension

- **WHEN** `aigw client enable hermes --extensions <dir>` runs, or the same for Claude Desktop
- **THEN** it SHALL refuse before any change, naming `--program`.

#### Scenario: An edit replaces the other declaration

- **WHEN** `aigw client edit codex --program <path>` runs while Codex declares `extensions`
- **THEN** Codex SHALL declare that `program` and no `extensions`
- **AND** `--extensions <dir>` SHALL likewise replace a declared `program`.

#### Scenario: An extensions directory resolves to no program

- **WHEN** `aigw client edit codex --extensions <dir>` names a directory that resolves to no runnable program of
  Codex's extension
- **THEN** it SHALL refuse before any change, with the same cause that a sync would give, without running a program.

#### Scenario: The retired flag is used

- **WHEN** a script runs `aigw client edit codex --executable <path>`
- **THEN** the command SHALL refuse it as an unknown flag and change nothing.

### Requirement: A missing program stops its client

When an enabled client's declared program is missing or cannot be resolved, or runs and cannot produce what AIGW
writes for it, such as Codex's model table, every command that would write that client's files SHALL write none of
them, its model table or AIGW's record of them, apart from withdrawing a file that the change drops, SHALL write the
rest of its change, and SHALL exit non-zero with the cause and fix that the client's local check gives, never
`aigw sync`.

#### Scenario: Codex's program is removed after a sync

- **GIVEN** `aigw sync` wrote Codex's configuration file, its model table and AIGW's record with a program that is
  then removed
- **WHEN** `aigw sync` runs, and runs again
- **THEN** each run SHALL leave the three files byte-identical and exit non-zero
- **AND** each SHALL name the missing program and the repair `aigw client edit codex --program <path>`, which also
  names `--extensions <dir>`.

#### Scenario: The other clients continue

- **GIVEN** Claude Code's program is present and Codex's configured program is missing
- **WHEN** `aigw sync` runs
- **THEN** it SHALL synchronize Claude Code's settings
- **AND** it SHALL exit non-zero for Codex.

#### Scenario: A check finds a missing program

- **WHEN** `aigw check --depth local` finds that the declared program of Codex, Claude Code, Claude Desktop or Hermes
  is missing, or that a declared extensions directory resolves to none
- **THEN** that client's local depth SHALL fail with the missing program as its problem
- **AND** its repair SHALL be `aigw client edit <client> --program <path>`, which for Codex and Claude Code also names
  `--extensions <dir>`.

#### Scenario: A runnable program ends the stop

- **WHEN** `aigw client edit codex --program <path>` names a runnable program, or `--extensions <dir>` names a
  directory whose highest version of Codex's extension holds its program
- **THEN** AIGW SHALL synchronize Codex with that program.

#### Scenario: Codex cannot produce its model table

- **GIVEN** a sync wrote Codex's model table with a program whose file then changes into one that runs and prints no
  table
- **WHEN** `aigw sync` runs, and runs again
- **THEN** each run SHALL leave Codex's file, AIGW's record and the table byte-identical and exit non-zero
- **AND** each run and `aigw check --depth local` SHALL say that Codex's program cannot produce its model table, with
  the fix that names another Codex program.

#### Scenario: A dropped file is withdrawn while the client is stopped

- **WHEN** `aigw client edit codex --target <file>` runs while Codex's configured program is missing
- **THEN** AIGW SHALL withdraw its settings, its record and its model table from the file that the edit drops
- **AND** it SHALL write nothing to the new file and exit non-zero with the stop.

#### Scenario: A selection meets a stopped client

- **WHEN** `aigw use codex <provider/model>` runs while Codex's configured program is missing
- **THEN** the selection SHALL be saved and Codex's files SHALL stay as they were
- **AND** the command SHALL keep its own report, its JSON fields among them, and then exit non-zero with the missing
  program, its fix and what became of Codex's files.

#### Scenario: A rollback meets a stopped client

- **WHEN** `aigw config rollback` runs while Codex's configured program is missing
- **THEN** `aigw.toml` SHALL be restored and Codex's files SHALL stay as they were
- **AND** the command SHALL exit non-zero, saying that `aigw.toml` was restored and that Codex's files were not
  written.

#### Scenario: A preview names the stopped clients

- **WHEN** `aigw sync --dry-run`, or the preview of another command that writes client files, runs while Codex's
  configured program is missing
- **THEN** it SHALL report its own plan of the other clients' files and then Codex's problem and fix, write nothing and
  exit non-zero.

#### Scenario: The repair refuses a program that cannot run

- **WHEN** `aigw client edit` or `aigw client enable` names a `--program` that is not an existing regular file with an
  execute bit
- **THEN** it SHALL refuse before anything changes, without running that file.

### Requirement: A client's projection follows its intent and AIGW's record

Whether a client's files count as projected SHALL follow from the client's enabled state, its targets and AIGW's
record of those files, never from whether the configuration holds a program path. `aigw status`, `aigw check` and every
command that writes client files SHALL report a client that its program stops as stopped, not as unprojected.

#### Scenario: A declared extensions directory without a program path

- **GIVEN** Codex is enabled with its targets, declares `extensions` and no `program`, and AIGW's record holds the
  files that a sync wrote
- **WHEN** `aigw status` or `aigw check --depth local` runs
- **THEN** Codex's files SHALL be judged against that record, and reported as projected when they match it.

#### Scenario: A stopped client is not reported as unprojected

- **GIVEN** Codex is enabled, AIGW's record holds its files, and its declared program is then removed
- **WHEN** `aigw status` or `aigw check --depth local` runs
- **THEN** Codex SHALL be reported as stopped by its missing program, with the stop's cause and repair
- **AND** not as a client whose files AIGW has not written.

#### Scenario: A client whose declaration was deleted keeps its record

- **GIVEN** Codex is enabled, AIGW's record holds the files that a sync wrote, and the operator deleted its declaration
- **WHEN** `aigw status` or `aigw check --depth local` runs
- **THEN** Codex's files SHALL be judged against that record, and the step SHALL be the one that gives Codex a program.

#### Scenario: A client without a usable program names its declaration

- **GIVEN** Codex is enabled, declares neither `program` nor `extensions`, and discovery finds no program outside an
  editor's extensions
- **WHEN** `aigw status` or `aigw check --depth local` runs
- **THEN** the step SHALL name `--program <path>` and `--extensions <dir>`, not only a synchronization that would find
  nothing.

#### Scenario: A disabled client keeps its declaration

- **WHEN** a client is disabled while its `program` stays declared
- **THEN** its files SHALL count as not projected, and the declaration SHALL stay in `aigw.toml`.

### Requirement: An unchanged Codex program is probed once per process

Within one AIGW process, reading Codex's bundled model table SHALL start an unchanged Codex program at most once for
its version and once for its table, and every later read in the process SHALL receive the same identity, table and
failure. Concurrent reads of one unchanged program SHALL wait for its one probe.

#### Scenario: A sync plans and applies an unchanged Codex

- **GIVEN** a Codex program that prints its version and its model table
- **WHEN** one sync plans and then applies Codex's files
- **THEN** the program SHALL have been started once with `--version` and once with `debug models --bundled`.

#### Scenario: A probe fails

- **GIVEN** a Codex program that reports its version and cannot print its model table
- **WHEN** the table is read twice in one process
- **THEN** both reads SHALL report the same failure with the same measured identity
- **AND** the program SHALL have been started once for its version and once for its table.

#### Scenario: Reads of one program overlap

- **WHEN** several reads of one unchanged program start while its probe runs
- **THEN** the program SHALL have been started once for its version and once for its table
- **AND** every read SHALL receive the same identity, table and failure.

### Requirement: A changed Codex program is probed again

A Codex program SHALL count as unchanged only while its configured path, the file that the path resolves to, that
file's bytes and the version that its package manifest declares stay the same; a change of any of them SHALL start the
program again at the next read. A read whose program file cannot be read SHALL start no program and SHALL keep nothing
for a later read.

#### Scenario: The program changes between two reads

- **GIVEN** a read of a Codex program's model table earlier in the process
- **WHEN** the program's bytes, the version that its package manifest declares, the file that its configured path
  resolves to or the configured path itself changes, and the table is read again
- **THEN** the program SHALL be started again.

#### Scenario: The program file cannot be read

- **GIVEN** a configured Codex path that names no file
- **WHEN** the table is read, the program is then written at that path, and the table is read again
- **THEN** the first read SHALL report that the program cannot be read and start nothing
- **AND** the second read SHALL start the program.
