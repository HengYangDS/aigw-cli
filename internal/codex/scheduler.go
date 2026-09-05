package codex

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// codexSchedulerKeys are the scheduler keys AIGW projects and validates. Codex
// reads [agents].max_threads as the session concurrency field and treats
// [agents].max_concurrent_threads_per_session as its retired alias, so a table
// carrying both is rejected by Codex. AIGW therefore binds exactly one member of
// that pair per table.
var codexSchedulerKeys = map[string]map[string]int{
	"agents": {
		"max_threads": codexSessionConcurrency,
		"max_depth":   codexAgentDepth,
	},
	"features.multi_agent_v2": {
		"max_concurrent_threads_per_session": codexSessionConcurrency,
	},
}

// codexRetiredSchedulerKeys are keys AIGW must clear rather than bind, because
// the table's projected key already carries their meaning. They are captured
// before removal so a restore still returns the user's original bytes.
var codexRetiredSchedulerKeys = map[string][]string{
	"agents": {"max_concurrent_threads_per_session"},
}

// codexSchedulerTargets lists every table and key AIGW either projects or
// retires, in a deterministic order.
func codexSchedulerTargets() [][2]string {
	targets := make([][2]string, 0, len(codexSchedulerKeys)+len(codexRetiredSchedulerKeys))
	for table, keys := range codexSchedulerKeys {
		for key := range keys {
			targets = append(targets, [2]string{table, key})
		}
	}
	for table, keys := range codexRetiredSchedulerKeys {
		for _, key := range keys {
			targets = append(targets, [2]string{table, key})
		}
	}
	sort.Slice(targets, func(i, j int) bool {
		if targets[i][0] != targets[j][0] {
			return targets[i][0] < targets[j][0]
		}
		return targets[i][1] < targets[j][1]
	})
	return targets
}

func captureCodexScheduler(text string) (map[string]*int, error) {
	if err := validateCodexTOML(text); err != nil {
		return nil, err
	}
	return captureCodexSchedulerInto(make(map[string]*int), text)
}

// captureCodexSchedulerInto records the current value of every projected and
// retired key that is not recorded yet. An absent key is recorded as absent so a
// restore removes it instead of leaving AIGW's value behind.
func captureCodexSchedulerInto(original map[string]*int, text string) (map[string]*int, error) {
	for _, target := range codexSchedulerTargets() {
		name := target[0] + "." + target[1]
		if _, recorded := original[name]; recorded {
			continue
		}
		value, present, err := codexIntegerKey(text, target[0], target[1])
		if err != nil {
			return nil, err
		}
		if present {
			copied := value
			original[name] = &copied
		} else {
			original[name] = nil
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
			result = setCodexIntegerKey(result, table, key, codexSchedulerKeys[table][key])
		}
	}
	// A projected key and its retired alias cannot share a table: Codex reads the
	// pair as one field declared twice and refuses to start.
	retiredTables := make([]string, 0, len(codexRetiredSchedulerKeys))
	for table := range codexRetiredSchedulerKeys {
		retiredTables = append(retiredTables, table)
	}
	sort.Strings(retiredTables)
	for _, table := range retiredTables {
		keys := append([]string(nil), codexRetiredSchedulerKeys[table]...)
		sort.Strings(keys)
		for _, key := range keys {
			result = removeCodexIntegerKey(result, table, key)
		}
	}
	return result, validateCodexTOML(result)
}

func restoreCodexScheduler(text string, original map[string]*int) (string, error) {
	targets := codexSchedulerTargets()
	expected := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		expected[target[0]+"."+target[1]] = struct{}{}
	}
	for name := range original {
		if _, ok := expected[name]; !ok {
			return "", fmt.Errorf("invalid Codex scheduler state key %q", name)
		}
	}
	if len(original) != len(expected) {
		return "", fmt.Errorf("incomplete Codex scheduler state")
	}

	result := text
	for _, target := range targets {
		table, key := target[0], target[1]
		name := table + "." + key
		if original[name] == nil {
			result = removeCodexIntegerKey(result, table, key)
		} else {
			result = setCodexIntegerKey(result, table, key, *original[name])
		}
		result = removeEmptyCodexTable(result, table)
	}
	result = regexp.MustCompile(`(?m)^([ \t]*max_(?:concurrent_threads_per_session|depth|threads)[ \t]*=[ \t]*[0-9]+)[ \t]*#[ \t]*managed by AIGW[ \t]*$`).ReplaceAllString(result, "$1")
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
	// A retired key is owned by its absence, so the fingerprint records only
	// whether it exists. Any value at all is drift, not just an integer AIGW
	// itself could have written.
	for table, keys := range codexRetiredSchedulerKeys {
		for _, key := range keys {
			state := "<missing>"
			if present, err := codexKeyPresent(text, table, key); err != nil || present {
				state = "<present>"
			}
			values = append(values, table+"."+key+"="+state)
		}
	}
	sort.Strings(values)
	return hashText(strings.Join(values, "\n"))
}

// codexSchedulerHashMatches accepts only the current projection identity.
func codexSchedulerHashMatches(recorded, text string) bool {
	return recorded != "" && recorded == codexSchedulerHash(text)
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
	// A retired key must stay absent. Codex rejects a table carrying both members
	// of the alias pair, so a key that reappears after projection is drift and is
	// reported here rather than removed: validation reads configuration, and
	// clearing a key the user may have written belongs to synchronization.
	for table, keys := range codexRetiredSchedulerKeys {
		for _, key := range keys {
			present, err := codexKeyPresent(text, table, key)
			if err != nil {
				return err
			}
			if present {
				return fmt.Errorf("Codex config scheduler key %s.%s is retired but present beside the key AIGW projects", table, key)
			}
		}
	}
	return nil
}

// codexKeyPresent reports whether a table assigns key at all. It asks the TOML
// parser instead of matching an assignment pattern, because an absence invariant
// has to hold for every shape the parser accepts — a string, a boolean, an array,
// a quoted key — not only for the integer assignment AIGW itself would write.
// A document that does not parse cannot prove absence, so the error is returned
// and every caller treats it as unproven rather than as absent.
func codexKeyPresent(text, table, key string) (bool, error) {
	var document map[string]any
	if err := toml.Unmarshal([]byte(text), &document); err != nil {
		return false, fmt.Errorf("parse Codex config: %w", err)
	}
	current := document
	for _, segment := range strings.Split(table, ".") {
		nested, ok := current[segment].(map[string]any)
		if !ok {
			return false, nil
		}
		current = nested
	}
	_, present := current[key]
	return present, nil
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

func setCodexIntegerKey(text, table, key string, value int) string {
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
		return text + separator + "[" + table + "]\n" + assignment + "\n"
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
	return text[:start] + section + text[end:]
}

func removeCodexIntegerKey(text, table, key string) string {
	start, end, present := codexTableBounds(text, table)
	if !present {
		return text
	}
	section := text[start:end]
	pattern := regexp.MustCompile(`(?m)^[ \t]*` + regexp.QuoteMeta(key) + `[ \t]*=.*(?:\n|$)`)
	section = pattern.ReplaceAllString(section, "")
	return text[:start] + section + text[end:]
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
