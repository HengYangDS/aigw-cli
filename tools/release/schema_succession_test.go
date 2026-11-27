package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	clientverification "aigw-cli/internal/adapter/verification"
	builtinclients "aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
	"aigw-cli/internal/platform/process"
	"aigw-cli/internal/program"
	"aigw-cli/internal/store"

	"github.com/pelletier/go-toml/v2"
)

// A published predecessor can write a configuration schema that the
// candidate no longer reads: 0.11.0 writes schema 8, which this candidate
// refuses until the operator makes the edits that its refusal names. A
// journey gives each program team files in that program's own schema, and
// crosses the boundary as the upgrade note tells an operator to: keep a copy
// of aigw.toml, make the edits, and put the copy back before a program
// rollback. A journey whose programs share one schema crosses nothing.

// schemaVersion reads the version of data, a configuration or a team file.
func schemaVersion(t *testing.T, data []byte) int {
	t.Helper()
	var header struct {
		Version int `toml:"version"`
	}
	if err := toml.Unmarshal(data, &header); err != nil || header.Version == 0 {
		t.Fatalf("read the schema version: %v", err)
	}
	return header.Version
}

// candidateSchemaEdits names the edits that the candidate's refusal asks of
// a configuration of schema: none for the candidate's own schema, and none
// for a schema that it knows no edits from.
func candidateSchemaEdits(schema int) string {
	return (&domain.UnsupportedConfigVersionError{Version: schema, ExpectedVersion: domain.ConfigVersion}).Edits()
}

// installedSchema is the configuration schema of the installed program: the
// version of the team file that it exports from a home without a
// configuration, an export that writes no file.
func (j *journeyFixture) installedSchema() int {
	j.testing.Helper()
	if _, err := os.Stat(j.config); !errors.Is(err, os.ErrNotExist) {
		j.testing.Fatalf("the installed program's schema is read before a configuration exists: %v", err)
	}
	return schemaVersion(j.testing, j.run("config", "export"))
}

// configurationSchema is the version of aigw.toml.
func (j *journeyFixture) configurationSchema() int {
	j.testing.Helper()
	return schemaVersion(j.testing, readFile(j.testing, j.config))
}

// sourceReadsConfiguration reports whether the program that the journey
// installed first reads aigw.toml as it is.
func (j *journeyFixture) sourceReadsConfiguration() bool {
	j.testing.Helper()
	return j.configurationSchema() == j.sourceSchema
}

// teamFileIn returns data, a team file, in schema. A team file of the schema
// that the candidate replaced has the candidate's form apart from its version
// and a recommendation's `authentication` and `model_provider`, which no
// journey writes, so only its version line changes.
func teamFileIn(t *testing.T, data []byte, schema int) []byte {
	t.Helper()
	from := schemaVersion(t, data)
	for _, version := range []int{from, schema} {
		if version != domain.ConfigVersion && candidateSchemaEdits(version) == "" {
			t.Fatalf("a journey writes team files in schema %d and in the schema that it replaced, not in %d", domain.ConfigVersion, version)
		}
	}
	header := fmt.Sprintf("version = %d\n", from)
	if !bytes.HasPrefix(data, []byte(header)) {
		t.Fatalf("the team file does not begin with %q", header)
	}
	return append([]byte(fmt.Sprintf("version = %d\n", schema)), data[len(header):]...)
}

// readTeamFile parses the journey's team file as the candidate reads it.
func (j *journeyFixture) readTeamFile() domain.Config {
	j.testing.Helper()
	cfg, err := store.ParseTeamFile(teamFileIn(j.testing, readFile(j.testing, j.manifest), domain.ConfigVersion), builtinclients.Admission())
	if err != nil {
		j.testing.Fatal(err)
	}
	return cfg
}

