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
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		report.addFinding(Finding{Rule: "text_trailing_blank_line", Path: relative, Line: len(lines), Message: "text files must not end with blank lines"})
	}
	protected := fencedMarkdownLines(relative, lines)
	blankStart := 0
	for index, line := range lines {
		number := index + 1
		if strings.TrimRight(line, " \t") != line {
			report.addFinding(Finding{Rule: "text_trailing_whitespace", Path: relative, Line: number, Message: "remove trailing whitespace"})
		}
		if protected[number] {
			blankStart = 0
			continue
		}
		if strings.TrimSpace(line) == "" {
			if blankStart == 0 {
				blankStart = number
			}
			continue
		}
		if blankStart != 0 && number-blankStart > 1 {
			report.addFinding(Finding{Rule: "text_blank_run", Path: relative, Line: blankStart, Message: "use at most one consecutive blank line"})
		}
		blankStart = 0
	}
	checkConfigTableBoundaries(relative, lines, report)
}

func fencedMarkdownLines(relative string, lines []string) map[int]bool {
	protected := map[int]bool{}
	if filepath.Ext(relative) != ".md" {
		return protected
	}
	active := false
	for index, line := range lines {
		number := index + 1
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			active = !active
			protected[number] = true
		} else if active {
			protected[number] = true
		}
	}
	return protected
}

func checkConfigTableBoundaries(relative string, lines []string, report *Report) {
	extension := strings.ToLower(filepath.Ext(relative))
	if extension != ".toml" && extension != ".ini" {
		return
	}
	for index, line := range lines {
		if !strings.HasPrefix(line, "[") || index == 0 {
			continue
		}
		commentStart := index
		for commentStart > 0 {
			trimmed := strings.TrimSpace(lines[commentStart-1])
			if !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, ";") {
				break
			}
			commentStart--
		}
		if commentStart > 0 && strings.TrimSpace(lines[commentStart-1]) != "" {
			report.addFinding(Finding{Rule: "config_table_boundary", Path: relative, Line: commentStart + 1, Message: "use one blank line before a configuration table"})
		}
	}
}
