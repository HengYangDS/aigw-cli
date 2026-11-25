package terminal

import (
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// Interactive reports whether a file descriptor is attached to a terminal.
func Interactive(file *os.File) bool {
	return file != nil && term.IsTerminal(int(file.Fd()))
}

// widthFromEnvironment returns a validated terminal width declared by the environment, or zero when absent.
func widthFromEnvironment(env map[string]string) int {
	width, err := strconv.Atoi(strings.TrimSpace(env["COLUMNS"]))
	if err != nil || width <= 0 {
		return 0
	}
	return width
}

// PresentationWidth selects the usable output width from the terminal and environment without mutating either.
func PresentationWidth(out io.Writer, env map[string]string) int {
	return presentationWidth(out, env, Interactive, term.GetSize)
}

func presentationWidth(out io.Writer, env map[string]string, interactive func(*os.File) bool, size func(int) (int, int, error)) int {
	if width := widthFromEnvironment(env); width > 0 {
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

// ColorEnabled decides whether color is safe for the environment and current output stream.
func ColorEnabled(env map[string]string, interactive bool) bool {
	return interactive && env["NO_COLOR"] == ""
}
