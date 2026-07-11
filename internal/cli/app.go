package cli

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/account"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/config"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/discovery"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/platform"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/secrets"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/selfupdate"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/shims"
	"golang.org/x/term"
)

type Runner interface {
	Run(context.Context, adapters.ProcessPlan) error
}

// CaptureRunner is implemented by runners that can return bounded process
// output without rendering it to the user's terminal. Protocol verification
// uses it to prove the expected sentinel was returned, rather than treating a
// zero process exit as sufficient evidence.
type CaptureRunner interface {
	RunCapture(context.Context, adapters.ProcessPlan) ([]byte, error)
}

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type Choice struct {
	Value string
	Label string
}

type Prompter interface {
	Secret(label string) (string, error)
	Text(label string) (string, error)
	Select(label string, choices []Choice) (string, error)
}

type Updater interface {
	Update(context.Context, string) (string, error)
}

type App struct {
	Config      config.Store
	Secrets     secrets.Store
	Accounts    account.Store
	In          io.Reader
	Out         io.Writer
	Err         io.Writer
	Interactive bool
	Color       bool
	Runner      Runner
	HTTP        HTTPDoer
	Shims       shims.Manager
	Prompt      Prompter
	Discovery   discovery.Discoverer
	Updater     Updater
}

type ProcessRunner struct{}

const capturedProcessOutputLimit = 64 * 1024

var errCapturedProcessOutputLimit = errors.New("captured process output exceeds limit")

type limitedBuffer struct {
	bytes.Buffer
	limit    int
	overflow bool
}

func (b *limitedBuffer) Write(data []byte) (int, error) {
	remaining := b.limit - b.Len()
	if remaining <= 0 {
		b.overflow = true
		return 0, errCapturedProcessOutputLimit
	}
	if len(data) > remaining {
		_, _ = b.Buffer.Write(data[:remaining])
		b.overflow = true
		return remaining, errCapturedProcessOutputLimit
	}
	return b.Buffer.Write(data)
}

func Execute(app *App, args []string) error {
	if mutationCommand(app, args) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		unlock, err := app.Config.Lock(ctx)
		if err != nil {
			return fmt.Errorf("%w; retry after the other command finishes", err)
		}
		defer unlock()
	}
	root := NewRoot(app)
	root.SetArgs(args)
	err := root.Execute()
	if err != nil {
		RenderError(app, err)
		return presented(err)
	}
	return nil
}

func mutationCommand(app *App, args []string) bool {
	if len(args) == 0 {
		cfg, err := app.Config.Load()
		return err == nil && len(cfg.Profiles) == 0 && app.Interactive
	}
	switch args[0] {
	case "setup", "add", "use", "rotate", "sync", "rollback":
		return true
	case "repair", "update":
		return true
	case "account":
		return len(args) > 1 && (args[1] == "connect" || args[1] == "disconnect")
	case "profile":
		return len(args) > 1 && (args[1] == "edit" || args[1] == "rename" || args[1] == "remove")
	case "route":
		return len(args) > 1 && args[1] == "reset"
	case "adapter":
		return len(args) > 1 && (args[1] == "enable" || args[1] == "auth" || args[1] == "disable")
	case "config":
		return len(args) > 1 && (args[1] == "import" || args[1] == "migrate")
	default:
		return false
	}
}

func (ProcessRunner) Run(ctx context.Context, plan adapters.ProcessPlan) error {
	if plan.Replace {
		return replaceProcess(plan)
	}
	cmd := exec.CommandContext(ctx, plan.Executable, plan.Args...)
	cmd.Env = plan.Env
	cmd.Stdin = strings.NewReader(plan.Stdin)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run %s: %w", plan.Executable, err)
	}
	return nil
}