// writeTeamFile writes cfg to path as a team file in the schema of the
// program that the journey installed first, which applies it.
func (j *journeyFixture) writeTeamFile(path string, cfg domain.Config) {
	j.testing.Helper()
	data, err := store.ExportTeamFile(cfg)
	if err != nil {
		j.testing.Fatal(err)
	}
	if err := os.WriteFile(path, teamFileIn(j.testing, data, j.sourceSchema), 0o600); err != nil {
		j.testing.Fatal(err)
	}
}

// candidateTeamFile is the journey's team file in the candidate's schema: the
// file itself, or a copy beside it when the first program has another
// schema.
func (j *journeyFixture) candidateTeamFile() string {
	j.testing.Helper()
	if j.sourceSchema == domain.ConfigVersion {
		return j.manifest
	}
	path := filepath.Join(j.root, "team-candidate.toml")
	if err := os.WriteFile(path, teamFileIn(j.testing, readFile(j.testing, j.manifest), domain.ConfigVersion), 0o600); err != nil {
		j.testing.Fatal(err)
	}
	return path
}

// editedConfiguration makes on data, a configuration of the schema that the
// candidate replaced, the edits that the candidate's refusal names, as an
// operator does in an editor: the version becomes the candidate's, each
// client's `executable` becomes `program`, and each `authentication`,
// `model_provider` and client `forwarding` goes. A journey's predecessor
// writes each of these keys on a line of its own and forwards no client, so
// no forwarding is set again here; a key left inside an inline table would
// fail the candidate's load, which names its line.
func editedConfiguration(t *testing.T, data []byte) []byte {
	t.Helper()
	from := schemaVersion(t, data)
	lines := strings.Split(string(data), "\n")
	if candidateSchemaEdits(from) == "" || lines[0] != fmt.Sprintf("version = %d", from) {
		t.Fatalf("the candidate names no edits for a configuration of schema %d", from)
	}
	edited := []string{fmt.Sprintf("version = %d", domain.ConfigVersion)}
	for _, line := range lines[1:] {
		key, value, assigned := strings.Cut(line, " = ")
		if assigned && (key == "authentication" || key == "model_provider" || key == "forwarding") {
			continue
		}
		if assigned && key == "executable" {
			line = "program = " + value
		}
		edited = append(edited, line)
	}
	return []byte(strings.Join(edited, "\n"))
}

// loadConfiguration loads aigw.toml as the candidate reads it. A file of the
// schema that the candidate replaced is read through the edits that its
// refusal names, made on a copy, so that a journey resolves what its
// predecessor configured while the file keeps its bytes.
func (j *journeyFixture) loadConfiguration() domain.Config {
	j.testing.Helper()
	path, data := j.config, readFile(j.testing, j.config)
	if schemaVersion(j.testing, data) != domain.ConfigVersion {
		path = filepath.Join(j.testing.TempDir(), "aigw.toml")
		if err := os.WriteFile(path, editedConfiguration(j.testing, data), 0o600); err != nil {
			j.testing.Fatal(err)
		}
	}
	cfg, err := store.NewStore(path, builtinclients.Admission()).Load()
	if err != nil {
		j.testing.Fatal(err)
	}
	return cfg
}

// journeyState is what a refused command leaves as it was: the configuration
// and its recovery state, the installed and retained programs, and every
// client file that AIGW writes.
type journeyState struct {
	configuration store.Snapshot
	programs      [2]platform.FileSnapshot
	clients       map[string]map[string]string
}

func (j *journeyFixture) captureState() journeyState {
	j.testing.Helper()
	configuration, err := store.NewStore(j.config, builtinclients.Admission()).CaptureSnapshot()
	if err != nil {
		j.testing.Fatal(err)
	}
	state := journeyState{configuration: configuration, clients: make(map[string]map[string]string)}
	for index, path := range []string{j.binary, program.RollbackPath(j.binary)} {
		if state.programs[index], err = platform.CaptureFileSnapshot(path); err != nil {
			j.testing.Fatal(err)
		}
	}
	for _, client := range deferredJourneyClientIDs() {
		state.clients[client] = j.clientProjectionSnapshot(client)
	}
	return state
}

