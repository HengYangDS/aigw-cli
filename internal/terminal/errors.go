package terminal

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"syscall"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"

	"github.com/pelletier/go-toml/v2"
)

type userError struct {
	problem Problem
	cause   error
}

func (e *userError) Error() string { return e.problem.Title }
func (e *userError) Unwrap() error { return e.cause }

type presentedError struct{ cause error }

func (e *presentedError) Error() string { return e.cause.Error() }
func (e *presentedError) Unwrap() error { return e.cause }

// usageError is a command invoked with an unknown command, flag or argument,
// which the help of the command that refused it explains.
type usageError struct {
	command string
	cause   error
}

func (e *usageError) Error() string { return e.cause.Error() }
func (e *usageError) Unwrap() error { return e.cause }

// Usage marks err as a usage error of the command at path: Cobra refused an
// unknown command, flag or argument before the command ran. The command's help
// is its fix unless the error names a fix of its own.
func Usage(path string, err error) error {
	return &usageError{command: path, cause: err}
}

type configurationRecovery interface {
	error
	ConfigurationRestored() bool
}

func configurationRecoveryImpact(restored bool) string {
	if restored {
		return "Configuration was restored; native client or credential entrypoint state may still need inspection."
	}
	return "Configuration restoration was incomplete; client and credential state may also have changed."
}

// ConfigurationRecoveryImpact describes the configuration recovery outcome that
// err verified, when it carries one.
func ConfigurationRecoveryImpact(err error) (string, bool) {
	recovery, ok := errors.AsType[configurationRecovery](err)
	if !ok {
		return "", false
	}
	return configurationRecoveryImpact(recovery.ConfigurationRestored()), true
}

// ProblemError creates a structured user-facing problem while preserving its underlying cause.
func ProblemError(title, evidence, impact, fix string, cause error) error {
	return &userError{problem: Problem{Title: title, Evidence: evidence, Impact: impact, Fix: fix}, cause: cause}
}

// Presented marks an error whose command result has already been rendered.
func Presented(err error) error { return &presentedError{cause: err} }

// RenderCredentialError renders only explicitly safe diagnostics, never raw credential errors.
func RenderCredentialError(renderer *Renderer, err error) {
	if _, safe := errors.AsType[*userError](err); !safe {
		err = ProblemError(
			"Credential helper could not complete",
			"", "No usable credential was returned.",
			"Check the AIGW configuration and credential backend; run `aigw sync` and reload the client's configuration.", err,
		)
	}
	RenderError(renderer, err, false)
}

// LocalFailure presents a command's failure in at, the run that failed. A
// configuration load failure names its stage, the AIGW version that stopped
// there and its fix, and a failure at any path names the fix that the path's
// condition needs, with the run's home directory and AIGW's configuration
// file, never the process's. Structured problems, configuration recovery
// outcomes, an unsupported configuration version, which carries its own
// guidance, and any other error return unchanged.
func LocalFailure(version string, at platform.PathContext, err error) error {
	if _, structured := errors.AsType[*userError](err); structured {
		return err
	}
	failure, loading := errors.AsType[*store.LoadError](err)
	_, recovery := ConfigurationRecoveryImpact(err)
	_, unsupported := errors.AsType[*domain.UnsupportedConfigVersionError](err)
	path := pathFailureIn(err)
	switch {
	case recovery || unsupported:
		return err
	case loading:
		evidence, _ := ConfigurationLoadEvidence("AIGW "+version, err)
		return ProblemError(configurationLoadTitle(failure.Phase), evidence, "", configurationFix(failure.Phase, path, at), err)
	case path == nil:
		return err
	}
	problem := problemIn(err, at)
	return ProblemError(problem.Title, problem.Evidence, problem.Impact, problem.Fix, err)
}

// configurationFix is the fix of a configuration that did not load at phase:
// the fix of the path failure that stopped its read, or the stage's own.
func configurationFix(phase store.LoadPhase, path *platform.PathConditionError, at platform.PathContext) string {
	if path != nil && path.Condition != platform.PathOther {
		return platform.PathFix(path, at)
	}
	return configurationLoadFix(phase)
}

