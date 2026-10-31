// Package gitfixture keeps repository-tool tests independent of host Git hooks
// and templates while preserving the host's other global Git settings.
package gitfixture

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rogpeppe/go-internal/robustio"
)

// Run executes tests under an owned global Git configuration and returns their
// exit code. The configuration includes the host's top-level global files and
// then names an empty owned directory as its hook directory, and
// GIT_TEMPLATE_DIR names the same directory as the template. Fixture-local
// configuration and command-line options keep their precedence, so fixtures
// retain their own hook directories. A test binary that a test runs again as a
// helper process inherits its parent's isolation and creates none, so a helper
// that exits inside a test leaves no directory behind.
func Run(tests interface{ Run() int }) int {
	if inherited() {
		return tests.Run()
	}
	directory, err := os.MkdirTemp("", directoryPrefix)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "isolate host Git policy:", err)
		return 1
	}
	code := 1
	if err := isolate(directory); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "isolate host Git policy:", err)
	} else {
		code = tests.Run()
	}
	if err := robustio.RemoveAll(directory); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "remove isolated Git policy:", err)
		code = 1
	}
	return code
}

// directoryPrefix begins the name of each isolation directory that Run creates.
const directoryPrefix = "aigw-git-"

// inherited reports whether a parent process's Run isolated Git for this
// process: both variables still name the files of that parent's directory,
// which still exists.
func inherited() bool {
	template := os.Getenv("GIT_TEMPLATE_DIR")
	directory := filepath.Dir(template)
	if filepath.Base(template) != "empty" || !strings.HasPrefix(filepath.Base(directory), directoryPrefix) ||
		os.Getenv("GIT_CONFIG_GLOBAL") != filepath.Join(directory, "config") {
		return false
	}
	info, err := os.Stat(template)
	return err == nil && info.IsDir()
}

func isolate(directory string) error {
	empty := filepath.Join(directory, "empty")
	if err := os.Mkdir(empty, 0o700); err != nil {
		return err
	}
	files, err := hostGlobalFiles()
	if err != nil {
		return err
	}
	var configuration strings.Builder
	for _, file := range files {
		configuration.WriteString("[include]\n\tpath = " + quoted(file) + "\n")
	}
	configuration.WriteString("[core]\n\thooksPath = " + quoted(empty) + "\n")
	path := filepath.Join(directory, "config")
	if err := os.WriteFile(path, []byte(configuration.String()), 0o600); err != nil {
		return err
	}
	return errors.Join(os.Setenv("GIT_CONFIG_GLOBAL", path), os.Setenv("GIT_TEMPLATE_DIR", empty))
}

// hostGlobalFiles lists the top-level global configuration files in the order
// Git reads them, as Git itself reports them, without following includes.
// Without a Git executable no Git subprocess can read host configuration.
func hostGlobalFiles() ([]string, error) {
	output, err := exec.Command("git", "config", "--list", "--show-scope", "--show-origin", "--no-includes", "-z").Output()
	if errors.Is(err, exec.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list host Git configuration: %w", err)
	}
	fields := bytes.Split(output, []byte{0})
	var files []string
	for index := 0; index+2 < len(fields); index += 3 {
		file, ok := strings.CutPrefix(string(fields[index+1]), "file:")
		if string(fields[index]) == "global" && ok && !slices.Contains(files, file) {
			files = append(files, file)
		}
	}
	return files, nil
}

func quoted(value string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value) + `"`
}
