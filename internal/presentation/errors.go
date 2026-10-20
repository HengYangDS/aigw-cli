package presentation

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"syscall"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"

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

// LocalFailure turns a configuration load failure into a problem that names
// its stage and the AIGW version that stopped there. Other errors, including
// structured problems, configuration recovery outcomes and an unsupported
// configuration version, which carries its own migration guidance, return
// unchanged.
func LocalFailure(version string, err error) error {
	if _, presented := errors.AsType[*userError](err); presented {
		return err
	}
	if _, recovery := ConfigurationRecoveryImpact(err); recovery {
		return err
	}
	if _, unsupported := errors.AsType[*configuration.UnsupportedConfigVersionError](err); unsupported {
		return err
	}
	failure, ok := errors.AsType[*configuration.LoadError](err)
	if !ok {
		return err
	}
	evidence, _ := ConfigurationLoadEvidence("AIGW "+version, err)
	return ProblemError(
		configurationLoadTitle(failure.Phase), evidence,
		"The command could not finish; inspect current state before retrying.",
		configurationLoadFix(failure.Phase), err,
	)
}

// ConfigurationLoadEvidence states the configuration stage at which subject
// stopped, with the operation and operating-system error of a read or the
// position of a parse failure. It never includes a path or document content.
func ConfigurationLoadEvidence(subject string, err error) (string, bool) {
	failure, ok := errors.AsType[*configuration.LoadError](err)
	if !ok {
		return "", false
	}
	detail := ""
	switch failure.Phase {
	case configuration.LoadPhaseRead:
		if access, ok := errors.AsType[*os.PathError](err); ok {
			detail = fmt.Sprintf(" (%s: %s)", access.Op, fileErrorReason(access.Err))
		}
	case configuration.LoadPhaseParse:
		if document, ok := errors.AsType[*toml.DecodeError](err); ok {
			line, column := document.Position()
			detail = fmt.Sprintf(" at line %d, column %d", line, column)
		}
	case configuration.LoadPhaseValidate:
	}
	return fmt.Sprintf("%s stopped at the configuration %s stage%s.", subject, failure.Phase, detail), true
}

func configurationLoadTitle(phase configuration.LoadPhase) string {
	switch phase {
	case configuration.LoadPhaseRead:
		return "Cannot read local configuration"
	case configuration.LoadPhaseParse:
		return "Cannot parse local configuration"
	case configuration.LoadPhaseValidate:
		return "Cannot validate local configuration"
	default:
		return "Cannot load local configuration"
	}
}

func configurationLoadFix(phase configuration.LoadPhase) string {
	switch phase {
	case configuration.LoadPhaseRead:
		return "Restore read access to the AIGW configuration; run `aigw doctor` to inspect it."
	case configuration.LoadPhaseParse:
		return "Correct the configuration at that position or restore it from a backup; run `aigw doctor` to inspect it."
	case configuration.LoadPhaseValidate:
		return "If a newer AIGW wrote this configuration, use that installed AIGW; otherwise run `aigw doctor` to inspect or restore it."
	default:
		return "Run `aigw doctor` to inspect or restore the configuration."
	}
}

func localFileAccessMessage(operation string, err error) string {
	return fmt.Sprintf("Local file access failed (%s: %s); run `aigw doctor` to inspect current state", operation, fileErrorReason(err))
}

// fileErrorReason names an operating-system file error from a closed set, so
// no path or caller-supplied text reaches a diagnostic.
func fileErrorReason(err error) string {
	if number, ok := errors.AsType[syscall.Errno](err); ok {
		return number.Error()
	}
	switch {
	case errors.Is(err, fs.ErrPermission):
		return "permission denied"
	case errors.Is(err, fs.ErrNotExist):
		return "no such file or directory"
	case errors.Is(err, fs.ErrExist):
		return "file exists"
	default:
		return "unexpected error"
	}
}