// ConfigurationLoadEvidence states the configuration stage at which subject
// stopped, with the operation and operating-system error of a read or the
// position of a parse failure. It never includes a path or document content.
func ConfigurationLoadEvidence(subject string, err error) (string, bool) {
	failure, ok := errors.AsType[*store.LoadError](err)
	if !ok {
		return "", false
	}
	detail := ""
	switch failure.Phase {
	case store.LoadPhaseRead:
		if path := pathFailureIn(err); path != nil && path.Op != "" {
			detail = fmt.Sprintf(" (%s: %s)", path.Op, pathReason(path))
		}
	case store.LoadPhaseParse:
		if document, ok := errors.AsType[*toml.DecodeError](err); ok {
			line, column := document.Position()
			detail = fmt.Sprintf(" at line %d, column %d", line, column)
		}
	case store.LoadPhaseValidate:
	}
	return fmt.Sprintf("%s stopped at the configuration %s stage%s.", subject, failure.Phase, detail), true
}

func configurationLoadTitle(phase store.LoadPhase) string {
	switch phase {
	case store.LoadPhaseRead:
		return "Cannot read local configuration"
	case store.LoadPhaseParse:
		return "Cannot parse local configuration"
	case store.LoadPhaseValidate:
		return "Cannot validate local configuration"
	default:
		return "Cannot load local configuration"
	}
}

// ConfigurationLoadFix is the fix for a configuration that did not load. It
// names the stage at which err stopped and `aigw config path`, never a command
// that loads the configuration and so fails the same way.
func ConfigurationLoadFix(err error) string {
	var phase store.LoadPhase
	if failure, ok := errors.AsType[*store.LoadError](err); ok {
		phase = failure.Phase
	}
	return configurationLoadFix(phase)
}

func configurationLoadFix(phase store.LoadPhase) string {
	switch phase {
	case store.LoadPhaseRead:
		return platform.ConfigurationReadFix
	case store.LoadPhaseParse:
		return "Correct the configuration file at that parse position, or restore it from a backup; `aigw config path` prints its location."
	case store.LoadPhaseValidate:
		return "If a newer AIGW wrote the configuration file, use that installed AIGW; otherwise correct the values that fail validation, or restore the file from a backup. `aigw config path` prints its location."
	default:
		return "Correct the configuration file, or restore it from a backup; `aigw config path` prints its location."
	}
}

// pathTitle is the title of a path failure, whose reason reads the same on
// every platform: a path that the operator gave by its name, and a path of
// AIGW's own by the operation that failed there.
func pathTitle(path *platform.PathConditionError, at platform.PathContext) string {
	if path.Given {
		return "AIGW cannot use " + at.Name(path, path.Path) + ": " + pathReason(path)
	}
	return fmt.Sprintf("Local file access failed (%s: %s)", path.Op, pathReason(path))
}

// ManifestReadFailure presents a team file that cannot be read, naming the
// path as every path failure does, under the home directory as ~/…, with the
// step that resolves the error.
func ManifestReadFailure(path string, at platform.PathContext, err error) error {
	failure := platform.ClassifyPath(path, platform.FileExpected, err).Owned(false)
	reason := pathReason(failure)
	fix := platform.PathFix(failure, at)
	// A team file is an input that the operator names, not one to create: a
	// path that holds none, or that a file in the way blocks, names another.
	if failure.Absent() || failure.Blocked() {
		fix = "Name a team file that exists, then run the same command again."
	}
	return ProblemError("Cannot read the team file "+at.Name(failure, path)+": "+reason, "", "Nothing changed.", fix, err)
}

// fileErrorReason names an operating-system file error from a closed set, so
// no path or caller-supplied text reaches a diagnostic.
func fileErrorReason(err error) string {
	if reason, known := knownFileErrorReason(err); known {
		return reason
	}
	return "unexpected error"
}

// knownFileErrorReason names an operating-system file error from the closed
// set, and reports whether err is one.
func knownFileErrorReason(err error) (string, bool) {
	// One reason per condition on every platform, before the operating
	// system's number, whose own text differs: Windows says that a path
	// through a file, or a missing one, cannot be found.
	switch {
	case errors.Is(err, platform.ErrFileAsFolder):
		return platform.ErrFileAsFolder.Error(), true
	case errors.Is(err, fs.ErrPermission):
		return platform.PermissionReason(err), true
	case errors.Is(err, fs.ErrNotExist):
		return "no such file or directory", true
	case errors.Is(err, fs.ErrExist):
		return "file exists", true
	}
	if number, ok := errors.AsType[syscall.Errno](err); ok {
		return number.Error(), true
	}
	return "", false
}

