package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func checkEnglishText(root string) error {
	output, err := exec.Command("git", "-C", root, "ls-files", "-z").Output()
	if err != nil {
		return err
	}
	for _, relative := range strings.Split(string(output), "\x00") {
		if relative == "" || strings.Contains("/"+relative+"/", "/.serena/") {
			continue
		}
		file, err := os.Open(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(file)
		line := 0
		for scanner.Scan() {
			line++
			for _, character := range scanner.Text() {
				if character >= 0x3400 && character <= 0x4dbf || character >= 0x4e00 && character <= 0x9fff || character >= 0xf900 && character <= 0xfaff {
					_ = file.Close()
					return fmt.Errorf("%s:%d: repository content must be English-only", relative, line)
				}
			}
		}
		if err := scanner.Err(); err != nil {
			_ = file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("close %s: %w", relative, err)
		}
	}
	return nil
}
