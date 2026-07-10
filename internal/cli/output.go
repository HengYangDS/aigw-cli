package cli

import (
	"errors"
	"fmt"

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
	renderer(app).Problem(presentation.Problem{
		Title:  err.Error(),
		Impact: "本次操作未完成；已提交的事务会尽量自动回滚。",
		Fix:    "aigw check",
	})
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
