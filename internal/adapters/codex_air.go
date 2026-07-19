package adapters

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/transaction"
)

const airProjectionFingerprintDomain = "aigw-codex-full-selection-v1\x00"

const (
	AirStateExternalClean              = "external-clean"
	AirStateExternalHostMirror         = "external-host-mirror"
	AirStateOrphanedExactFullSelection = "orphaned-exact-full-selection"
	AirStatePartialOrForeignResidue    = "partial-or-foreign-residue"
)

var exactAirManagedProviderLine = regexp.MustCompile(`^model_provider = "aigw" # managed by AIGW$`)
var exactAirManagedModelLine = regexp.MustCompile(`^model = "[^"\r\n]+" # managed by AIGW$`)
var airAIGWSelectionValue = regexp.MustCompile(`^[ \t]*(?:"aigw(?:_fallback)?"|'aigw(?:_fallback)?')[ \t]*(?:#[^\r\n]*)?$`)

const airAliasDecodedKeyLimit = 64

type airTOMLKeySegment struct {
	value    string
	raw      string
	end      int
	valid    bool
	basic    bool
	overflow bool
}

type airTextSpan struct {
	start int
	end   int
}

type airProjectionLine struct {
	text string
	span airTextSpan
}

type airManagedProjection struct {
	block        string
	fingerprint  string
	removalSpans []airTextSpan
}

// AirOrphanRemovalPlan is a read-only, byte-exact removal proposal for the
// single admitted Air orphan shape. It contains no path and performs no write.
type AirOrphanRemovalPlan struct {
	Preimage                    transaction.FileSnapshot `json:"-"`
	Cleaned                     transaction.FileSnapshot `json:"-"`
	ProjectionFingerprintSHA256 string                   `json:"-"`
}

// InspectAirCodexConfig preserves sidecar-backed AIGW ownership semantics and
// otherwise compares an exact Air full selection with the admitted standalone
// projection. A host mirror remains external and never becomes AIGW-managed.
func InspectAirCodexConfig(airPath, standalonePath string) (CodexInspection, error) {
	inspection, _, _, err := inspectAirCodexConfig(airPath, standalonePath)
	return inspection, err
}

// PlanAirOrphanRemoval returns snapshots only for an exact, sidecar-absent Air
// full selection that is not the current standalone host mirror.
func PlanAirOrphanRemoval(airPath, standalonePath string) (AirOrphanRemovalPlan, error) {
	inspection, projection, preimage, err := inspectAirCodexConfig(airPath, standalonePath)
	if err != nil {
		return AirOrphanRemovalPlan{}, err
	}
	if inspection.State != AirStateOrphanedExactFullSelection || projection == nil {
		return AirOrphanRemovalPlan{}, fmt.Errorf("Air state %q is not an exact removable orphan", inspection.State)
	}
	cleaned, err := removeAirProjectionSpans(string(preimage.Data), projection.removalSpans)
	if err != nil {
		return AirOrphanRemovalPlan{}, fmt.Errorf("plan exact Air orphan removal: %w", err)
	}
	cleanedProviders, _ := topLevelAirSelectionLines(cleaned)
	if len(cleanedProviders) != 0 || hasAirAIGWResidue(cleaned) {
		return AirOrphanRemovalPlan{}, errors.New("exact Air orphan removal did not produce an unset external configuration")
	}
	return AirOrphanRemovalPlan{
		Preimage:                    preimage,
		Cleaned:                     airSnapshotWithData(preimage, []byte(cleaned)),
		ProjectionFingerprintSHA256: projection.fingerprint,
	}, nil
}

