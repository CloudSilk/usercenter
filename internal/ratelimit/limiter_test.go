package ratelimit

import (
	"testing"
	"time"
)

func TestLimiter_AllowWithinBurst(t *testing.T) {
	l := New(100, 5) // 100 req/s, burst 5
	for i := 0; i < 5; i++ {
		if !l.Allow("user1") {
			t.Fatalf("expected allow on attempt %d within burst", i+1)
		}
	}
}

func TestLimiter_DenyOverBurst(t *testing.T) {
	l := New(100, 3) // burst 3
	for i := 0; i < 3; i++ {
		l.Allow("user1")
	}
	if l.Allow("user1") {
		t.Fatal("expected deny after burst exhausted")
	}
}

func TestLimiter_Refill(t *testing.T) {
	l := New(100, 2) // 100 req/s, burst 2
	l.Allow("user1")
	l.Allow("user1")
	// 立即再请求应被拒
	if l.Allow("user1") {
		t.Fatal("expected deny immediately after burst")
	}
	// 等待令牌补充（100 req/s = 10ms/token，等 20ms 足够补充 1+ 个）
	time.Sleep(25 * time.Millisecond)
	if !l.Allow("user1") {
		t.Fatal("expected allow after refill")
	}
}

func TestLimiter_PerPrincipalIsolation(t *testing.T) {
	l := New(100, 2)
	// user1 耗尽
	l.Allow("user1")
	l.Allow("user1")
	// user2 不受影响
	if !l.Allow("user2") {
		t.Fatal("user2 should not be affected by user1's usage")
	}
}

func TestLimiter_SetConfigChangesLimits(t *testing.T) {
	l := New(100, 1)
	l.Allow("user1") // 耗尽 burst 1
	if l.Allow("user1") {
		t.Fatal("expected deny after burst 1")
	}
	// 调整为 burst 5
	l.SetConfig("user1", 100, 5)
	for i := 0; i < 4; i++ {
		if !l.Allow("user1") {
			t.Fatalf("expected allow after config increase, attempt %d", i+1)
		}
	}
}

func TestLimiter_Stats(t *testing.T) {
	l := New(100, 10)
	l.Allow("user1")
	l.Allow("user1")
	stats := l.Stats()
	if stats["user1"].Tokens > 10 || stats["user1"].Tokens < 0 {
		t.Fatalf("unexpected token count: %v", stats["user1"].Tokens)
	}
	if stats["user1"].Rate != 100 {
		t.Fatalf("expected rate 100, got %v", stats["user1"].Rate)
	}
}

func TestDefaultSingleton(t *testing.T) {
	if Default == nil {
		t.Fatal("Default limiter should be initialized")
	}
	// Default 应该允许初次请求
	if !CheckRateLimit("test-user") {
		t.Fatal("default limiter should allow first request")
	}
}

func TestSetBudgetRateLimit(t *testing.T) {
	SetBudgetRateLimit("budget-user", 5, 3)
	stats := Default.Stats()
	if v, ok := stats["budget-user"]; !ok {
		t.Fatal("budget-user should exist in stats after SetBudgetRateLimit")
	} else if v.Rate != 5 || v.Burst != 3 {
		t.Fatalf("expected rate=5 burst=3, got rate=%v burst=%v", v.Rate, v.Burst)
	}
}