// failureSchemaVersion is the version of the failure document that a command
// writes when it cannot produce its own.
const failureSchemaVersion = 3

// failureDocument is the JSON failure of a command that could not produce its
// own document. Its impact is present only when the failure states a specific
// one. Its next step is a command that a script can run as it stands, or
// guidance when the step needs a choice or is prose.
type failureDocument struct {
	documentHeader
	Error      string `json:"error"`
	Evidence   string `json:"evidence,omitempty"`
	Impact     string `json:"impact,omitempty"`
	NextAction string `json:"next_action,omitempty"`
	NextDetail string `json:"next_detail,omitempty"`
	Stage      string `json:"stage,omitempty"`
	Line       int    `json:"line,omitempty"`
	Column     int    `json:"column,omitempty"`
}

// configurationFailure names the configuration step at which err stopped,
// configuration_read, configuration_parse or configuration_validate, and a
// parse failure's line and column. Any other error has no stage.
func configurationFailure(err error) (stage string, line, column int) {
	failure, ok := errors.AsType[*store.LoadError](err)
	if !ok {
		return "", 0, 0
	}
	if failure.Phase == store.LoadPhaseParse {
		if document, ok := errors.AsType[*toml.DecodeError](err); ok {
			line, column = document.Position()
		}
	}
	return "configuration_" + string(failure.Phase), line, column
}

// RenderError emits one structured actionable error and records any output failure on the renderer.
func RenderError(renderer *Renderer, err error, jsonMode bool) {
	if _, ok := errors.AsType[*presentedError](err); ok {
		return
	}
	problem := ProblemFor(err)
	if jsonMode {
		failure := failureDocument{SchemaVersion: failureSchemaVersion,
			Error: problem.Title, Evidence: problem.Evidence, Impact: problem.Impact}
		failure.NextAction, failure.NextDetail = NextSteps(problem.Fix)
		failure.Stage, failure.Line, failure.Column = configurationFailure(err)
		renderer.err = WriteJSON(renderer.out, failure)
		return
	}
	renderer.Problem(problem)
}

// ProblemFor is the actionable problem that err presents: a typed problem as
// it was raised, or a product-owned message for any other error, whose
// impact states each undo that err reports as failed, with its cause, after
// any impact of its own. A typed problem states what its raiser reported.
func ProblemFor(err error) Problem {
	return problemIn(err, platform.PathContext{})
}

// problemIn is err's problem in at, the run that presents it, whose home
// directory and configuration file a path failure's fix names. A run that does
// not pass its own names no path.
func problemIn(err error, at platform.PathContext) Problem {
	// A nil error is a caller's mistake; it is reported, never dereferenced.
	if err == nil {
		return Problem{Title: "AIGW reported a failure without a cause", Fix: "aigw check"}
	}
	problem := problemFor(err, at)
	if _, structured := errors.AsType[*userError](err); structured {
		return problem
	}
	for _, failure := range failedUndos(err) {
		problem.Impact = strings.TrimSpace(problem.Impact + " " + capitalized(failure.Undo) + " failed: " + undoReason(failure.Err) + ".")
	}
	return problem
}

// failedUndos returns each failed undo that err reports, in the order in
// which its tree wraps them; [errors.Join] keeps them all.
func failedUndos(err error) []*platform.RestoreError {
	//nolint:errorlint // The walk visits every error that err joins, where errors.As stops at the first.
	switch typed := err.(type) {
	case *platform.RestoreError:
		return []*platform.RestoreError{typed}
	case interface{ Unwrap() []error }:
		var failures []*platform.RestoreError
		for _, inner := range typed.Unwrap() {
			failures = append(failures, failedUndos(inner)...)
		}
		return failures
	case interface{ Unwrap() error }:
		return failedUndos(typed.Unwrap())
	}
	return nil
}

// undoReason says why an undo failed: the operating-system error of a file,
// that the file changed after AIGW wrote it, or else the failure's own text.
func undoReason(err error) string {
	if failed, atFile := errors.AsType[*platform.FileError](err); atFile && failed.Changed {
		return "it changed after AIGW wrote it"
	}
	if reason, known := knownFileErrorReason(err); known {
		return reason
	}
	return err.Error()
}