func inspectAirCodexConfig(airPath, standalonePath string) (CodexInspection, *airManagedProjection, transaction.FileSnapshot, error) {
	inspection, err := InspectCodexConfig(airPath)
	if err != nil {
		return CodexInspection{}, nil, transaction.FileSnapshot{}, err
	}
	if inspection.SidecarPresent {
		return inspection, nil, transaction.FileSnapshot{}, nil
	}
	preimage, err := transaction.CaptureFileSnapshot(airPath)
	if err != nil {
		return CodexInspection{}, nil, transaction.FileSnapshot{}, fmt.Errorf("capture Air config: %w", err)
	}
	if !preimage.Exists {
		return inspection, nil, preimage, nil
	}
	text := string(preimage.Data)
	projection, exact := exactAirManagedProjection(text)
	if !exact {
		if hasAirAIGWResidue(text) {
			inspection.State = AirStatePartialOrForeignResidue
		} else {
			inspection.State = AirStateExternalClean
		}
		inspection.AIGWManaged = false
		return inspection, nil, preimage, nil
	}
	inspection.DiskSelection = "aigw-managed"

	if standalonePath != "" {
		standaloneInspection, standaloneErr := InspectCodexConfig(standalonePath)
		if standaloneErr != nil {
			return CodexInspection{}, nil, transaction.FileSnapshot{}, fmt.Errorf("inspect standalone Codex config: %w", standaloneErr)
		}
		if isAdmittedStandaloneProjection(standaloneInspection) {
			standaloneData, readErr := os.ReadFile(standalonePath)
			if readErr != nil {
				return CodexInspection{}, nil, transaction.FileSnapshot{}, fmt.Errorf("read standalone Codex config: %w", readErr)
			}
			if standaloneProjection, ok := exactAirManagedProjection(string(standaloneData)); ok && standaloneProjection.fingerprint == projection.fingerprint {
				inspection.State = AirStateExternalHostMirror
				inspection.AIGWManaged = false
				return inspection, projection, preimage, nil
			}
		}
	}

	inspection.State = AirStateOrphanedExactFullSelection
	inspection.AIGWManaged = false
	return inspection, projection, preimage, nil
}

func isAdmittedStandaloneProjection(inspection CodexInspection) bool {
	return inspection.State == "aigw-managed" &&
		inspection.ProjectionMode == CodexProjectionFullSelection &&
		inspection.AttributionState == "recognized" &&
		inspection.AIGWManaged &&
		inspection.SidecarPresent &&
		inspection.SidecarHashMatches
}

func exactAirManagedProjection(text string) (*airManagedProjection, bool) {
	if strings.Count(text, codexBegin) != 1 ||
		strings.Count(text, codexEnd) != 1 ||
		strings.Count(text, "[model_providers.aigw") != 1 ||
		airAIGWProviderTableCount(text) != 1 ||
		strings.Contains(text, codexFallbackBegin) ||
		strings.Contains(text, codexFallbackEnd) ||
		strings.Contains(text, "[model_providers.aigw_fallback]") {
		return nil, false
	}
	providers, models := topLevelAirSelectionLines(text)
	if len(providers) != 1 || !exactAirManagedProviderLine.MatchString(normalizeAirProjectionLine(providers[0].text)) {
		return nil, false
	}
	if len(models) > 1 || (len(models) == 1 && !exactAirManagedModelLine.MatchString(normalizeAirProjectionLine(models[0].text))) {
		return nil, false
	}
	block, err := codexManagedBlockIn(text)
	if err != nil || !exactCodexManagedBlock.MatchString(block) {
		return nil, false
	}
	lines := splitAirProjectionLines(text)
	var beginLine *airProjectionLine
	for index := range lines {
		if normalizeAirProjectionLine(lines[index].text) == codexBegin {
			beginLine = &lines[index]
			break
		}
	}
	if beginLine == nil {
		return nil, false
	}
	blockStart := strings.Index(text, block)
	if blockStart < 0 || beginLine.span.end != blockStart {
		return nil, false
	}
	normalizedBlock := normalizeAirProjectionNewlines(block)
	removalSpans := []airTextSpan{providers[0].span, beginLine.span, {start: blockStart, end: blockStart + len(block)}}
	model := ""
	if len(models) == 1 {
		model = normalizeAirProjectionLine(models[0].text)
		removalSpans = append(removalSpans, models[0].span)
	}
	remainder, err := removeAirProjectionSpans(text, removalSpans)
	if err != nil || hasAirAIGWResidue(remainder) {
		return nil, false
	}
	type fingerprintPart struct {
		start int
		text  string
	}
	parts := []fingerprintPart{
		{start: providers[0].span.start, text: normalizeAirProjectionLine(providers[0].text) + "\n"},
		{start: blockStart, text: normalizedBlock},
	}
	if len(models) == 1 {
		parts = append(parts, fingerprintPart{start: models[0].span.start, text: model + "\n"})
	}
	sort.Slice(parts, func(left, right int) bool { return parts[left].start < parts[right].start })
	var fingerprintInput strings.Builder
	fingerprintInput.WriteString(airProjectionFingerprintDomain)
	for _, part := range parts {
		fingerprintInput.WriteString(part.text)
	}
	sum := sha256.Sum256([]byte(fingerprintInput.String()))
	return &airManagedProjection{
		block:        block,
		fingerprint:  hex.EncodeToString(sum[:]),
		removalSpans: removalSpans,
	}, true
}

