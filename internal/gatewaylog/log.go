package gatewaylog

import (
	"context"
	"time"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils/log"
	"github.com/CloudSilk/usercenter/internal/store"
)

// GatewayLog records every AI gateway request for full audit trail
type GatewayLog struct {
	commonmodel.Model
	TenantID        string  `json:"tenantID" gorm:"index;size:36"`
	PrincipalID     string  `json:"principalID" gorm:"index;size:36"`
	RequestID       string  `json:"requestID" gorm:"index;size:64"`
	Method          string  `json:"method" gorm:"size:50;index"` // /v1/chat/completions etc
	ModelAlias      string  `json:"modelAlias" gorm:"size:100"`
	Stream          bool    `json:"stream"`
	PromptTokens    int64   `json:"promptTokens"`
	CompTokens      int64   `json:"compTokens"`
	Cost            float64 `json:"cost"`
	LatencyMs       int64   `json:"latencyMs"`
	StatusCode      int     `json:"statusCode"`
	Success         bool    `json:"success"`
	ErrorMessage    string  `json:"errorMessage" gorm:"size:500"`
	Cached          bool    `json:"cached"`
	SessionID       string  `json:"sessionID" gorm:"size:36"`
	PromptHash      string  `json:"promptHash" gorm:"size:64;index"`
	RequestBody     string  `json:"requestBody" gorm:"type:text"`
	ResponsePreview string  `json:"responsePreview" gorm:"type:text"`
}

func (GatewayLog) TableName() string {
	return "gateway_logs"
}

// GatewayStats holds aggregated gateway statistics
type GatewayStats struct {
	TotalRequests   int64   `json:"totalRequests"`
	SuccessRequests int64   `json:"successRequests"`
	TotalTokens     int64   `json:"totalTokens"`
	TotalCost       float64 `json:"totalCost"`
	AvgLatencyMs    float64 `json:"avgLatencyMs"`
	CachedRequests  int64   `json:"cachedRequests"`
	CacheHitRate    float64 `json:"cacheHitRate"`
}

// Record saves a gateway log entry to the database.
// Errors are logged but not propagated to the caller.
func Record(logEntry *GatewayLog) {
	if logEntry.CreatedAt.IsZero() {
		logEntry.CreatedAt = time.Now()
	}
	if logEntry.UpdatedAt.IsZero() {
		logEntry.UpdatedAt = time.Now()
	}
	if err := store.DB().Create(logEntry).Error; err != nil {
		log.Errorf(context.Background(), "[GatewayLog] failed to record gateway log: %v", err)
	}
}

// Query returns a paginated list of gateway logs for a given tenant,
// filtered by time range. Returns the records, total count, and any error.
func Query(tenantID string, startTime, endTime int64, limit, offset int) ([]GatewayLog, int64, error) {
	var logs []GatewayLog
	var total int64

	db := store.DB().Model(&GatewayLog{})

	if tenantID != "" {
		db = db.Where("tenant_id = ?", tenantID)
	}
	if startTime > 0 {
		db = db.Where("created_at >= ?", time.Unix(startTime, 0))
	}
	if endTime > 0 {
		db = db.Where("created_at <= ?", time.Unix(endTime, 0))
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := db.Order("created_at DESC").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// GetByID retrieves a single gateway log record by its primary key.
func GetByID(id string) (*GatewayLog, error) {
	var logEntry GatewayLog
	if err := store.DB().Where("id = ?", id).First(&logEntry).Error; err != nil {
		return nil, err
	}
	return &logEntry, nil
}

// GetStats returns aggregated statistics for a given tenant and time range.
func GetStats(tenantID string, startTime, endTime int64) (*GatewayStats, error) {
	var stats GatewayStats

	db := store.DB().Model(&GatewayLog{})

	if tenantID != "" {
		db = db.Where("tenant_id = ?", tenantID)
	}
	if startTime > 0 {
		db = db.Where("created_at >= ?", time.Unix(startTime, 0))
	}
	if endTime > 0 {
		db = db.Where("created_at <= ?", time.Unix(endTime, 0))
	}

	stats.TotalRequests = db.Model(&GatewayLog{}).Where("tenant_id = ? OR tenant_id = ''", tenantID).Count(&stats.TotalRequests).RowsAffected
	// Use a fresh query for each aggregate to avoid scope leakage.
	base := store.DB().Model(&GatewayLog{})
	if tenantID != "" {
		base = base.Where("tenant_id = ?", tenantID)
	}
	if startTime > 0 {
		base = base.Where("created_at >= ?", time.Unix(startTime, 0))
	}
	if endTime > 0 {
		base = base.Where("created_at <= ?", time.Unix(endTime, 0))
	}

	// Total count
	if err := base.Count(&stats.TotalRequests).Error; err != nil {
		return nil, err
	}

	// Success count
	if err := base.Where("success = ?", true).Count(&stats.SuccessRequests).Error; err != nil {
		return nil, err
	}

	// Token sum
	if err := base.Select("COALESCE(SUM(prompt_tokens + comp_tokens), 0)").Scan(&stats.TotalTokens).Error; err != nil {
		return nil, err
	}

	// Cost sum
	if err := base.Select("COALESCE(SUM(cost), 0)").Scan(&stats.TotalCost).Error; err != nil {
		return nil, err
	}

	// Average latency
	if err := base.Select("COALESCE(AVG(latency_ms), 0)").Scan(&stats.AvgLatencyMs).Error; err != nil {
		return nil, err
	}

	// Cached count
	if err := base.Where("cached = ?", true).Count(&stats.CachedRequests).Error; err != nil {
		return nil, err
	}

	// Cache hit rate
	if stats.TotalRequests > 0 {
		stats.CacheHitRate = float64(stats.CachedRequests) / float64(stats.TotalRequests) * 100
	}

	return &stats, nil
}
