package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func checkGoFormat(root string) error {
	arguments := []string{"-l"}
	for _, relative := range []string{"cmd", "internal", "tools"} {
		if information, err := os.Stat(filepath.Join(root, relative)); err == nil && information.IsDir() {
			arguments = append(arguments, relative)
		} else if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("inspect Go source root %s: %w", relative, err)
		}
	}
	if len(arguments) == 1 {
		return nil
	}
	command := exec.Command("gofmt", arguments...)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("check Go formatting: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if paths := strings.TrimSpace(string(output)); paths != "" {
		return fmt.Errorf("Go files require formatting:\n%s", paths)
	}
	return nil
}
