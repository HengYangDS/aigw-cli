package domain

import (
	"math"
	"testing"
)

func TestConnectionRetryPolicyRequiresTOMLIntegerBudgets(t *testing.T) {
	spec, _ := testAdmission().Spec(ClientCodex)
	for _, value := range []uint64{0, math.MaxInt64, math.MaxInt64 + 1} {
		for _, policy := range []*RetryPolicy{{RequestMaxRetries: &value}, {StreamMaxRetries: &value}} {
			err := policy.validate(ClientCodex, spec)
			if (err == nil) != (value <= math.MaxInt64) {
				t.Fatalf("budget %d validation = %v", value, err)
			}
		}
	}
}
