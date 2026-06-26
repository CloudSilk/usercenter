package aicache

import (
	"crypto/sha256"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/internal/store"
)

// EmbeddingFunc 生成 prompt 的嵌入向量。网关注入实现（调用 /v1/embeddings）。
// 返回 nil 表示无法生成嵌入（此时回退到精确哈希匹配）。
type EmbeddingFunc func(prompt string) []float32

// CacheEntry represents a cached LLM response
type CacheEntry struct {
	commonmodel.Model
	PromptHash   string    `json:"promptHash" gorm:"uniqueIndex;size:64"`
	ResponseBody string    `json:"responseBody" gorm:"type:text"`
	ModelName    string    `json:"modelName" gorm:"size:100"`
	PromptTokens int64     `json:"promptTokens"`
	CompTokens   int64     `json:"compTokens"`
	Cost         float64   `json:"cost"`
	ExpiresAt    time.Time `json:"expiresAt" gorm:"index"`
	// Embedding 向量（内存中维护，不持久化到 DB）。nil 表示无嵌入（精确哈希模式）。
	embedding []float32 `gorm:"-"`
	Prompt    string    `json:"-" gorm:"-"`
}

func (CacheEntry) TableName() string { return "ai_cache_entries" }

// CacheConfig holds configuration for the semantic cache
type CacheConfig struct {
	Enabled             bool          `json:"enabled"`
	SimilarityThreshold float64       `json:"similarityThreshold"` // 0-1, default 0.95
	TTL                 time.Duration `json:"ttl"`
	MaxEntries          int           `json:"maxEntries"`
}

// Cache provides semantic caching for LLM responses.
// 若注入了 EmbeddingFunc，则按余弦相似度匹配（真正的语义缓存）；
// 否则回退到 SHA-256 精确哈希匹配。
type Cache struct {
	mu          sync.RWMutex
	entries     map[string]*CacheEntry // prompt_hash -> entry
	config      CacheConfig
	stats       CacheStats
	embeddingFn EmbeddingFunc
	embedWG     sync.WaitGroup // 跟踪进行中的异步嵌入计算，便于测试等待
}

// CacheStats holds cache performance statistics
type CacheStats struct {
	Hits            int64   `json:"hits"`
	Misses          int64   `json:"misses"`
	HitRate         float64 `json:"hitRate"`
	Entries         int     `json:"entries"`
	SemanticMatches int64   `json:"semanticMatches"` // 语义相似命中数
}

// New creates a new Cache instance
func New(config CacheConfig) *Cache {
	if config.SimilarityThreshold == 0 {
		config.SimilarityThreshold = 0.95
	}
	if config.TTL == 0 {
		config.TTL = 24 * time.Hour
	}
	if config.MaxEntries == 0 {
		config.MaxEntries = 10000
	}

	c := &Cache{
		entries: make(map[string]*CacheEntry),
		config:  config,
	}

	// Load existing entries from DB
	c.loadFromDB()

	return c
}

// SetEmbeddingFunc 注入嵌入生成函数，启用语义相似度匹配。
func (c *Cache) SetEmbeddingFunc(fn EmbeddingFunc) {
	c.mu.Lock()
	c.embeddingFn = fn
	c.mu.Unlock()
}

// Get retrieves a cached response for the given prompt.
// 优先尝试语义匹配（若有 embeddingFn），回退到精确哈希。
func (c *Cache) Get(prompt string, modelName string) (*CacheEntry, bool) {
	if !c.config.Enabled {
		return nil, false
	}

	hash := c.hashPrompt(prompt)

	c.mu.RLock()
	entry, exists := c.entries[hash]
	embeddingFn := c.embeddingFn
	c.mu.RUnlock()

	// 1. 精确哈希匹配（快路径）
	if exists && c.entryValid(entry, modelName) {
		c.mu.Lock()
		c.stats.Hits++
		c.mu.Unlock()
		return entry, true
	}

	// 2. 语义相似度匹配（若有 embeddingFn 且 prompt 较长值得匹配）
	if embeddingFn != nil && len(prompt) > 20 {
		if e, ok := c.semanticMatch(prompt, modelName); ok {
			c.mu.Lock()
			c.stats.Hits++
			c.stats.SemanticMatches++
			c.mu.Unlock()
			return e, true
		}
	}

	c.mu.Lock()
	c.stats.Misses++
	c.mu.Unlock()
	return nil, false
}

