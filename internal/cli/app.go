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
	"time"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/config"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/discovery"
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

type Choice struct {
	Value string
	Label string
}

type Prompter interface {
	Secret(label string) (string, error)
	Select(label string, choices []Choice) (string, error)
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
	Prompt      Prompter
	Discovery   discovery.Discoverer
}

type ProcessRunner struct{}

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
	return root.Execute()
}

func mutationCommand(app *App, args []string) bool {
	if len(args) == 0 {
		cfg, err := app.Config.Load()
		return err == nil && len(cfg.Profiles) == 0 && app.Interactive
	}
	switch args[0] {
	case "setup", "add", "use", "rotate", "sync":
		return true
	case "profile":
		return len(args) > 1 && (args[1] == "edit" || args[1] == "rename" || args[1] == "remove")
	case "route":
		return len(args) > 1 && args[1] == "reset"
	case "adapter":
		return len(args) > 1 && (args[1] == "enable" || args[1] == "disable")
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
	secretStore, err := secrets.Select(env["AIGW_SECRET_BACKEND"], os.Getenv)
	if err != nil {
		return nil, err
	}
	return &App{
		Config:      config.NewStore(path),
		Secrets:     secretStore,
		In:          os.Stdin,
		Out:         os.Stdout,
		Err:         os.Stderr,
		Interactive: isTerminal(os.Stdin),
		Runner:      ProcessRunner{},
		HTTP:        &http.Client{},
		Shims:       shims.Manager{GOOS: runtime.GOOS, BinDir: binDir, AIGWExecutable: executable},
		Prompt:      terminalPrompt{in: os.Stdin, out: os.Stdout},
		Discovery:   discovery.Current(),
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
