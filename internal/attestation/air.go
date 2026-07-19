package attestation

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	airSurfaceID          = "jetbrains-air-codex"
	airEvidenceSource     = "air-log"
	airTimestampLayout    = "20060102 15:04:05.000"
	airEvidenceMaxAge     = 24 * time.Hour
	maxAirLogLineBytes    = 64 * 1024
	maxAirLogScannedBytes = 64 * 1024 * 1024
)

var (
	errAirLogScanLimit = errors.New("Air router log scan limit exceeded")
	airForwardingLine  = regexp.MustCompile(`^\[(\d{8} \d{2}:\d{2}:\d{2}\.\d{3}) INFO  ([0-9]+:WS:[A-Za-z0-9_-]{1,128}) f\.a\.a\.c\.w\.CodexOpenAiApiRouterServer\]\[[^\]\r\n]{1,512}\] Forwarding CallTraceId\(id=[A-Za-z0-9._:-]{1,128}\)/POST:/responses to (\S+)$`)
)

// AirOptions binds the read-only Air log observation to the current local
// Codex route and the separately inspected Air configuration state.
type AirOptions struct {
	LogDir             string
	AIGWEndpoint       string
	ConfigurationState string
	Now                time.Time
}

// AirRuntimeAttestation is deliberately bounded to stable classifications,
// counts, timestamps, and one-way hashes. It never contains a raw route, host,
// path, process identifier, trace identifier, request body, or credential.
type AirRuntimeAttestation struct {
	SurfaceID             string   `json:"surface_id"`
	ConfigurationState    string   `json:"configuration_state"`
	State                 string   `json:"state"`
	RuntimeAuthority      string   `json:"runtime_authority"`
	ObservedProcessStart  string   `json:"observed_process_start,omitempty"`
	WindowStart           string   `json:"window_start,omitempty"`
	WindowEnd             string   `json:"window_end,omitempty"`
	RequestCount          int      `json:"request_count"`
	JetBrainsRequestCount int      `json:"jetbrains_request_count"`
	AIGWRequestCount      int      `json:"aigw_request_count"`
	OtherRequestCount     int      `json:"other_request_count"`
	HostHashes            []string `json:"host_hashes"`
	HostAuthentication    string   `json:"host_authentication"`
	BillingEvidence       string   `json:"billing_evidence"`
	EvidenceSource        string   `json:"evidence_source"`
	ReadOnly              bool     `json:"read_only"`
}

type airRecord struct {
	observedAt time.Time
	generation string
	target     routeIdentity
	lineHash   [sha256.Size]byte
}

type routeIdentity struct {
	scheme   string
	hostname string
	port     string
	path     string
}

// InspectAirRuntime reads only the bounded Air router logs under options.LogDir.
// Missing, stale, future-dated, oversized, or absent forwarding evidence is a
// secret-free unknown observation rather than permission to mutate Air.
func InspectAirRuntime(options AirOptions) (AirRuntimeAttestation, error) {
	report := baseAirReport(options.ConfigurationState)
	now := options.Now
	if now.IsZero() {
		now = time.Now()
	}
	aigwRoute, ok := parseRouteIdentity(options.AIGWEndpoint)
	if !ok {
		return AirRuntimeAttestation{}, errors.New("configured Codex route is unavailable")
	}
	budget := int64(maxAirLogScannedBytes)
	currentRecords, present, err := scanAirLog(filepath.Join(options.LogDir, "air.log"), now.Location(), &budget)
	if err != nil {
		if errors.Is(err, errAirLogScanLimit) {
			return report, nil
		}
		return AirRuntimeAttestation{}, errors.New("Air router log is unavailable")
	}
	if !present || len(currentRecords) == 0 {
		return report, nil
	}
	selected := currentRecords[0]
	for _, record := range currentRecords[1:] {
		if !record.observedAt.Before(selected.observedAt) {
			selected = record
		}
	}

	records := recordsForGeneration(currentRecords, selected.generation)
	rotationCutoff := records[0].observedAt
	for _, record := range records[1:] {
		if record.observedAt.Before(rotationCutoff) {
			rotationCutoff = record.observedAt
		}
	}
	for index := 1; index <= 9; index++ {
		rotated, present, err := scanAirLog(filepath.Join(options.LogDir, "air"+strconv.Itoa(index)+".log"), now.Location(), &budget)
		if err != nil {
			if errors.Is(err, errAirLogScanLimit) {
				return report, nil
			}
			return AirRuntimeAttestation{}, errors.New("Air router log is unavailable")
		}
		if !present {
			continue
		}
		for _, record := range recordsForGeneration(rotated, selected.generation) {
			if !record.observedAt.After(rotationCutoff) {
				records = append(records, record)
			}
		}
	}
	return summarizeAirRecords(report, records, aigwRoute, now), nil
}