// semanticMatch 计算输入 prompt 的嵌入，与所有缓存条目比较余弦相似度。
func (c *Cache) semanticMatch(prompt string, modelName string) (*CacheEntry, bool) {
	queryVec := c.safeEmbed(prompt)
	if queryVec == nil {
		return nil, false
	}

	// 仅在持锁期间拷贝候选条目指针并做廉价过滤（embedding 是否存在 / 过期 / 模型匹配），
	// 立即释放读锁后再做 O(N) 余弦相似度计算，避免长时间阻塞写操作（Set/EnforceMaxEntries）。
	threshold := c.config.SimilarityThreshold
	now := time.Now()
	c.mu.RLock()
	candidates := make([]*CacheEntry, 0, len(c.entries))
	for _, entry := range c.entries {
		if entry.embedding == nil {
			continue
		}
		if now.After(entry.ExpiresAt) {
			continue
		}
		if entry.ModelName != "" && modelName != "" && entry.ModelName != modelName {
			continue
		}
		candidates = append(candidates, entry)
	}
	c.mu.RUnlock()

	var best *CacheEntry
	bestSim := threshold
	for _, entry := range candidates {
		sim := cosineSimilarity(queryVec, entry.embedding)
		if sim >= bestSim {
			bestSim = sim
			best = entry
		}
	}
	return best, best != nil
}

// Set stores a response in the cache
func (c *Cache) Set(prompt, response, modelName string, promptTokens, compTokens int64, cost float64) {
	if !c.config.Enabled {
		return
	}

	hash := c.hashPrompt(prompt)
	now := time.Now()

	entry := &CacheEntry{
		PromptHash:   hash,
		ResponseBody: response,
		ModelName:    modelName,
		PromptTokens: promptTokens,
		CompTokens:   compTokens,
		Cost:         cost,
		ExpiresAt:    now.Add(c.config.TTL),
		Prompt:       prompt,
	}

	// 若有 embeddingFn，异步为该 prompt 计算嵌入（避免阻塞当前请求）。
	// 嵌入在后台算好后写回条目：写时持锁，semanticMatch 也在锁内判定 embedding==nil，
	// 因此 nil→set 的转换由互斥量同步，转换后字段不再变化，无数据竞争。
	c.mu.RLock()
	fn := c.embeddingFn
	c.mu.RUnlock()

	c.mu.Lock()
	c.entries[hash] = entry
	c.stats.Entries = len(c.entries)
	c.mu.Unlock()

	// Persist to DB asynchronously（不持久化 embedding/Prompt 字段）
	go c.persistEntry(entry)

	// 异步计算嵌入并写回（仅当条目仍是当前条目时，避免写入被淘汰/替换的条目）
	if fn != nil && len(prompt) > 20 {
		c.embedWG.Add(1)
		go func() {
			defer c.embedWG.Done()
			defer func() { _ = recover() }()
			emb := fn(prompt)
			if emb == nil {
				return
			}
			c.mu.Lock()
			if cur, ok := c.entries[hash]; ok && cur == entry {
				entry.embedding = emb
			}
			c.mu.Unlock()
		}()
	}

	// 淘汰超额条目
	c.EnforceMaxEntries()
}

// Clear removes all cached entries
func (c *Cache) Clear() {
	c.mu.Lock()
	c.entries = make(map[string]*CacheEntry)
	c.stats = CacheStats{}
	c.mu.Unlock()

	if store.DB() != nil {
		store.DB().Exec("DELETE FROM ai_cache_entries")
	}
}

// CleanupExpired removes entries whose TTL has elapsed, from both memory and DB.
// 用于回收 loadFromDB 不会重载、但 EnforceMaxEntries（按数量淘汰）也不会删除的过期 DB 行。
func (c *Cache) CleanupExpired() int {
	now := time.Now()
	c.mu.Lock()
	var expired []string
	for hash, e := range c.entries {
		if now.After(e.ExpiresAt) {
			delete(c.entries, hash)
			expired = append(expired, hash)
		}
	}
	c.stats.Entries = len(c.entries)
	c.mu.Unlock()

	if len(expired) > 0 && store.DB() != nil {
		store.DB().Where("prompt_hash IN ?", expired).Delete(&CacheEntry{})
	}
	return len(expired)
}

// Stats returns current cache statistics
func (c *Cache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := c.stats
	if stats.Hits+stats.Misses > 0 {
		stats.HitRate = float64(stats.Hits) / float64(stats.Hits+stats.Misses) * 100
	}
	stats.Entries = len(c.entries)
	return stats
}

// hashPrompt creates a SHA-256 hash of the prompt
func (c *Cache) hashPrompt(prompt string) string {
	hash := sha256.Sum256([]byte(prompt))
	return fmt.Sprintf("%x", hash)
}

// safeEmbed 调用 embeddingFn，panic 恢复返回 nil
func (c *Cache) safeEmbed(prompt string) []float32 {
	c.mu.RLock()
	fn := c.embeddingFn
	c.mu.RUnlock()
	if fn == nil {
		return nil
	}
	defer func() { _ = recover() }()
	return fn(prompt)
}

