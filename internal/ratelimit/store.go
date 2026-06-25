package ratelimit

import "time"

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

func init() {
	// 定期清理空闲超过 30 分钟的令牌桶，避免瞬时 principal（如轮换的 API Key、匿名调用方）
	// 残留的桶导致内存无限增长。每 10 分钟扫描一次。
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			Default.Cleanup(30 * time.Minute)
		}
	}()
}
