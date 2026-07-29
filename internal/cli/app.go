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
	"path"
	"runtime"
	"strconv"
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
	UpdateCandidate(context.Context, string, selfupdate.CandidateArchive) (string, error)
	Rollback(context.Context) (string, error)
}

type App struct {
	GOOS        string
	DataDir     string
	Now         func() time.Time
	Version     string
	Config      config.Store
	Secrets     secrets.Store
	Accounts    account.Store
	Env         []string
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
	renderErr   error
}

type renderErrorWriter struct {
	writer io.Writer
	err    *error
}

func (w renderErrorWriter) Write(data []byte) (int, error) {
	count, writeErr := w.writer.Write(data)
	if writeErr != nil && *w.err == nil {
		*w.err = writeErr
	}
	return count, writeErr
}

type ProcessRunner struct{}

const (
	capturedProcessOutputLimit = 64 * 1024
	capturedProcessWaitDelay   = 2 * time.Second
)

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
	app.renderErr = nil
	var unlock func() error
	if mutationCommand(app, args) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		locked, err := app.Config.Lock(ctx)
		if err != nil {
			return fmt.Errorf("%w; retry after the other command finishes", err)
		}
		unlock = locked
	}
	root := NewRoot(app)
	root.SetArgs(args)
	err := root.Execute()
	if err == nil && app.renderErr != nil {
		err = app.renderErr
	}
	if unlock != nil {
		if unlockErr := unlock(); unlockErr != nil {
			if err == nil {
				err = fmt.Errorf("release config lock: %w", unlockErr)
			} else {
				err = fmt.Errorf("%w; release config lock: %v", err, unlockErr)
			}
		}
	}
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
	case "repair":
		return !hasArgument(args[1:], "--dry-run")
	case "update":
		return true
	case "account":
		if len(args) < 2 {
			return false
		}
		if args[1] == "rename" {
			return !boolArgumentEnabled(args[2:], "--dry-run")
		}
		return args[1] == "connect" || args[1] == "disconnect" || args[1] == "edit"
	case "profile":
		if len(args) < 2 {
			return false
		}
		if args[1] == "rename" {
			return !boolArgumentEnabled(args[2:], "--dry-run")
		}
		return args[1] == "add" || args[1] == "edit" || args[1] == "remove"
	case "route":
		if len(args) < 2 {
			return false
		}
		switch args[1] {
		case "reset":
			return true
		case "fallback", "restore", "recover", "recover-orphan", "settle":
			return !hasArgument(args[2:], "--dry-run")
		default:
			return false
		}
	case "adapter":
		return len(args) > 1 && (args[1] == "enable" || args[1] == "auth" || args[1] == "disable")
	case "config":
		return len(args) > 1 && args[1] == "import"
	default:
		return false
	}
}

func hasArgument(values []string, want string) bool {
	for _, value := range values {
		if value == want || strings.HasPrefix(value, want+"=") {
			return true
		}
	}
	return false
}

func boolArgumentEnabled(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
		if !strings.HasPrefix(value, want+"=") {
			continue
		}
		enabled, err := strconv.ParseBool(strings.TrimPrefix(value, want+"="))
		return err == nil && enabled
	}
	return false
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
		return nil, fmt.Errorf("a captured process cannot replace the current process")
	}
	cmd := exec.CommandContext(ctx, plan.Executable, plan.Args...)
	cmd.Env = plan.Env
	cmd.Stdin = strings.NewReader(plan.Stdin)
	cmd.WaitDelay = capturedProcessWaitDelay
	stdout := &limitedBuffer{limit: capturedProcessOutputLimit}
	stderr := &limitedBuffer{limit: capturedProcessOutputLimit}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cleanup, err := startCapturedProcess(cmd)
	if err != nil {
		return nil, fmt.Errorf("start %s: %w", plan.Executable, err)
	}
	defer cleanup()
	if err := cmd.Wait(); err != nil {
		if stdout.overflow || stderr.overflow || errors.Is(err, errCapturedProcessOutputLimit) {
			return nil, fmt.Errorf("captured output from %s exceeds %d bytes", plan.Executable, capturedProcessOutputLimit)
		}
		if errors.Is(err, exec.ErrWaitDelay) || (errors.Is(ctx.Err(), context.DeadlineExceeded) && stdout.Len()+stderr.Len() > 0) {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil, fmt.Errorf("%s exceeded its verification limit and its output pipes did not close within %s: %w", plan.Executable, capturedProcessWaitDelay, err)
			}
			return nil, fmt.Errorf("output pipes for %s did not close within %s: %w", plan.Executable, capturedProcessWaitDelay, err)
		}
		return nil, fmt.Errorf("run %s: %w", plan.Executable, err)
	}
	if stdout.overflow || stderr.overflow {
		return nil, fmt.Errorf("captured output from %s exceeds %d bytes", plan.Executable, capturedProcessOutputLimit)
	}
	return append([]byte(nil), stdout.Bytes()...), nil
}