func baseAirReport(configurationState string) AirRuntimeAttestation {
	state := "not-a-host-mirror"
	if configurationState == "external-host-mirror" {
		state = "host-mirror-runtime-unattested"
	}
	return AirRuntimeAttestation{
		SurfaceID:          airSurfaceID,
		ConfigurationState: configurationState,
		State:              state,
		RuntimeAuthority:   "unknown",
		HostHashes:         []string{},
		HostAuthentication: "not-probed",
		BillingEvidence:    "unknown",
		EvidenceSource:     airEvidenceSource,
		ReadOnly:           true,
	}
}

func recordsForGeneration(records []airRecord, generation string) []airRecord {
	selected := make([]airRecord, 0, len(records))
	for _, record := range records {
		if record.generation == generation {
			selected = append(selected, record)
		}
	}
	return selected
}

func summarizeAirRecords(report AirRuntimeAttestation, records []airRecord, aigwRoute routeIdentity, now time.Time) AirRuntimeAttestation {
	if len(records) == 0 {
		return report
	}
	unique := make([]airRecord, 0, len(records))
	seenLines := make(map[[sha256.Size]byte]struct{}, len(records))
	for _, record := range records {
		if _, exists := seenLines[record.lineHash]; exists {
			continue
		}
		seenLines[record.lineHash] = struct{}{}
		unique = append(unique, record)
	}
	if len(unique) == 0 {
		return report
	}
	windowStart, windowEnd := unique[0].observedAt, unique[0].observedAt
	for _, record := range unique[1:] {
		if record.observedAt.Before(windowStart) {
			windowStart = record.observedAt
		}
		if record.observedAt.After(windowEnd) {
			windowEnd = record.observedAt
		}
	}
	if windowEnd.After(now) || now.Sub(windowEnd) > airEvidenceMaxAge {
		return report
	}

	hashes := map[string]struct{}{}
	for _, record := range unique {
		hashes[hashRouteAuthority(record.target)] = struct{}{}
		switch classifyRoute(record.target, aigwRoute) {
		case "aigw":
			report.AIGWRequestCount++
		case "jetbrains-ai":
			report.JetBrainsRequestCount++
		default:
			report.OtherRequestCount++
		}
	}
	report.RequestCount = len(unique)
	report.WindowStart = windowStart.UTC().Format(time.RFC3339)
	report.WindowEnd = windowEnd.UTC().Format(time.RFC3339)
	report.HostHashes = make([]string, 0, len(hashes))
	for hash := range hashes {
		report.HostHashes = append(report.HostHashes, hash)
	}
	sort.Strings(report.HostHashes)

	switch {
	case report.AIGWRequestCount > 0 && report.JetBrainsRequestCount == 0 && report.OtherRequestCount == 0:
		report.RuntimeAuthority = "aigw"
	case report.JetBrainsRequestCount > 0 && report.AIGWRequestCount == 0 && report.OtherRequestCount == 0:
		report.RuntimeAuthority = "jetbrains-ai"
	case report.AIGWRequestCount > 0 || report.JetBrainsRequestCount > 0:
		report.RuntimeAuthority = "mixed"
	default:
		report.RuntimeAuthority = "unknown"
	}
	if report.ConfigurationState == "external-host-mirror" && report.RuntimeAuthority == "jetbrains-ai" {
		report.State = "host-mirror-runtime-attested"
	}
	return report
}