// capitalized makes the first letter of a clause upper case.
func capitalized(clause string) string {
	if clause == "" {
		return ""
	}
	return strings.ToUpper(clause[:1]) + clause[1:]
}

func problemFor(err error, at platform.PathContext) Problem {
	// A path failure states its condition's fix, whatever wraps it, unless an
	// outer error states its own.
	user, structured := errors.AsType[*userError](err)
	failure, atFile := errors.AsType[*adapter.ApplyError](err)
	target, atTarget := errors.AsType[*adapter.TargetError](err)
	impact, recovered := ConfigurationRecoveryImpact(err)
	switch {
	case structured:
		return user.problem
	case errors.Is(err, secret.ErrNativeReaderUnverified):
		return Problem{
			Title:    "Cannot verify native Account Token access for this AIGW version",
			Evidence: "Noninteractive native-store preflight could not establish reader access.",
			Impact:   "No new client projection was applied; no Account Token was returned.",
			Fix:      "Run `aigw check --depth local` to inspect the selected backend; explicitly restage or authorize the Account Token for this AIGW version, then run `aigw sync`.",
		}
	case atFile:
		return clientFileProblem(failure, err)
	case atTarget && target.Path != "" && target.Client != "":
		return targetProblem(target, err)
	case recovered:
		// Without a file to name, status shows what each client needs, and it
		// sends no probe; a configuration that could not be restored needs the
		// local checks, which inspect it and the client files.
		fix := "aigw status"
		if recovery, _ := errors.AsType[configurationRecovery](err); !recovery.ConfigurationRestored() {
			fix = restoreFailedFix
		}
		return Problem{Title: "Updating the client files failed", Impact: impact, Fix: fix}
	}
	message := localizedErrorMessage(err, at)
	return Problem{Title: message, Fix: errorFix(err, message, at)}
}

func localizedErrorMessage(err error, at platform.PathContext) string {
	if message, ok := typedErrorMessage(err, at); ok {
		return message
	}
	return localizeCobraError(err.Error())
}

func typedErrorMessage(err error, at platform.PathContext) (string, bool) {
	if conflict, ok := errors.AsType[interface {
		error
		ProjectionConflict() bool
	}](err); ok && conflict.ProjectionConflict() {
		// The conflict's own message replaces any decoding cause it wraps; the
		// surrounding context still names the target.
		return err.Error(), true
	}
	if mismatch, ok := errors.AsType[*domain.RuntimeModelClientMismatchError](err); ok {
		return fmt.Sprintf("route %q is for %s, not %s", mismatch.ModelID, mismatch.ExpectedClient, mismatch.ActualClient), true
	}
	if unknownAccount, ok := errors.AsType[*domain.RuntimeModelUnknownProviderError](err); ok {
		return fmt.Sprintf("route %q references unknown account %q", unknownAccount.ModelID, unknownAccount.ProviderID), true
	}
	if missingEndpoint, ok := errors.AsType[*domain.RuntimeMissingEndpointError](err); ok {
		return missingEndpoint.Error(), true
	}
	if version, ok := errors.AsType[*domain.UnsupportedConfigVersionError](err); ok {
		message := fmt.Sprintf(
			"unsupported configuration version: found %d, expected %d. AIGW does not reinterpret configuration schemas",
			version.Version,
			version.ExpectedVersion,
		)
		return message, true
	}
	// An operating-system error at a path is titled by its condition; a
	// refusal that wraps a classification keeps its own text, which says why.
	if path := pathFailureIn(err); path != nil && atOperatingSystem(err) {
		return pathTitle(path, at), true
	}
	if document, ok := errors.AsType[*toml.DecodeError](err); ok {
		line, column := document.Position()
		return fmt.Sprintf("Cannot decode TOML document at line %d, column %d", line, column), true
	}
	return "", false
}

func localizeCobraError(message string) string {
	if command, ok := cobraUnknownCommand(message); ok {
		return fmt.Sprintf("unknown command %q", command)
	}
	if flag, hint, ok := cobraUnknownFlag(message); ok {
		return fmt.Sprintf("unknown option --%s%s", flag, hint)
	}
	return message
}

func cobraUnknownCommand(message string) (string, bool) {
	const prefix = `unknown command "`
	if !strings.HasPrefix(message, prefix) {
		return "", false
	}
	rest := strings.TrimPrefix(message, prefix)
	command, suffix, ok := strings.Cut(rest, `" for "`)
	if !ok || !strings.HasSuffix(suffix, `"`) || command == "" {
		return "", false
	}
	return command, true
}

