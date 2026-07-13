package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/presentation"
)

type userError struct {
	problem presentation.Problem
	cause   error
}

func (e *userError) Error() string { return e.problem.Title }
func (e *userError) Unwrap() error { return e.cause }

type presentedError struct{ cause error }

func (e *presentedError) Error() string { return e.cause.Error() }
func (e *presentedError) Unwrap() error { return e.cause }

func renderer(app *App) *presentation.Renderer {
	return presentation.New(app.Out, app.Color)
}

func problem(title, evidence, impact, fix string, cause error) error {
	return &userError{problem: presentation.Problem{Title: title, Evidence: evidence, Impact: impact, Fix: fix}, cause: cause}
}

func presented(err error) error { return &presentedError{cause: err} }

func RenderError(app *App, err error) {
	var already *presentedError
	if errors.As(err, &already) {
		return
	}
	var user *userError
	if errors.As(err, &user) {
		renderer(app).Problem(user.problem)
		return
	}
	message := localizeCobraError(err.Error())
	renderer(app).Problem(presentation.Problem{
		Title:  message,
		Impact: "本次操作未完成；已提交的事务会尽量自动回滚。",
		Fix:    suggestedFix(message),
	})
}

func localizeCobraError(message string) string {
	if command, ok := cobraUnknownCommand(message); ok {
		return fmt.Sprintf("未知命令 %q", command)
	}
	if flag, ok := cobraUnknownFlag(message); ok {
		return fmt.Sprintf("未知选项 --%s", flag)
	}
	if profile, expected, actual, ok := runtimeProfileClientMismatch(message); ok {
		return fmt.Sprintf("配置 %q 仅适用于 %s，不能用于 %s", profile, expected, actual)
	}
	if profile, account, ok := runtimeProfileUnknownAccount(message); ok {
		return fmt.Sprintf("配置 %q 引用了未知服务账户 %q", profile, account)
	}
	if account, ok := runtimeMissingEndpoint(message, "Anthropic"); ok {
		return fmt.Sprintf("服务账户 %q 未设置 Anthropic 端点", account)
	}
	if account, ok := runtimeMissingEndpoint(message, "OpenAI Responses"); ok {
		return fmt.Sprintf("服务账户 %q 未设置 OpenAI Responses 端点", account)
	}
	if version, expected, ok := unsupportedConfigVersion(message); ok {
		return fmt.Sprintf("配置版本不受支持：当前为 %s，要求 %s", version, expected)
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

func runtimeProfileClientMismatch(message string) (profile, expected, actual string, ok bool) {
	const prefix = `profile "`
	if !strings.HasPrefix(message, prefix) {
		return "", "", "", false
	}
	rest := strings.TrimPrefix(message, prefix)
	profile, rest, found := strings.Cut(rest, `" is for `)
	if !found || profile == "" {
		return "", "", "", false
	}
	expected, actual, found = strings.Cut(rest, ", not ")
	if !found || expected == "" || actual == "" {
		return "", "", "", false
	}
	return profile, expected, actual, true
}

func runtimeProfileUnknownAccount(message string) (profile, account string, ok bool) {
	const prefix = `profile "`
	if !strings.HasPrefix(message, prefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(message, prefix)
	profile, rest, found := strings.Cut(rest, `" references unknown account "`)
	if !found || profile == "" || !strings.HasSuffix(rest, `"`) {
		return "", "", false
	}
	account = strings.TrimSuffix(rest, `"`)
	if account == "" {
		return "", "", false
	}
	return profile, account, true
}

func runtimeMissingEndpoint(message, protocol string) (account string, ok bool) {
	const prefix = `account "`
	if !strings.HasPrefix(message, prefix) {
		return "", false
	}
	rest := strings.TrimPrefix(message, prefix)
	account, rest, found := strings.Cut(rest, `" has no `+protocol+` endpoint`)
	if !found || account == "" || rest != "" {
		return "", false
	}
	return account, true
}

func unsupportedConfigVersion(message string) (version, expected string, ok bool) {
	const prefix = "validate config: unsupported config version "
	if !strings.HasPrefix(message, prefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(message, prefix)
	version, rest, found := strings.Cut(rest, "; expected ")
	if !found || version == "" || rest == "" {
		return "", "", false
	}
	if _, err := strconv.Atoi(version); err != nil {
		return "", "", false
	}
	if _, err := strconv.Atoi(rest); err != nil {
		return "", "", false
	}
	return version, rest, true
}

func suggestedFix(message string) string {
	if command := mentionedAIGWCommand(message); command != "" {
		return command
	}
	switch {
	case strings.Contains(message, "unknown command"), strings.Contains(message, "未知命令"), strings.Contains(message, "unknown flag"), strings.Contains(message, "未知选项"):
		return "aigw --help"
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
	end := strings.Index(value, "`")
	if end < 0 {
		return ""
	}
	return value[:end]
}

func healthImpact(cfgClients int) string {
	switch cfgClients {
	case 0:
		return "当前 API 调用不可用。"
	case 1:
		return "已配置的 AI 客户端无法正常调用。"
	default:
		return fmt.Sprintf("已配置的 %d 个 AI 客户端无法正常调用。", cfgClients)
	}
}
