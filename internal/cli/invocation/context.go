// Package invocation defines the capabilities available to one CLI invocation.
package invocation

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"aigw-cli/internal/account"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/presentation"
	"aigw-cli/internal/process"
	"aigw-cli/internal/prompt"
	"aigw-cli/internal/secrets"
	"aigw-cli/internal/selfupdate"
	"aigw-cli/internal/synchronization"
)

type Runner interface {
	Run(context.Context, process.Plan) error
}

// HTTPDoer executes one HTTP request.
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type Prompter interface {
	Secret(label string) (string, error)
	Text(label string) (string, error)
	Select(label string, choices []prompt.Choice) (string, error)
}

// Updater performs one verified program lifecycle transition.
type Updater interface {
	Update(context.Context, string) (string, error)
	UpdateCandidate(context.Context, string, selfupdate.CandidateArchive) (string, error)
	Rollback(context.Context) (string, error)
}

// Context carries capabilities for one command execution without product-global state.
type Context struct {
	Version            string
	Executable         string
	InstallTarget      string
	ClaudeSettingsPath string
	Config             configuration.Store
	Secrets            secrets.Store
	Accounts           account.Store
	In                 io.Reader
	Out                io.Writer
	Color              bool
	Width              int
	Interactive        bool
	Runner             Runner
	HTTP               HTTPDoer
	Prompt             Prompter
	Discovery          discovery.Discoverer
	Updater            Updater
	RenderOut          io.Writer
	Now                func() time.Time
	Problem            func(title, evidence, impact, fix string, cause error) error
}

// ReadToken reads an explicitly requested token source without consulting
// process-global input or environment state.
func ReadToken(runtime Context, stdinMode bool, confirm bool) (string, error) {
	if stdinMode {
		if runtime.In == nil {
			return "", fmt.Errorf("token input is unavailable")
		}
		reader := bufio.NewReader(runtime.In)
		value, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return "", fmt.Errorf("Failed to read token from standard input: %w", err)
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return "", fmt.Errorf("Empty tokens are not accepted")
		}
		return value, nil
	}
	if !runtime.Interactive {
		return "", fmt.Errorf("Token input requires an interactive terminal; pipe it to `aigw` with --token-stdin")
	}
	return prompt.ReadHiddenToken(runtime.Out, confirm)
}

// Renderer creates the presentation projection bound to this invocation.
func Renderer(runtime Context) *presentation.Renderer {
	out := runtime.RenderOut
	if out == nil {
		out = runtime.Out
	}
	if out == nil {
		out = io.Discard
	}
	return presentation.NewWithWidth(out, runtime.Color, runtime.Width)
}

// Discover reads client discovery through the invocation capability boundary.
func Discover(runtime Context) (discovery.Result, error) {
	if runtime.Discovery == nil {
		return discovery.Result{}, fmt.Errorf("client discovery is unavailable")
	}
	return runtime.Discovery.Discover(), nil
}

// Problem preserves a structured user-facing error when the composition root supplies one.
func Problem(runtime Context, title, evidence, impact, fix string, cause error) error {
	if runtime.Problem != nil {
		return runtime.Problem(title, evidence, impact, fix, cause)
	}
	return cause
}

// Title capitalizes a stable client identifier for presentation.
func Title(value string) string {
	if value == "" {
		return value
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

// Synchronizer assembles the synchronization domain from invocation
// capabilities without leaking CLI composition details into domain packages.
func Synchronizer(runtime Context) synchronization.Synchronizer {
	return synchronization.Synchronizer{
		Config:             runtime.Config,
		Secrets:            runtime.Secrets,
		Runner:             runtime.Runner,
		Discovery:          runtime.Discovery,
		ClaudeSettingsPath: runtime.ClaudeSettingsPath,
	}
}
