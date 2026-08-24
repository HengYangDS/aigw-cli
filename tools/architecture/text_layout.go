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
	if filepath.Ext(relative) == ".md" {
		checkMarkdownTableStructure(relative, lines, report)
	}
}

func checkMarkdownTableStructure(relative string, lines []string, report *Report) {
	inFence := false
	for index := 1; index < len(lines); index++ {
		trimmed := strings.TrimSpace(lines[index])
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		delimiters, ok := markdownTableDelimiterColumns(trimmed)
		if !ok {
			continue
		}
		headings := markdownTableColumns(strings.TrimSpace(lines[index-1]))
		if headings != delimiters {
			report.addFinding(Finding{
				Rule:    "markdown_table_structure",
				Path:    relative,
				Line:    index + 1,
				Message: fmt.Sprintf("table header has %d columns but delimiter row has %d", headings, delimiters),
			})
		}
	}
}

func markdownTableDelimiterColumns(line string) (int, bool) {
	columns := splitMarkdownTableRow(line)
	if len(columns) == 0 {
		return 0, false
	}
	for _, column := range columns {
		cell := strings.TrimSpace(column)
		cell = strings.TrimPrefix(cell, ":")
		cell = strings.TrimSuffix(cell, ":")
		if len(cell) < 3 || strings.Trim(cell, "-") != "" {
			return 0, false
		}
	}
	return len(columns), true
}

func markdownTableColumns(line string) int {
	return len(splitMarkdownTableRow(line))
}

func splitMarkdownTableRow(line string) []string {
	line = strings.TrimSpace(line)
	if !strings.Contains(line, "|") {
		return nil
	}
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	columns := []string{}
	start := 0
	escaped := false
	for index, character := range line {
		switch {
		case escaped:
			escaped = false
		case character == '\\':
			escaped = true
		case character == '|':
			columns = append(columns, line[start:index])
			start = index + 1
		}
	}
	columns = append(columns, line[start:])
	return columns
}
