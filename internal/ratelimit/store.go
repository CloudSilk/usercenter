package ratelimit

// Default is a package-level singleton Limiter initialized with 10 tokens/sec and burst 20.
var Default = New(10, 20)

// CheckRateLimit checks whether a request from principalID is allowed under the default limiter.
func CheckRateLimit(principalID string) bool {
	return Default.Allow(principalID)
}

// SetBudgetRateLimit configures a per-principal rate limit on the default limiter.
func SetBudgetRateLimit(principalID string, ratePerSecond, burst int) {
	Default.SetConfig(principalID, float64(ratePerSecond), burst)
}
