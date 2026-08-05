package codex

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

var codexSchedulerKeys = map[string]map[string]int{
	"agents": {
		"max_concurrent_threads_per_session": codexSessionConcurrency,
		"max_depth":                          codexAgentDepth,
	},
	"features.multi_agent_v2": {
		"max_concurrent_threads_per_session": codexSessionConcurrency,
	},
}

func captureCodexScheduler(text string) (map[string]*int, error) {
	if err := validateCodexTOML(text); err != nil {
		return nil, err
	}
	original := make(map[string]*int)
	for table, keys := range codexSchedulerKeys {
		for key := range keys {
			value, present, err := codexIntegerKey(text, table, key)
			if err != nil {
				return nil, err
			}
			name := table + "." + key
			if present {
				copied := value
				original[name] = &copied
			} else {
				original[name] = nil
			}
		}
	}
	return original, nil
}

func projectCodexScheduler(text string) (string, error) {
	if err := validateCodexTOML(text); err != nil {
		return "", err
	}
	result := text
	tables := make([]string, 0, len(codexSchedulerKeys))
	for table := range codexSchedulerKeys {
		tables = append(tables, table)
	}
	sort.Strings(tables)
	for _, table := range tables {
		keys := make([]string, 0, len(codexSchedulerKeys[table]))
		for key := range codexSchedulerKeys[table] {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			var err error
			result, err = setCodexIntegerKey(result, table, key, codexSchedulerKeys[table][key])
			if err != nil {
				return "", err
			}
		}
	}
	return result, validateCodexTOML(result)
}

func restoreCodexScheduler(text string, original map[string]*int) (string, error) {
	if len(original) == 0 {
		return text, nil
	}
	result := text
	names := make([]string, 0, len(original))
	for name := range original {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		table, key, ok := strings.Cut(name, ".max_")
		if !ok {
			return "", fmt.Errorf("invalid Codex scheduler state key %q", name)
		}
		key = "max_" + key
		var err error
		if original[name] == nil {
			result, err = removeCodexIntegerKey(result, table, key)
		} else {
			result, err = setCodexIntegerKey(result, table, key, *original[name])
		}
		if err != nil {
			return "", err
		}
		result = removeEmptyCodexTable(result, table)
	}
	result = regexp.MustCompile(`(?m)^([ \t]*max_(?:concurrent_threads_per_session|depth)[ \t]*=[ \t]*[0-9]+)[ \t]*#[ \t]*managed by AIGW[ \t]*$`).ReplaceAllString(result, "$1")
	return result, validateCodexTOML(result)
}

func codexSchedulerHash(text string) string {
	values := make([]string, 0)
	for table, keys := range codexSchedulerKeys {
		for key := range keys {
			value, present, err := codexIntegerKey(text, table, key)
			if err != nil || !present {
				values = append(values, table+"."+key+"=<missing>")
				continue
			}
			values = append(values, fmt.Sprintf("%s.%s=%d", table, key, value))
		}
	}
	sort.Strings(values)
	return hashText(strings.Join(values, "\n"))
}

func validateCodexScheduler(text string) error {
	for table, keys := range codexSchedulerKeys {
		for key, expected := range keys {
			actual, present, err := codexIntegerKey(text, table, key)
			if err != nil {
				return err
			}
			if !present || actual != expected {
				return fmt.Errorf("Codex config scheduler key %s.%s does not match AIGW", table, key)
			}
		}
	}
	return nil
}

func validateCodexTOML(text string) error {
	var value map[string]any
	if err := toml.Unmarshal([]byte(text), &value); err != nil {
		return fmt.Errorf("parse Codex config: %w", err)
	}
	return nil
}

func codexTableBounds(text, table string) (int, int, bool) {
	header := "[" + table + "]"
	lineStart := 0
	for lineStart <= len(text) {
		lineEnd := strings.IndexByte(text[lineStart:], '\n')
		if lineEnd < 0 {
			lineEnd = len(text)
		} else {
			lineEnd += lineStart + 1
		}
		line := strings.TrimSpace(strings.TrimSuffix(text[lineStart:lineEnd], "\n"))
		if line == header {
			end := lineEnd
			for end < len(text) {
				nextEnd := strings.IndexByte(text[end:], '\n')
				if nextEnd < 0 {
					nextEnd = len(text)
				} else {
					nextEnd += end + 1
				}
				next := strings.TrimSpace(strings.TrimSuffix(text[end:nextEnd], "\n"))
				if strings.HasPrefix(next, "[") && strings.HasSuffix(next, "]") {
					break
				}
				end = nextEnd
			}
			return lineStart, end, true
		}
		if lineEnd == len(text) {
			break
		}
		lineStart = lineEnd
	}
	return 0, 0, false
}

func codexIntegerKey(text, table, key string) (int, bool, error) {
	start, end, present := codexTableBounds(text, table)
	if !present {
		return 0, false, nil
	}
	pattern := regexp.MustCompile(`(?m)^[ \t]*` + regexp.QuoteMeta(key) + `[ \t]*=[ \t]*([0-9]+)[ \t]*(?:#.*)?$`)
	match := pattern.FindStringSubmatch(text[start:end])
	if match == nil {
		return 0, false, nil
	}
	var value int
	if _, err := fmt.Sscanf(match[1], "%d", &value); err != nil {
		return 0, false, fmt.Errorf("parse Codex scheduler key %s.%s: %w", table, key, err)
	}
	return value, true, nil
}

func setCodexIntegerKey(text, table, key string, value int) (string, error) {
	start, end, present := codexTableBounds(text, table)
	assignment := fmt.Sprintf("%s = %d # managed by AIGW", key, value)
	if !present {
		separator := ""
		if text != "" && !strings.HasSuffix(text, "\n") {
			separator = "\n"
		}
		if strings.TrimSpace(text) != "" {
			separator += "\n"
		}
		return text + separator + "[" + table + "]\n" + assignment + "\n", nil
	}
	section := text[start:end]
	pattern := regexp.MustCompile(`(?m)^[ \t]*` + regexp.QuoteMeta(key) + `[ \t]*=.*$`)
	if pattern.MatchString(section) {
		section = pattern.ReplaceAllString(section, assignment)
	} else {
		headerEnd := strings.IndexByte(section, '\n')
		if headerEnd < 0 {
			section += "\n" + assignment + "\n"
		} else {
			headerEnd++
			section = section[:headerEnd] + assignment + "\n" + section[headerEnd:]
		}
	}
	return text[:start] + section + text[end:], nil
}

func removeCodexIntegerKey(text, table, key string) (string, error) {
	start, end, present := codexTableBounds(text, table)
	if !present {
		return text, nil
	}
	section := text[start:end]
	pattern := regexp.MustCompile(`(?m)^[ \t]*` + regexp.QuoteMeta(key) + `[ \t]*=.*(?:\n|$)`)
	section = pattern.ReplaceAllString(section, "")
	return text[:start] + section + text[end:], nil
}

func removeEmptyCodexTable(text, table string) string {
	start, end, present := codexTableBounds(text, table)
	if !present {
		return text
	}
	section := text[start:end]
	lines := strings.Split(section, "\n")
	for _, line := range lines[1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			return text
		}
	}
	return text[:start] + text[end:]
}