func topLevelAirSelectionLines(text string) ([]airProjectionLine, []airProjectionLine) {
	providers := make([]airProjectionLine, 0, 1)
	models := make([]airProjectionLine, 0, 1)
	inTable := false
	for _, line := range splitAirProjectionLines(text) {
		trimmed := strings.TrimSpace(normalizeAirProjectionLine(line.text))
		if strings.HasPrefix(trimmed, "[") {
			inTable = true
		}
		if inTable {
			continue
		}
		key, _, ok := airTopLevelAssignment(line.text)
		if !ok {
			continue
		}
		if airKeyMatchesAlias(key, "model_provider") {
			providers = append(providers, line)
		}
		if airKeyMatchesAlias(key, "model") {
			models = append(models, line)
		}
	}
	return providers, models
}

func splitAirProjectionLines(text string) []airProjectionLine {
	lines := make([]airProjectionLine, 0, strings.Count(text, "\n")+1)
	for start := 0; start < len(text); {
		newline := strings.IndexByte(text[start:], '\n')
		if newline < 0 {
			lines = append(lines, airProjectionLine{text: text[start:], span: airTextSpan{start: start, end: len(text)}})
			break
		}
		lineEnd := start + newline
		lines = append(lines, airProjectionLine{text: text[start:lineEnd], span: airTextSpan{start: start, end: lineEnd + 1}})
		start = lineEnd + 1
	}
	return lines
}

func removeAirProjectionSpans(text string, spans []airTextSpan) (string, error) {
	ordered := append([]airTextSpan(nil), spans...)
	sort.Slice(ordered, func(left, right int) bool { return ordered[left].start < ordered[right].start })
	for index, span := range ordered {
		if span.start < 0 || span.end < span.start || span.end > len(text) {
			return "", errors.New("exact Air projection span is outside the captured preimage")
		}
		if index > 0 && ordered[index-1].end > span.start {
			return "", errors.New("exact Air projection spans overlap")
		}
	}
	for index := len(ordered) - 1; index >= 0; index-- {
		span := ordered[index]
		text = text[:span.start] + text[span.end:]
	}
	return text, nil
}

func normalizeAirProjectionNewlines(text string) string {
	return strings.ReplaceAll(text, "\r\n", "\n")
}

func normalizeAirProjectionLine(line string) string {
	return strings.TrimSuffix(normalizeAirProjectionNewlines(line), "\r")
}

func hasAirAIGWResidue(text string) bool {
	if strings.Contains(text, codexBegin) ||
		strings.Contains(text, codexEnd) ||
		strings.Contains(text, codexFallbackBegin) ||
		strings.Contains(text, codexFallbackEnd) ||
		strings.Contains(text, "[model_providers.aigw") ||
		airAIGWProviderTableCount(text) != 0 ||
		strings.Contains(text, "managed by AIGW") ||
		strings.Contains(text, `name = "AIGW:`) ||
		strings.Contains(text, `name = "AIGW fallback:`) {
		return true
	}
	for _, line := range splitAirProjectionLines(text) {
		if airLineSelectsAIGW(line.text) || airLineHasIncompleteProtectedAlias(line.text) {
			return true
		}
	}
	return false
}

func airAIGWProviderTableCount(text string) int {
	count := 0
	for _, line := range splitAirProjectionLines(normalizeAirProjectionNewlines(text)) {
		if airLineDeclaresAIGWProviderTable(line.text) {
			count++
		}
	}
	return count
}

