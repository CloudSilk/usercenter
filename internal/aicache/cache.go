package aicache

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/internal/store"
)

// CacheEntry represents a cached LLM response
type CacheEntry struct {
	commonmodel.Model
	PromptHash   string  `json:"promptHash" gorm:"uniqueIndex;size:64"`
	ResponseBody string  `json:"responseBody" gorm:"type:text"`
	ModelName    string  `json:"modelName" gorm:"size:100"`
	PromptTokens int64   `json:"promptTokens"`
	CompTokens   int64   `json:"compTokens"`
	Cost         float64 `json:"cost"`
	ExpiresAt    time.Time `json:"expiresAt" gorm:"index"`
}

func (CacheEntry) TableName() string { return "ai_cache_entries" }

// CacheConfig holds configuration for the semantic cache
type CacheConfig struct {
	Enabled               bool    `json:"enabled"`
	SimilarityThreshold   float64 `json:"similarityThreshold"` // 0-1, default 0.95
	TTL                   time.Duration `json:"ttl"`
	MaxEntries            int     `json:"maxEntries"`
}

// Cache provides semantic caching for LLM responses
type Cache struct {
	mu      sync.RWMutex
	entries map[string]*CacheEntry // prompt_hash -> entry
	config  CacheConfig
	stats   CacheStats
}

// CacheStats holds cache performance statistics
type CacheStats struct {
	Hits      int64   `json:"hits"`
	Misses    int64   `json:"misses"`
	HitRate   float64 `json:"hitRate"`
	Entries   int     `json:"entries"`
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

// Get retrieves a cached response for the given prompt
func (c *Cache) Get(prompt string, modelName string) (*CacheEntry, bool) {
	if !c.config.Enabled {
		return nil, false
	}

	hash := c.hashPrompt(prompt)

	c.mu.RLock()
	entry, exists := c.entries[hash]
	c.mu.RUnlock()

	if !exists {
		c.mu.Lock()
		c.stats.Misses++
		c.mu.Unlock()
		return nil, false
	}

	// Check expiration
	if time.Now().After(entry.ExpiresAt) {
		c.mu.Lock()
		delete(c.entries, hash)
		c.stats.Misses++
		c.mu.Unlock()
		return nil, false
	}

	// Check model match (empty model matches any)
	if entry.ModelName != "" && modelName != "" && entry.ModelName != modelName {
		c.mu.Lock()
		c.stats.Misses++
		c.mu.Unlock()
		return nil, false
	}

	c.mu.Lock()
	c.stats.Hits++
	c.mu.Unlock()

	return entry, true
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
	}

	c.mu.Lock()
	c.entries[hash] = entry
	c.stats.Entries = len(c.entries)
	c.mu.Unlock()

	// Persist to DB asynchronously
	go c.persistEntry(entry)
}

// Clear removes all cached entries
func (c *Cache) Clear() {
	c.mu.Lock()
	c.entries = make(map[string]*CacheEntry)
	c.stats = CacheStats{}
	c.mu.Unlock()

	// Clear DB
	if store.DB() != nil {
		store.DB().Exec("DELETE FROM ai_cache_entries")
	}
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

	// Upsert by prompt_hash
	existing := &CacheEntry{}
	if err := store.DB().Where("prompt_hash = ?", entry.PromptHash).First(existing).Error; err == nil {
		// Update existing
		store.DB().Model(existing).Updates(map[string]interface{}{
			"response_body": entry.ResponseBody,
			"model_name":    entry.ModelName,
			"prompt_tokens": entry.PromptTokens,
			"comp_tokens":   entry.CompTokens,
			"cost":          entry.Cost,
			"expires_at":    entry.ExpiresAt,
		})
	} else {
		// Create new
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

	// Find and remove oldest entries
	type entryWithTime struct {
		hash string
		time time.Time
	}
	var toRemove []entryWithTime
	for hash, entry := range c.entries {
		toRemove = append(toRemove, entryWithTime{hash: hash, time: entry.ExpiresAt})
	}

	// Sort by expiration time (oldest first)
	for i := 0; i < len(toRemove)-1; i++ {
		for j := i + 1; j < len(toRemove); j++ {
			if toRemove[i].time.After(toRemove[j].time) {
				toRemove[i], toRemove[j] = toRemove[j], toRemove[i]
			}
		}
	}

	// Remove excess entries
	excess := len(c.entries) - c.config.MaxEntries
	for i := 0; i < excess && i < len(toRemove); i++ {
		delete(c.entries, toRemove[i].hash)
	}
	c.stats.Entries = len(c.entries)
	c.mu.Unlock()
}

// Package-level default cache instance
var Default *Cache

// Init initializes the default cache with given config
func Init(config CacheConfig) {
	Default = New(config)
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