func NewDefault() (*App, error) {
	env := environmentMap(os.Environ())
	path, err := platform.ConfigPathFor(runtime.GOOS, env)
	if err != nil {
		return nil, err
	}
	dataDir, err := platform.DataDirFor(runtime.GOOS, env)
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
	secretStore, err := secrets.Select(env["AIGW_SECRET_BACKEND"], os.Getenv)
	if err != nil {
		return nil, err
	}
	return &App{
		GOOS:        runtime.GOOS,
		DataDir:     dataDir,
		Now:         time.Now,
		Version:     Version,
		Config:      config.NewStore(path),
		Secrets:     secretStore,
		Accounts:    account.NewKeyringStore(),
		Env:         os.Environ(),
		In:          os.Stdin,
		Out:         os.Stdout,
		Err:         os.Stderr,
		Interactive: isTerminal(os.Stdin),
		Color:       colorEnabled(runtime.GOOS, env, isTerminal(os.Stdout), enableWindowsVirtualTerminal),
		Runner:      ProcessRunner{},
		HTTP:        &http.Client{},
		Shims: shims.Manager{
			GOOS:           runtime.GOOS,
			BinDir:         binDir,
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
	return executableDirFor(goos, executable), nil
}

// executableDirFor reports the directory containing executable using goos's
// own path convention rather than the host build's. filepath.Dir is bound to
// the runtime GOOS, so a cross-compiled fallback (for example computing a
// Windows-targeted answer from a POSIX build, or vice versa) must not use it
// directly: the separator it recognizes would not match the target platform.
func executableDirFor(goos, executable string) string {
	if goos == "windows" {
		return windowsDirName(executable)
	}
	return path.Dir(executable)
}

// windowsDirName is a minimal, host-independent equivalent of filepath.Dir
// for Windows-style paths, accepting both "\\" and "/" separators the way
// Windows itself does.
func windowsDirName(name string) string {
	trimmed := strings.TrimRight(name, `\/`)
	if trimmed == "" {
		return name
	}
	idx := strings.LastIndexAny(trimmed, `\/`)
	if idx < 0 {
		return "."
	}
	if idx == 0 {
		return trimmed[:1]
	}
	if idx == 2 && trimmed[1] == ':' {
		return trimmed[:idx+1]
	}
	return trimmed[:idx]
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

func presentationWidthFromEnvironment(env map[string]string) int {
	width, err := strconv.Atoi(strings.TrimSpace(env["COLUMNS"]))
	if err != nil || width <= 0 {
		return 0
	}
	return width
}

func presentationWidth(out io.Writer, env map[string]string) int {
	if width := presentationWidthFromEnvironment(env); width > 0 {
		return width
	}
	file, ok := out.(*os.File)
	if !ok || !isTerminal(file) {
		return 0
	}
	width, _, err := term.GetSize(int(file.Fd()))
	if err != nil || width <= 0 {
		return 0
	}
	return width
}

func (a *App) readToken(stdinMode bool, confirm bool) (string, error) {
	if stdinMode {
		reader := bufio.NewReader(a.In)
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
	if !a.Interactive {
		return "", fmt.Errorf("Token input requires an interactive terminal; pipe it to `aigw` with --token-stdin")
	}
	return readHiddenToken(a.Out, confirm)
}

func colorEnabled(goos string, env map[string]string, terminal bool, enableVT func() bool) bool {
	if !terminal || env["NO_COLOR"] != "" {
		return false
	}
	if goos != "windows" {
		return true
	}
	if enableVT != nil && enableVT() {
		return true
	}
	return env["WT_SESSION"] != "" || env["ANSICON"] != "" || strings.EqualFold(env["ConEmuANSI"], "ON")
}