// RunCapture runs a bounded, non-interactive process invocation. It never
// embeds captured output in returned errors, so a misbehaving client cannot
// accidentally surface process environment or response material.
func (ProcessRunner) RunCapture(ctx context.Context, plan adapters.ProcessPlan) ([]byte, error) {
	if plan.Replace {
		return nil, fmt.Errorf("captured execution cannot replace the current process")
	}
	cmd := exec.CommandContext(ctx, plan.Executable, plan.Args...)
	cmd.Env = plan.Env
	cmd.Stdin = strings.NewReader(plan.Stdin)
	stdout := &limitedBuffer{limit: capturedProcessOutputLimit}
	stderr := &limitedBuffer{limit: capturedProcessOutputLimit}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if stdout.overflow || stderr.overflow || errors.Is(err, errCapturedProcessOutputLimit) {
			return nil, fmt.Errorf("run %s: captured output exceeded %d bytes", plan.Executable, capturedProcessOutputLimit)
		}
		return nil, fmt.Errorf("run %s: %w", plan.Executable, err)
	}
	if stdout.overflow || stderr.overflow {
		return nil, fmt.Errorf("run %s: captured output exceeded %d bytes", plan.Executable, capturedProcessOutputLimit)
	}
	return append([]byte(nil), stdout.Bytes()...), nil
}

func NewDefault() (*App, error) {
	env := environmentMap(os.Environ())
	path, err := platform.ConfigPathFor(runtime.GOOS, env)
	if err != nil {
		return nil, err
	}
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve AIGW executable: %w", err)
	}
	binDir, err := defaultShimDirFor(runtime.GOOS, env, executable)
	if err != nil {
		return nil, err
	}
	legacyBinDir, err := platform.UserBinDirFor(runtime.GOOS, env)
	if err != nil {
		return nil, err
	}
	secretStore, err := secrets.Select(env["AIGW_SECRET_BACKEND"], os.Getenv)
	if err != nil {
		return nil, err
	}
	return &App{
		Config:      config.NewStore(path),
		Secrets:     secretStore,
		Accounts:    account.NewKeyringStore(),
		In:          os.Stdin,
		Out:         os.Stdout,
		Err:         os.Stderr,
		Interactive: isTerminal(os.Stdin),
		Color:       env["NO_COLOR"] == "" && isTerminal(os.Stdout),
		Runner:      ProcessRunner{},
		HTTP:        &http.Client{},
		Shims: shims.Manager{
			GOOS:           runtime.GOOS,
			BinDir:         binDir,
			LegacyBinDir:   legacyBinDir,
			Home:           env["HOME"],
			Shell:          env["SHELL"],
			AIGWExecutable: executable,
		},
		Prompt:    terminalPrompt{in: os.Stdin, out: os.Stdout, accessible: env["NO_COLOR"] != ""},
		Discovery: discovery.Current(),
		Updater:   selfupdate.Current(executable),
	}, nil
}

func defaultShimDirFor(goos string, env map[string]string, executable string) (string, error) {
	if value := strings.TrimSpace(env["AIGW_SHIM_DIR"]); value != "" {
		return value, nil
	}
	if dir, err := platform.ShimDirFor(goos, env); err == nil {
		return dir, nil
	}
	return filepath.Dir(executable), nil
}

func environmentMap(values []string) map[string]string {
	out := map[string]string{}
	for _, value := range values {
		key, v, ok := strings.Cut(value, "=")
		if ok {
			out[key] = v
		}
	}
	return out
}

func isTerminal(file *os.File) bool {
	return file != nil && term.IsTerminal(int(file.Fd()))
}

func (a *App) readToken(stdinMode bool, confirm bool) (string, error) {
	if stdinMode {
		reader := bufio.NewReader(a.In)
		value, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return "", fmt.Errorf("read token from stdin: %w", err)
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return "", fmt.Errorf("empty token refused")
		}
		return value, nil
	}
	if !a.Interactive {
		return "", fmt.Errorf("token input requires a terminal; pipe it to `aigw` and add --token-stdin")
	}
	return readHiddenToken(a.Out, confirm)
}
