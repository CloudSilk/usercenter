package aicache

import (
	"strings"
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

// TestCache_SemanticMatchWithEmbeddings 验证注入 EmbeddingFunc 后的语义相似度匹配。
func TestCache_SemanticMatchWithEmbeddings(t *testing.T) {
	c := newTestCache(t)
	c.config.SimilarityThreshold = 0.9

	// 模拟嵌入函数：相似文本返回接近的向量
	// "今天天气怎么样" 和 "今天天气如何" 映射到近似向量
	embedFn := func(prompt string) []float32 {
		// 简单的确定性伪嵌入：基于关键词生成向量
		vec := make([]float32, 8)
		if strings.Contains(prompt, "天气") {
			vec[0] = 0.9
		}
		if strings.Contains(prompt, "今天") {
			vec[1] = 0.8
		}
		if strings.Contains(prompt, "怎么") || strings.Contains(prompt, "如何") {
			vec[2] = 0.7
		}
		return vec
	}
	c.SetEmbeddingFunc(embedFn)

	// 存入一个长 prompt（>20 字符才触发语义匹配）
	original := "请问今天天气怎么样，需要带伞吗，我想出门逛街"
	c.Set(original, `{"choices":[{"message":{"content":"晴，25度"}}]}`, "gpt-4", 10, 5, 0.001)

	// 用不同措辞但语义相近的 prompt 查询（向量高度相似）
	similar := "今天天气如何呢，要不要带伞，准备出门呢"
	entry, ok := c.Get(similar, "gpt-4")
	if !ok {
		t.Fatal("expected semantic match for similar prompt")
	}
	if !strings.Contains(entry.ResponseBody, "晴") {
		t.Fatalf("unexpected response from semantic match: %s", entry.ResponseBody)
	}
}

// TestCache_SemanticMissOnUnrelated 验证不相关的 prompt 不命中。
func TestCache_SemanticMissOnUnrelated(t *testing.T) {
	c := newTestCache(t)
	c.config.SimilarityThreshold = 0.9

	embedFn := func(prompt string) []float32 {
		vec := make([]float32, 4)
		if strings.Contains(prompt, "天气") {
			vec[0] = 1.0
		}
		if strings.Contains(prompt, "代码") {
			vec[1] = 1.0
		}
		return vec
	}
	c.SetEmbeddingFunc(embedFn)

	c.Set("请问今天天气怎么样需要带伞吗我想出门逛街", "天气回答", "gpt-4", 5, 5, 0)

	// 完全不相关的 prompt
	if _, ok := c.Get("帮我写一段快速排序的代码实现并解释原理", "gpt-4"); ok {
		t.Fatal("expected miss for unrelated prompt")
	}
}

// TestCosineSimilarity 验证余弦相似度计算。
func TestCosineSimilarity(t *testing.T) {
	// 相同向量应返回 1.0
	if s := cosineSimilarity([]float32{1, 0, 0}, []float32{1, 0, 0}); s < 0.99 {
		t.Fatalf("identical vectors should have similarity ~1.0, got %f", s)
	}
	// 正交向量应返回 0
	if s := cosineSimilarity([]float32{1, 0}, []float32{0, 1}); s > 0.01 {
		t.Fatalf("orthogonal vectors should have similarity ~0, got %f", s)
	}
	// 不同长度应返回 0
	if s := cosineSimilarity([]float32{1, 2, 3}, []float32{1, 2}); s != 0 {
		t.Fatalf("different-length vectors should return 0, got %f", s)
	}
	// 空向量应返回 0
	if s := cosineSimilarity([]float32{}, []float32{}); s != 0 {
		t.Fatalf("empty vectors should return 0, got %f", s)
	}
}

// TestSetEmbeddingFunc 验证注入嵌入函数。
func TestSetEmbeddingFunc(t *testing.T) {
	c := newTestCache(t)
	c.SetEmbeddingFunc(func(s string) []float32 { return []float32{1, 2, 3} })
	c.mu.RLock()
	fn := c.embeddingFn
	c.mu.RUnlock()
	if fn == nil {
		t.Fatal("embedding function should be set")
	}
}
