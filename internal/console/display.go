// Package console detects terminal display capabilities without owning
// application presentation or interactive input policy.
package console

import (
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

func Interactive(file *os.File) bool {
	return file != nil && term.IsTerminal(int(file.Fd()))
}

func WidthFromEnvironment(env map[string]string) int {
	width, err := strconv.Atoi(strings.TrimSpace(env["COLUMNS"]))
	if err != nil || width <= 0 {
		return 0
	}
	return width
}

func PresentationWidth(out io.Writer, env map[string]string) int {
	return presentationWidth(out, env, Interactive, term.GetSize)
}

func presentationWidth(out io.Writer, env map[string]string, interactive func(*os.File) bool, size func(int) (int, int, error)) int {
	if width := WidthFromEnvironment(env); width > 0 {
		return width
	}
	file, ok := out.(*os.File)
	if !ok || !interactive(file) {
		return 0
	}
	width, _, err := size(int(file.Fd()))
	if err != nil || width <= 0 {
		return 0
	}
	return width
}

func ColorEnabled(goos string, env map[string]string, interactive bool, enableVT func() bool) bool {
	if !interactive || env["NO_COLOR"] != "" {
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