func airTopLevelAssignment(line string) (airTOMLKeySegment, string, bool) {
	line = normalizeAirProjectionLine(line)
	start := skipAirTOMLKeySpace(line, 0)
	key, ok := parseAirTOMLKeySegment(line[start:])
	if !ok {
		return airTOMLKeySegment{}, "", false
	}
	end := skipAirTOMLKeySpace(line, start+key.end)
	if end >= len(line) || line[end] != '=' {
		return airTOMLKeySegment{}, "", false
	}
	return key, line[end+1:], true
}

func airLineSelectsAIGW(line string) bool {
	key, value, ok := airTopLevelAssignment(line)
	return ok && airKeyMatchesAlias(key, "model_provider") && airAIGWSelectionValue.MatchString(value)
}

func airLineHasIncompleteProtectedAlias(line string) bool {
	line = normalizeAirProjectionLine(line)
	position := skipAirTOMLKeySpace(line, 0)
	if position >= len(line) {
		return false
	}
	if line[position] == '[' {
		return airLineHasIncompleteProtectedTableAlias(line[position:])
	}
	key, ok := parseAirTOMLKeySegment(line[position:])
	if !ok {
		return false
	}
	position = skipAirTOMLKeySpace(line, position+key.end)
	if position < len(line) && line[position] == '=' {
		return false
	}
	return airKeyMatchesAlias(key, "model_provider", "model")
}

func airLineHasIncompleteProtectedTableAlias(line string) bool {
	position := 1
	if position < len(line) && line[position] == '[' {
		position++
	}
	position = skipAirTOMLKeySpace(line, position)
	namespace, ok := parseAirTOMLKeySegment(line[position:])
	if !ok || !airKeyMatchesAlias(namespace, "model_providers") {
		return false
	}
	position = skipAirTOMLKeySpace(line, position+namespace.end)
	if position < len(line) && line[position] == '.' {
		return false
	}
	return !namespace.valid || position >= len(line) || line[position] != ']'
}

func airLineDeclaresAIGWProviderTable(line string) bool {
	line = normalizeAirProjectionLine(line)
	position := skipAirTOMLKeySpace(line, 0)
	if position >= len(line) || line[position] != '[' {
		return false
	}
	position++
	if position < len(line) && line[position] == '[' {
		position++
	}
	position = skipAirTOMLKeySpace(line, position)
	namespace, ok := parseAirTOMLKeySegment(line[position:])
	if !ok || !airKeyMatchesAlias(namespace, "model_providers") {
		return false
	}
	position = skipAirTOMLKeySpace(line, position+namespace.end)
	if position >= len(line) || line[position] != '.' {
		return false
	}
	position = skipAirTOMLKeySpace(line, position+1)
	provider, ok := parseAirTOMLKeySegment(line[position:])
	if !ok || !airKeyMatchesAlias(provider, "aigw", "aigw_fallback") {
		return false
	}
	position = skipAirTOMLKeySpace(line, position+provider.end)
	return position == len(line) || line[position] == ']' || line[position] == '.'
}

func skipAirTOMLKeySpace(text string, position int) int {
	for position < len(text) && (text[position] == ' ' || text[position] == '\t') {
		position++
	}
	return position
}

func parseAirTOMLKeySegment(text string) (airTOMLKeySegment, bool) {
	if text == "" {
		return airTOMLKeySegment{}, false
	}
	switch text[0] {
	case '"':
		return parseAirTOMLBasicKeySegment(text), true
	case '\'':
		if end := strings.IndexByte(text[1:], '\''); end >= 0 {
			end++
			return airTOMLKeySegment{value: text[1:end], raw: text[1:end], end: end + 1, valid: true}, true
		}
		return airTOMLKeySegment{value: text[1:], raw: text[1:], end: len(text), valid: false}, true
	default:
		end := 0
		for end < len(text) && isAirTOMLBareKeyByte(text[end]) {
			end++
		}
		if end == 0 {
			return airTOMLKeySegment{}, false
		}
		return airTOMLKeySegment{value: text[:end], raw: text[:end], end: end, valid: true}, true
	}
}

