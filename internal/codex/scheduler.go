package codex

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/pelletier/go-toml/v2/unstable"
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
// before removal so a restore still returns the user's original values.
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
	for _, table := range slices.Sorted(maps.Keys(codexSchedulerKeys)) {
		for _, key := range slices.Sorted(maps.Keys(codexSchedulerKeys[table])) {
			var err error
			assignment := fmt.Sprintf("%s = %d # managed by AIGW", key, codexSchedulerKeys[table][key])
			result, err = setCodexTableAssignment(result, table, key, assignment)
			if err != nil {
				return "", err
			}
		}
	}
	// A projected key and its retired alias cannot share a table: Codex reads the
	// pair as one field declared twice and refuses to start.
	for _, table := range slices.Sorted(maps.Keys(codexRetiredSchedulerKeys)) {
		keys := append([]string(nil), codexRetiredSchedulerKeys[table]...)
		sort.Strings(keys)
		for _, key := range keys {
			var err error
			result, err = setCodexTableAssignment(result, table, key, "")
			if err != nil {
				return "", err
			}
		}
	}
	return result, validateCodexTOML(result)
}

func restoreCodexScheduler(text string, original map[string]*int) (string, error) {
	result := text
	for _, target := range codexSchedulerTargets() {
		table, key := target[0], target[1]
		name := table + "." + key
		assignment := ""
		if original[name] != nil {
			assignment = fmt.Sprintf("%s = %d", key, *original[name])
		}
		var err error
		result, err = setCodexTableAssignment(result, table, key, assignment)
		if err != nil {
			return "", err
		}
		result, err = removeEmptyCodexTable(result, table)
		if err != nil {
			return "", err
		}
	}
	return result, validateCodexTOML(result)
}

func validateCodexSchedulerState(original map[string]*int) error {
	targets := codexSchedulerTargets()
	expected := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		expected[target[0]+"."+target[1]] = struct{}{}
	}
	for name := range original {
		if _, ok := expected[name]; !ok {
			return fmt.Errorf("invalid Codex scheduler state key %q", name)
		}
	}
	if len(original) != len(expected) {
		return fmt.Errorf("incomplete Codex scheduler state")
	}
	return nil
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

func validateCodexSchedulerOwnership(state codexState, text string) error {
	if !codexSchedulerHashMatches(state.ProjectedSchedulerHash, text) {
		return fmt.Errorf("Codex config conflict: AIGW-managed scheduler keys changed; refusing to overwrite user edits")
	}
	if err := validateCodexSchedulerState(state.OriginalScheduler); err != nil {
		return fmt.Errorf("Codex config conflict: %w", err)
	}
	return nil
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
	for segment := range strings.SplitSeq(table, ".") {
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

func codexTableBounds(text, table string) (int, int, error) {
	var parser unstable.Parser
	parser.Reset([]byte(text))
	start, end := -1, len(text)
	wanted := strings.Split(table, ".")
	for parser.NextExpression() {
		node := parser.Expression()
		if node.Kind != unstable.Table && node.Kind != unstable.ArrayTable {
			continue
		}
		keys := node.Key()
		lineStart := strings.LastIndexByte(text[:node.Child().Raw.Offset], '\n') + 1
		matched, count := true, 0
		for keys.Next() {
			if count >= len(wanted) || string(keys.Node().Data) != wanted[count] {
				matched = false
			}
			count++
		}
		if start >= 0 && end == len(text) {
			end = lineStart
		}
		if node.Kind == unstable.Table && matched && count == len(wanted) {
			if start >= 0 {
				return -1, -1, fmt.Errorf("ambiguous Codex scheduler table %q", table)
			}
			start = lineStart
		}
	}
	if err := parser.Error(); err != nil {
		return -1, -1, fmt.Errorf("parse Codex scheduler table: %w", err)
	}
	return start, end, nil
}

func codexIntegerKey(text, table, key string) (int, bool, error) {
	start, end, err := codexTableBounds(text, table)
	if err != nil || start < 0 {
		return 0, false, err
	}
	section := text[start:end]
	body := section[strings.IndexByte(section, '\n')+1:]
	line, err := codexSelectionLine(body, key)
	if err != nil || line == "" {
		return 0, false, err
	}
	var value map[string]int
	if err := toml.Unmarshal([]byte(line), &value); err != nil {
		return 0, false, fmt.Errorf("parse Codex scheduler key %s.%s: %w", table, key, err)
	}
	return value[key], true, nil
}

func setCodexTableAssignment(text, table, key, assignment string) (string, error) {
	start, end, err := codexTableBounds(text, table)
	if err != nil {
		return "", err
	}
	if start < 0 {
		if assignment == "" {
			return text, nil
		}
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
	headerEnd := strings.IndexByte(section, '\n')
	if headerEnd < 0 {
		if assignment == "" {
			return text, nil
		}
		return text[:end] + "\n" + assignment + "\n" + text[end:], nil
	}
	bodyStart := start + headerEnd + 1
	body, err := setCodexSelection(text[bodyStart:end], key, assignment)
	if err != nil {
		return "", err
	}
	return text[:bodyStart] + body + text[end:], nil
}

func removeEmptyCodexTable(text, table string) (string, error) {
	start, end, err := codexTableBounds(text, table)
	if err != nil || start < 0 {
		return text, err
	}
	section := text[start:end]
	lines := strings.Split(section, "\n")
	for _, line := range lines[1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			return text, nil
		}
	}
	return text[:start] + text[end:], nil
}
