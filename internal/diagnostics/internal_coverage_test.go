package diagnostics

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

func TestAllKindRejectsEmptyResults(t *testing.T) {
	if allKind(nil, Healthy) {
		t.Fatal("allKind(nil) = true")
	}
}

func TestWaitForRecoveryWithNoDelay(t *testing.T) {
	if err := waitForRecovery(context.Background(), 0); err != nil {
		t.Fatalf("waitForRecovery(active) error = %v", err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitForRecovery(canceled, 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("waitForRecovery(canceled) error = %v", err)
	}
}

func TestProbeWithNoTimeoutUsesProbe(t *testing.T) {
	result := probeWithTimeout(context.Background(), http.DefaultClient, domain.Runtime{}, "secret", 0)
	if result.Kind != EndpointMismatch || result.Summary != "Invalid API URL" {
		t.Fatalf("probeWithTimeout() = %#v", result)
	}
}

func TestCompactRedactsAndBoundsDetail(t *testing.T) {
	const secret = "compact-secret"
	result := compact(strings.Repeat("word ", 150)+secret, secret)
	if strings.Contains(result, secret) || !strings.HasSuffix(result, "…") || len(result) > 503 {
		t.Fatalf("compact() returned unsafe or unbounded detail: length=%d, value=%q", len(result), result)
	}
}