// RenderError emits one structured actionable error and records any output failure on the renderer.
func RenderError(renderer *Renderer, err error, jsonMode bool) {
	if _, ok := errors.AsType[*presentedError](err); ok {
		return
	}
	var problem Problem
	if user, ok := errors.AsType[*userError](err); ok {
		problem = user.problem
	} else if errors.Is(err, secrets.ErrNativeReaderUnverified) {
		problem = Problem{
			Title:    "Cannot verify native Account Token access for this AIGW version",
			Evidence: "Noninteractive native-store preflight could not establish reader access.",
			Impact:   "No new client projection was applied; no Account Token was returned.",
			Fix:      "Run `aigw doctor` to inspect the selected backend; explicitly restage or authorize the Account Token for this AIGW version, then run `aigw sync`.",
		}
	} else if impact, ok := ConfigurationRecoveryImpact(err); ok {
		problem = Problem{
			Title:  "Client projection failed",
			Impact: impact,
			Fix:    "Run `aigw doctor` before retrying.",
		}
	} else {
		message := localizedErrorMessage(err)
		problem = Problem{
			Title:  message,
			Impact: "The command could not finish; inspect current state before retrying.",
			Fix:    suggestedFix(message),
		}
	}
	if jsonMode {
		renderer.err = WriteJSON(renderer.out, struct {
			OK bool `json:"ok"`
			Problem
		}{Problem: problem})
		return
	}
	renderer.Problem(problem)
}

func localizedErrorMessage(err error) string {
	if message, ok := typedErrorMessage(err); ok {
		return message
	}
	return localizeCobraError(err.Error())
}

func typedErrorMessage(err error) (string, bool) {
	if mismatch, ok := errors.AsType[*configuration.RuntimeRouteClientMismatchError](err); ok {
		return fmt.Sprintf("route %q is for %s, not %s", mismatch.RouteID, mismatch.ExpectedClient, mismatch.ActualClient), true
	}
	if unknownAccount, ok := errors.AsType[*configuration.RuntimeRouteUnknownAccountError](err); ok {
		return fmt.Sprintf("route %q references unknown account %q", unknownAccount.RouteID, unknownAccount.AccountID), true
	}
	if missingEndpoint, ok := errors.AsType[*configuration.RuntimeMissingEndpointError](err); ok {
		return missingEndpoint.Error(), true
	}
	if version, ok := errors.AsType[*configuration.UnsupportedConfigVersionError](err); ok {
		message := fmt.Sprintf(
			"unsupported configuration version: found %d, expected %d. AIGW does not reinterpret configuration schemas",
			version.Version,
			version.ExpectedVersion,
		)
		if (version.Version == configuration.LegacyConfigVersion || version.Version == configuration.PublishedConfigVersion) && version.ExpectedVersion == configuration.ConfigVersion {
			message += "; run `aigw config migrate --dry-run`"
		}
		return message, true
	}
	if failure, ok := errors.AsType[*configuration.LoadError](err); ok {
		return configurationLoadTitle(failure.Phase) + "; run `aigw doctor` to inspect or restore it", true
	}
	if access, ok := errors.AsType[*os.PathError](err); ok {
		return localFileAccessMessage(access.Op, access.Err), true
	}
	if link, ok := errors.AsType[*os.LinkError](err); ok {
		return localFileAccessMessage(link.Op, link.Err), true
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
	if flag, ok := cobraUnknownFlag(message); ok {
		return fmt.Sprintf("unknown option --%s", flag)
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

func cobraUnknownFlag(message string) (string, bool) {
	const prefix = "unknown flag: --"
	if !strings.HasPrefix(message, prefix) {
		return "", false
	}
	flag := strings.TrimSpace(strings.TrimPrefix(message, prefix))
	if flag == "" || strings.ContainsAny(flag, " \t\r\n") {
		return "", false
	}
	return flag, true
}

func suggestedFix(message string) string {
	if command := mentionedAIGWCommand(message); command != "" {
		return command
	}
	switch {
	case strings.Contains(message, "unknown command"), strings.Contains(message, "unknown option"), strings.Contains(message, "unknown flag"):
		return "aigw --help"
	case strings.Contains(message, "unsupported configuration version"):
		return "aigw doctor"
	case strings.Contains(message, "Cannot decode TOML document"):
		return "Correct the value at the reported document location and rerun the command."
	default:
		return "aigw check"
	}
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