// cobraUnknownFlag reads Cobra's unknown-flag error and the hint that follows
// it, such as the command's help.
func cobraUnknownFlag(message string) (string, string, bool) {
	const prefix = "unknown flag: --"
	if !strings.HasPrefix(message, prefix) {
		return "", "", false
	}
	flag, hint, _ := strings.Cut(strings.TrimPrefix(message, prefix), ";")
	flag = strings.TrimSpace(flag)
	if flag == "" || strings.ContainsAny(flag, " \t\r\n") {
		return "", "", false
	}
	if hint != "" {
		hint = ";" + hint
	}
	return flag, hint, true
}

// errorFix prefers the fix that an error states for itself: only the guard
// that refused knows which action can succeed on the same state. A failure
// that AIGW recognizes by its type has a known fix, and a usage error names
// the help of the command that refused it. Any other error names
// `aigw check`, a diagnostic that does not fail the same way.
func errorFix(err error, message string, at platform.PathContext) string {
	if fix, ok := statedFix(err, message); ok {
		return fix
	}
	if fix, ok := typedErrorFix(err, at); ok {
		return fix
	}
	if usage, ok := errors.AsType[*usageError](err); ok {
		return usage.command + " --help"
	}
	// A path of AIGW's own, which AIGW creates as it writes it.
	if path := pathFailureIn(err); path != nil {
		return platform.PathFix(path, at)
	}
	return "aigw check"
}

// StatesFix reports whether err states its own fix: a structured problem, a
// next action of err itself, or an `aigw` command in the text that err adds to
// the errors that it wraps. A fix that a wrapped cause names, or a hint that the
// presentation adds, is not err's own, so a caller that presents err unchanged
// shows no cause that it meant to keep private.
func StatesFix(err error) bool {
	if user, ok := errors.AsType[*userError](err); ok {
		return user.problem.Fix != ""
	}
	// Only err's own fix counts, never a wrapped cause's, except a path failure
	// that the operation classified, whose condition decides the fix wherever
	// it is presented.
	if refusal, ok := err.(interface{ NextAction() string }); ok && refusal.NextAction() != "" {
		return true
	}
	if path, classified := errors.AsType[*platform.PathConditionError](err); classified && path.Condition != platform.PathOther {
		return true
	}
	return mentionedAIGWCommand(ownMessage(err)) != ""
}

// ownMessage is the text that err adds to the messages of the errors that it
// wraps.
func ownMessage(err error) string {
	message, causes := err.Error(), []error{errors.Unwrap(err)}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		causes = joined.Unwrap()
	}
	for _, cause := range causes {
		if cause != nil {
			message = strings.Replace(message, cause.Error(), "", 1)
		}
	}
	return message
}

// statedFix is the next action that err states for itself, or else the
// `aigw` command that its message names.
func statedFix(err error, message string) (string, bool) {
	if refusal, ok := errors.AsType[interface {
		error
		NextAction() string
	}](err); ok && refusal.NextAction() != "" {
		return refusal.NextAction(), true
	}
	command := mentionedAIGWCommand(message)
	return command, command != ""
}

// typedErrorFix is the fix of a failure that AIGW recognizes by its type,
// independent of its text: a client without a selected Route, a configuration
// that does not load, and a TOML document that does not decode.
func typedErrorFix(err error, at platform.PathContext) (string, bool) {
	if unselected, ok := errors.AsType[*domain.RuntimeBindingUnselectedError](err); ok {
		return fmt.Sprintf("aigw use %s <provider/model>", unselected.Client), true
	}
	if failure, ok := errors.AsType[*store.LoadError](err); ok {
		return configurationFix(failure.Phase, pathFailureIn(err), at), true
	}
	if _, ok := errors.AsType[*toml.DecodeError](err); ok {
		return "Correct the value at the reported document location and rerun the command.", true
	}
	return "", false
}

func mentionedAIGWCommand(message string) string {
	start := strings.Index(message, "`aigw ")
	if start < 0 {
		return ""
	}
	value := message[start+1:]
	before, _, ok := strings.Cut(value, "`")
	if !ok {
		return ""
	}
	return before
}
