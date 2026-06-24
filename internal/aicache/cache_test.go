package aicache

import (
	"testing"
	"time"
)

func newTestCache(t *testing.T) *Cache {
	t.Helper()
	// 不依赖 DB：直接构造内存实例，DB 操作在 nil store 下为 no-op
	c := &Cache{
		entries: make(map[string]*CacheEntry),
		config: CacheConfig{
			Enabled:             true,
			SimilarityThreshold: 0.95,
			TTL:                 1 * time.Hour,
			MaxEntries:          100,
		},
	}
	return c
}

func TestCache_SetAndGet(t *testing.T) {
	c := newTestCache(t)
	c.Set("hello world", `{"choices":[]}`, "gpt-4", 10, 20, 0.001)

	entry, ok := c.Get("hello world", "gpt-4")
	if !ok {
		t.Fatal("expected cache hit after Set")
	}
	if entry.ResponseBody != `{"choices":[]}` {
		t.Fatalf("unexpected response body: %s", entry.ResponseBody)
	}
	if entry.PromptTokens != 10 || entry.CompTokens != 20 {
		t.Fatalf("unexpected tokens: prompt=%d comp=%d", entry.PromptTokens, entry.CompTokens)
	}
}

func TestCache_MissOnUnknownPrompt(t *testing.T) {
	c := newTestCache(t)
	c.Set("prompt A", "resp A", "gpt-4", 5, 5, 0)

	if _, ok := c.Get("prompt B", "gpt-4"); ok {
		t.Fatal("expected miss for unknown prompt")
	}
}

func TestCache_DisabledNoOp(t *testing.T) {
	c := newTestCache(t)
	c.config.Enabled = false
	c.Set("x", "y", "gpt-4", 1, 1, 0)
	if _, ok := c.Get("x", "gpt-4"); ok {
		t.Fatal("disabled cache should not store or return entries")
	}
}

func TestCache_Expiration(t *testing.T) {
	c := newTestCache(t)
	c.Set("expiring", "resp", "gpt-4", 1, 1, 0)

	// 手动把过期时间设到过去
	hash := c.hashPrompt("expiring")
	c.mu.Lock()
	c.entries[hash].ExpiresAt = time.Now().Add(-1 * time.Minute)
	c.mu.Unlock()

	if _, ok := c.Get("expiring", "gpt-4"); ok {
		t.Fatal("expected miss for expired entry")
	}
}

func TestCache_ModelMismatchMiss(t *testing.T) {
	c := newTestCache(t)
	c.Set("prompt", "resp", "gpt-4", 1, 1, 0)

	// 不同模型名应 miss（除非 entry.ModelName 为空）
	if _, ok := c.Get("prompt", "claude-3"); ok {
		t.Fatal("expected miss for different model name")
	}
}

func TestCache_EmptyModelMatchesAny(t *testing.T) {
	c := newTestCache(t)
	// 存入时 model 为空 → 对任意 model 名都命中
	c.Set("prompt", "resp", "", 1, 1, 0)
	if _, ok := c.Get("prompt", "gpt-4"); !ok {
		t.Fatal("empty model entry should match any model query")
	}
	if _, ok := c.Get("prompt", "claude-3"); !ok {
		t.Fatal("empty model entry should match any model query")
	}
}

func TestCache_Clear(t *testing.T) {
	c := newTestCache(t)
	c.Set("a", "1", "gpt-4", 1, 1, 0)
	c.Set("b", "2", "gpt-4", 1, 1, 0)

	c.Clear()

	if _, ok := c.Get("a", "gpt-4"); ok {
		t.Fatal("expected miss after Clear")
	}
	if _, ok := c.Get("b", "gpt-4"); ok {
		t.Fatal("expected miss after Clear")
	}

	stats := c.Stats()
	if stats.Entries != 0 {
		t.Fatalf("expected 0 entries after Clear, got %d", stats.Entries)
	}
}

func TestCache_Stats(t *testing.T) {
	c := newTestCache(t)
	c.Set("hit-prompt", "resp", "gpt-4", 1, 1, 0)

	c.Get("hit-prompt", "gpt-4") // hit
	c.Get("hit-prompt", "gpt-4") // hit
	c.Get("miss-prompt", "gpt-4") // miss

	stats := c.Stats()
	if stats.Hits != 2 {
		t.Fatalf("expected 2 hits, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Fatalf("expected 1 miss, got %d", stats.Misses)
	}
	if stats.Entries != 1 {
		t.Fatalf("expected 1 entry, got %d", stats.Entries)
	}
	expectedRate := 2.0 / 3.0 * 100
	if stats.HitRate < expectedRate-1 || stats.HitRate > expectedRate+1 {
		t.Fatalf("expected hitRate ~%.1f, got %.1f", expectedRate, stats.HitRate)
	}
}

func TestCache_HashDeterministic(t *testing.T) {
	c := newTestCache(t)
	h1 := c.hashPrompt("test prompt")
	h2 := c.hashPrompt("test prompt")
	if h1 != h2 {
		t.Fatal("hash should be deterministic for same input")
	}
	h3 := c.hashPrompt("different prompt")
	if h1 == h3 {
		t.Fatal("hash should differ for different input")
	}
}

func TestCache_EnforceMaxEntries(t *testing.T) {
	c := newTestCache(t)
	c.config.MaxEntries = 3
	for i := 0; i < 5; i++ {
		c.Set("prompt-"+string(rune('a'+i)), "resp", "gpt-4", 1, 1, 0)
	}
	c.EnforceMaxEntries()
	if len(c.entries) > 3 {
		t.Fatalf("expected at most 3 entries after enforce, got %d", len(c.entries))
	}
}

func TestCache_InitCreatesDefault(t *testing.T) {
	Init(CacheConfig{
		Enabled: true,
		TTL:     30 * time.Minute,
	})
	if Default == nil {
		t.Fatal("Init should create Default cache")
	}
	if !Default.config.Enabled {
		t.Fatal("Default cache should be enabled")
	}
	if Default.config.TTL != 30*time.Minute {
		t.Fatalf("expected TTL 30m, got %v", Default.config.TTL)
	}
}
