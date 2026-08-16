package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func checkTextLayout(root string, report *Report) error {
	files, err := trackedFiles(root)
	if err != nil {
		return err
	}
	for _, relative := range files {
		if filepath.Ext(relative) == ".py" {
			continue
		}
		if filepath.Ext(relative) == "" && !strings.Contains(relative, "/") {
			continue
		}
		absolute := filepath.Join(root, filepath.FromSlash(relative))
		info, err := os.Lstat(absolute)
		if err != nil {
			return fmt.Errorf("stat %s: %w", relative, err)
		}
		if !info.Mode().IsRegular() {
			continue
		}
		data, err := os.ReadFile(absolute)
		if err != nil {
			return fmt.Errorf("read %s: %w", relative, err)
		}
		if bytes.IndexByte(data, 0) >= 0 {
			continue
		}
		if filepath.Ext(relative) == ".txt" {
			continue
		}
		checkTextFile(relative, data, report)
	}
	return nil
}

func checkTextFile(relative string, data []byte, report *Report) {
	if bytes.Contains(data, []byte{'\r'}) {
		report.addFinding(Finding{Rule: "text_line_ending", Path: relative, Line: 1, Message: "text files must use LF line endings"})
	}
	if len(data) == 0 {
		return
	}
	if data[len(data)-1] != '\n' {
		report.addFinding(Finding{Rule: "text_final_newline", Path: relative, Line: bytes.Count(data, []byte{'\n'}) + 1, Message: "text files must end with one newline"})
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if len(lines) > 1 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for index, line := range lines {
		number := index + 1
		if strings.TrimRight(line, " \t") != line {
			report.addFinding(Finding{Rule: "text_trailing_whitespace", Path: relative, Line: number, Message: "remove trailing whitespace"})
		}
	}
}
