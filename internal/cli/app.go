package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/config"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/platform"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/secrets"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/shims"
)

type Runner interface {
	Run(context.Context, adapters.ProcessPlan) error
}

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type App struct {
	Config      config.Store
	Secrets     secrets.Store
	In          io.Reader
	Out         io.Writer
	Err         io.Writer
	Interactive bool
	Runner      Runner
	HTTP        HTTPDoer
	Shims       shims.Manager
}

type ProcessRunner struct{}

func (ProcessRunner) Run(ctx context.Context, plan adapters.ProcessPlan) error {
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
	binDir := filepath.Dir(executable)
	return &App{
		Config:      config.NewStore(path),
		Secrets:     secrets.NewKeyringStore(),
		In:          os.Stdin,
		Out:         os.Stdout,
		Err:         os.Stderr,
		Interactive: isTerminal(os.Stdin),
		Runner:      ProcessRunner{},
		HTTP:        &http.Client{},
		Shims:       shims.Manager{GOOS: runtime.GOOS, BinDir: binDir, AIGWExecutable: executable},
	}, nil
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
	info, err := file.Stat()
	return err == nil && (info.Mode()&os.ModeCharDevice) != 0
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
