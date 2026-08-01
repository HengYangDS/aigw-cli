// Package account owns optional provider-native diagnostic credentials and
// results. Canonical Account configuration remains in package configuration.
package account

// Report is a provider-native, optional account diagnostic result. Core AIGW
// configuration and routing do not require this capability.
type Report struct {
	AccountBalance      float64
	TokenName           string
	TokenStatus         string
	TokenUsed           float64
	TokenRemaining      float64
	TokenUnlimitedQuota bool
	TokenRemainingCount int64
	TokenUnlimitedCount bool
	TokenExpiredAt      int64
}