func parseAirTOMLBasicKeySegment(text string) airTOMLKeySegment {
	decoded := make([]rune, 0, 16)
	valid := true
	decodePrefix := true
	overflow := false
	for position := 1; position < len(text); {
		if text[position] == '"' {
			return airTOMLKeySegment{
				value:    string(decoded),
				raw:      text[1:position],
				end:      position + 1,
				valid:    valid,
				basic:    true,
				overflow: overflow,
			}
		}
		if text[position] == '\\' {
			r, width, ok := decodeAirTOMLBasicKeyEscape(text[position:])
			if !ok {
				valid = false
				decodePrefix = false
				position += min(2, len(text)-position)
				continue
			}
			if decodePrefix {
				decoded, overflow = appendAirAliasRune(decoded, r, overflow)
			}
			position += width
			continue
		}
		r, width := utf8.DecodeRuneInString(text[position:])
		if r == utf8.RuneError && width == 1 {
			valid = false
			decodePrefix = false
			position++
			continue
		}
		if r < 0x20 || r == 0x7f {
			valid = false
			decodePrefix = false
		}
		if decodePrefix {
			decoded, overflow = appendAirAliasRune(decoded, r, overflow)
		}
		position += width
	}
	return airTOMLKeySegment{
		value:    string(decoded),
		raw:      text[1:],
		end:      len(text),
		valid:    false,
		basic:    true,
		overflow: overflow,
	}
}

func decodeAirTOMLBasicKeyEscape(text string) (rune, int, bool) {
	if len(text) < 2 || text[0] != '\\' {
		return 0, 0, false
	}
	switch text[1] {
	case 'b':
		return '\b', 2, true
	case 't':
		return '\t', 2, true
	case 'n':
		return '\n', 2, true
	case 'f':
		return '\f', 2, true
	case 'r':
		return '\r', 2, true
	case '"':
		return '"', 2, true
	case '\\':
		return '\\', 2, true
	case 'u':
		return decodeAirTOMLUnicodeEscape(text, 4)
	case 'U':
		return decodeAirTOMLUnicodeEscape(text, 8)
	default:
		return 0, 0, false
	}
}

func decodeAirTOMLUnicodeEscape(text string, digits int) (rune, int, bool) {
	width := digits + 2
	if len(text) < width {
		return 0, 0, false
	}
	value := uint32(0)
	for _, digit := range text[2:width] {
		value <<= 4
		switch {
		case digit >= '0' && digit <= '9':
			value += uint32(digit - '0')
		case digit >= 'a' && digit <= 'f':
			value += uint32(digit-'a') + 10
		case digit >= 'A' && digit <= 'F':
			value += uint32(digit-'A') + 10
		default:
			return 0, 0, false
		}
	}
	r := rune(value)
	if !utf8.ValidRune(r) || r >= 0xd800 && r <= 0xdfff {
		return 0, 0, false
	}
	return r, width, true
}

func appendAirAliasRune(decoded []rune, r rune, overflow bool) ([]rune, bool) {
	if overflow || len(decoded) >= airAliasDecodedKeyLimit {
		return decoded, true
	}
	return append(decoded, r), false
}

func airKeyMatchesAlias(key airTOMLKeySegment, aliases ...string) bool {
	for _, alias := range aliases {
		if key.valid && !key.overflow && key.value == alias {
			return true
		}
		if !key.valid && !key.basic && !key.overflow && key.value == alias {
			return true
		}
		if !key.valid && key.basic && airMalformedBasicKeyResemblesAlias(key, alias) {
			return true
		}
	}
	return false
}

func airMalformedBasicKeyResemblesAlias(key airTOMLKeySegment, alias string) bool {
	if strings.Contains(key.raw, alias) {
		return true
	}
	return key.value != "" && (strings.HasPrefix(alias, key.value) || strings.HasPrefix(key.value, alias))
}

func isAirTOMLBareKeyByte(value byte) bool {
	return value >= 'a' && value <= 'z' ||
		value >= 'A' && value <= 'Z' ||
		value >= '0' && value <= '9' ||
		value == '_' || value == '-'
}

func airSnapshotWithData(preimage transaction.FileSnapshot, data []byte) transaction.FileSnapshot {
	sum := sha256.Sum256(data)
	return transaction.FileSnapshot{
		Exists: true,
		Data:   append([]byte(nil), data...),
		SHA256: hex.EncodeToString(sum[:]),
		Mode:   preimage.Mode,
	}
}