// runRefused runs binary, which must exit non-zero on its own, and returns
// what it wrote to its two streams.
func (j *journeyFixture) runRefused(binary string, args ...string) []byte {
	j.testing.Helper()
	ctx, cancel := context.WithTimeout(j.testing.Context(), clientverification.ProtocolTimeout)
	defer cancel()
	stdout, stderr, err := (process.Runner{}).RunCaptureStreams(ctx, process.Plan{Executable: binary, Args: args, Env: j.environment})
	if _, exited := errors.AsType[*exec.ExitError](err); !exited || ctx.Err() != nil {
		j.testing.Fatalf("%s %s was not refused: %v\nstdout:\n%s\nstderr:\n%s", binary, strings.Join(args, " "), err,
			domain.Redact(string(stdout), j.sensitiveInputs...), domain.Redact(string(stderr), j.sensitiveInputs...))
	}
	return append(stdout, stderr...)
}

// requireSchemaRefusal runs binary's sync over a configuration of the schema
// that the candidate replaced: it must refuse, name the edits that the
// candidate asks for, and leave everything that journeyState holds as it
// was.
func (j *journeyFixture) requireSchemaRefusal(binary string) {
	j.testing.Helper()
	edits := candidateSchemaEdits(j.configurationSchema())
	if edits == "" {
		j.testing.Fatalf("aigw.toml has schema %d, for which the candidate names no edits", j.configurationSchema())
	}
	before := j.captureState()
	if output := j.runRefused(binary, "sync"); !bytes.Contains(output, []byte(edits)) {
		j.testing.Fatalf("the refusal of schema %d does not name its edits:\n%s", j.configurationSchema(), domain.Redact(string(output), j.sensitiveInputs...))
	}
	if !reflect.DeepEqual(before, j.captureState()) {
		j.testing.Fatal("the refusal changed the configuration, its recovery state, a program or a client file")
	}
}

// crossSchema brings aigw.toml to the candidate's schema when it has the one
// that the candidate replaced: binary, a candidate program, refuses it, and
// the journey keeps a copy and makes the edits on the text. A file that the
// candidate reads stays as it is.
func (j *journeyFixture) crossSchema(binary string) {
	j.testing.Helper()
	if j.configurationSchema() == domain.ConfigVersion {
		return
	}
	j.requireSchemaRefusal(binary)
	j.keptConfiguration = readFile(j.testing, j.config)
	if err := os.WriteFile(j.config, editedConfiguration(j.testing, j.keptConfiguration), 0o600); err != nil {
		j.testing.Fatal(err)
	}
}

// prepareProgramRollback readies a rollback to the program that the journey
// installed first. While aigw.toml has a schema that this program cannot
// read, the rollback refuses and leaves everything that journeyState holds
// as it was; the copy kept before the edits then goes back, as the upgrade
// note tells an operator.
func (j *journeyFixture) prepareProgramRollback() {
	j.testing.Helper()
	if j.sourceReadsConfiguration() {
		return
	}
	if len(j.keptConfiguration) == 0 {
		j.testing.Fatal("no copy of aigw.toml was kept before the schema edits")
	}
	before := j.captureState()
	if output := j.runRefused(j.binary, "self", "rollback"); !bytes.Contains(output, []byte("Program rollback did not complete")) {
		j.testing.Fatalf("a rollback to a program that cannot read aigw.toml did not refuse:\n%s", domain.Redact(string(output), j.sensitiveInputs...))
	}
	if !reflect.DeepEqual(before, j.captureState()) {
		j.testing.Fatal("the refused rollback changed the configuration, its recovery state, a program or a client file")
	}
	if err := os.WriteFile(j.config, j.keptConfiguration, 0o600); err != nil {
		j.testing.Fatal(err)
	}
	j.keptConfiguration = nil
}