func scanAirLog(path string, location *time.Location, budget *int64) ([]airRecord, bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil || !info.Mode().IsRegular() {
		return nil, false, errors.New("unavailable")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, false, errors.New("unavailable")
	}
	defer file.Close()

	reader := bufio.NewReaderSize(file, maxAirLogLineBytes)
	records := make([]airRecord, 0)
	line := make([]byte, 0, 1024)
	oversized := false
	for {
		fragment, prefix, readErr := reader.ReadLine()
		*budget -= int64(len(fragment))
		if !prefix {
			*budget--
		}
		if *budget < 0 {
			return nil, true, errAirLogScanLimit
		}
		if !oversized {
			if len(line)+len(fragment) > maxAirLogLineBytes {
				oversized = true
				line = line[:0]
			} else {
				line = append(line, fragment...)
			}
		}
		if !prefix {
			if !oversized && len(line) > 0 {
				if record, ok := parseAirRecord(line, location); ok {
					records = append(records, record)
				}
			}
			line = line[:0]
			oversized = false
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return nil, true, errors.New("unavailable")
		}
	}
	return records, true, nil
}

func parseAirRecord(line []byte, location *time.Location) (airRecord, bool) {
	matches := airForwardingLine.FindSubmatch(line)
	if len(matches) != 4 {
		return airRecord{}, false
	}
	observedAt, err := time.ParseInLocation(airTimestampLayout, string(matches[1]), location)
	if err != nil {
		return airRecord{}, false
	}
	target, ok := parseRouteIdentity(string(matches[3]))
	if !ok {
		return airRecord{}, false
	}
	return airRecord{
		observedAt: observedAt,
		generation: string(matches[2]),
		target:     target,
		lineHash:   sha256.Sum256(line),
	}, true
}

func parseRouteIdentity(raw string) (routeIdentity, bool) {
	parsed, err := url.Parse(raw)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return routeIdentity{}, false
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return routeIdentity{}, false
	}
	hostname := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if hostname == "" {
		return routeIdentity{}, false
	}
	port, ok := normalizedPort(scheme, parsed.Port())
	if !ok {
		return routeIdentity{}, false
	}
	path := parsed.EscapedPath()
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		return routeIdentity{}, false
	}
	if path != "/" {
		path = strings.TrimRight(path, "/")
	}
	return routeIdentity{scheme: scheme, hostname: hostname, port: port, path: path}, true
}

func normalizedPort(scheme, raw string) (string, bool) {
	if raw == "" {
		if scheme == "https" {
			return "443", true
		}
		if scheme == "http" {
			return "80", true
		}
		return "", false
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 || value > 65535 {
		return "", false
	}
	return strconv.Itoa(value), true
}

func classifyRoute(observed, aigw routeIdentity) string {
	if sameAIGWRoute(observed, aigw) {
		return "aigw"
	}
	if observed.scheme == "https" && (observed.hostname == "jetbrains.ai" || strings.HasSuffix(observed.hostname, ".jetbrains.ai")) {
		return "jetbrains-ai"
	}
	return "other"
}

func sameAIGWRoute(observed, configured routeIdentity) bool {
	if observed.scheme != configured.scheme || observed.hostname != configured.hostname || observed.port != configured.port {
		return false
	}
	if configured.path == "/" {
		return true
	}
	return observed.path == configured.path || strings.HasPrefix(observed.path, configured.path+"/")
}

func hashRouteAuthority(route routeIdentity) string {
	sum := sha256.Sum256([]byte("aigw-air-route-authority-v1\x00" + route.scheme + "://" + route.hostname + ":" + route.port))
	return hex.EncodeToString(sum[:])
}