// entryValid 检查条目是否未过期且模型匹配（读锁外调用需自行加锁）。
func (c *Cache) entryValid(entry *CacheEntry, modelName string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.entryValidLocked(entry, modelName)
}

func (c *Cache) entryValidLocked(entry *CacheEntry, modelName string) bool {
	if time.Now().After(entry.ExpiresAt) {
		return false
	}
	if entry.ModelName != "" && modelName != "" && entry.ModelName != modelName {
		return false
	}
	return true
}

// loadFromDB loads cache entries from the database
func (c *Cache) loadFromDB() {
	if store.DB() == nil {
		return
	}

	var entries []CacheEntry
	if err := store.DB().Where("expires_at > ?", time.Now()).Find(&entries).Error; err != nil {
		return
	}

	c.mu.Lock()
	for _, entry := range entries {
		e := entry // copy
		c.entries[entry.PromptHash] = &e
	}
	c.stats.Entries = len(c.entries)
	c.mu.Unlock()
}

// persistEntry saves a cache entry to the database
func (c *Cache) persistEntry(entry *CacheEntry) {
	if store.DB() == nil {
		return
	}

	existing := &CacheEntry{}
	if err := store.DB().Where("prompt_hash = ?", entry.PromptHash).First(existing).Error; err == nil {
		store.DB().Model(existing).Updates(map[string]interface{}{
			"response_body": entry.ResponseBody,
			"model_name":    entry.ModelName,
			"prompt_tokens": entry.PromptTokens,
			"comp_tokens":   entry.CompTokens,
			"cost":          entry.Cost,
			"expires_at":    entry.ExpiresAt,
		})
	} else {
		store.DB().Create(entry)
	}
}

// EnforceMaxEntries removes oldest entries if cache exceeds max size
func (c *Cache) EnforceMaxEntries() {
	c.mu.Lock()
	if len(c.entries) <= c.config.MaxEntries {
		c.mu.Unlock()
		return
	}

	type entryWithTime struct {
		hash string
		time time.Time
	}
	var all []entryWithTime
	for hash, entry := range c.entries {
		all = append(all, entryWithTime{hash: hash, time: entry.ExpiresAt})
	}
	// 按 ExpiresAt 升序（最早过期的先淘汰）
	sort.Slice(all, func(i, j int) bool {
		return all[i].time.Before(all[j].time)
	})

	excess := len(c.entries) - c.config.MaxEntries
	if excess > len(all) {
		excess = len(all)
	}
	evictedHashes := make([]string, 0, excess)
	for i := 0; i < excess; i++ {
		delete(c.entries, all[i].hash)
		evictedHashes = append(evictedHashes, all[i].hash)
	}
	c.stats.Entries = len(c.entries)
	c.mu.Unlock()

	// 同步清理 DB 中的孤儿行，否则 ai_cache_entries 表会随时间无限增长
	// （被淘汰的行不会被 loadFromDB 重新加载，但也永远不会被删除）。
	if len(evictedHashes) > 0 && store.DB() != nil {
		store.DB().Where("prompt_hash IN ?", evictedHashes).Delete(&CacheEntry{})
	}
}

// cosineSimilarity 计算两个向量的余弦相似度。
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// Package-level default cache instance
var Default *Cache

// Init initializes the default cache with given config
func Init(config CacheConfig) {
	Default = New(config)
}

// SetEmbeddingFunc sets the embedding function on the default cache.
func SetEmbeddingFunc(fn EmbeddingFunc) {
	if Default != nil {
		Default.SetEmbeddingFunc(fn)
	}
}

// Get retrieves from default cache
func Get(prompt, modelName string) (*CacheEntry, bool) {
	if Default == nil {
		return nil, false
	}
	return Default.Get(prompt, modelName)
}

// Set stores in default cache
func Set(prompt, response, modelName string, promptTokens, compTokens int64, cost float64) {
	if Default == nil {
		return
	}
	Default.Set(prompt, response, modelName, promptTokens, compTokens, cost)
}

// Clear clears default cache
func Clear() {
	if Default == nil {
		return
	}
	Default.Clear()
}

// Stats returns default cache stats
func Stats() CacheStats {
	if Default == nil {
		return CacheStats{}
	}
	return Default.Stats()
}

// CleanupExpired removes expired entries from the default cache (memory + DB).
func CleanupExpired() int {
	if Default == nil {
		return 0
	}
	return Default.CleanupExpired()
}

func init() {
	// 每小时清理一次过期缓存条目（内存 + DB），避免 ai_cache_entries 累积过期行。
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			CleanupExpired()
		}
	}()
}
