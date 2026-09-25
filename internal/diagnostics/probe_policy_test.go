package diagnostics

import (
	"strings"
	"testing"
)

func TestCompactRedactsAndBoundsDetail(t *testing.T) {
	const secret = "compact-secret"
	result := compact(strings.Repeat("word ", 150)+secret, secret)
	if strings.Contains(result, secret) || !strings.HasSuffix(result, "…") || len(result) > 503 {
		t.Fatalf("compact() returned unsafe or unbounded detail: length=%d, value=%q", len(result), result)
	}
}
