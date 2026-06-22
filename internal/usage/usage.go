package usage

import (
	"context"
	"time"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils/log"
	"github.com/CloudSilk/usercenter/internal/store"
	"gorm.io/gorm"
)

// UsageRecord 一次 LLM 调用的计量记录
type UsageRecord struct {
	commonmodel.Model
	PrincipalID   string  `json:"principalID" gorm:"index;size:36;comment:主体ID"`
	PrincipalKind int32   `json:"principalKind" gorm:"index;comment:0人1Agent2Service"`
	TenantID      string  `json:"tenantID" gorm:"index;size:36"`
	ProviderID    string  `json:"providerID" gorm:"index;size:36"`
	ProviderName  string  `json:"providerName" gorm:"size:100"`
	ModelName     string  `json:"model" gorm:"size:100;index;comment:模型名称"`
	PromptTokens  int64   `json:"promptTokens"`
	CompTokens    int64   `json:"completionTokens"`
	CacheTokens   int64   `json:"cacheTokens"`
	TotalTokens   int64   `json:"totalTokens"`
	Cost          float64 `json:"cost" gorm:"comment:费用(USD)"`
	RequestID     string  `json:"requestID" gorm:"index;size:64"`
	Success       bool    `json:"success" gorm:"index"`
	ErrorCode     string  `json:"errorCode" gorm:"size:50"`
	LatencyMs     int64   `json:"latencyMs"`
}

func (UsageRecord) TableName() string { return "usage_record" }

// UsageSummary 聚合查询结果
type UsageSummary struct {
	TenantID     string  `json:"tenantID"`
	TotalTokens  int64   `json:"totalTokens"`
	TotalCost    float64 `json:"totalCost"`
	RequestCount int64   `json:"requestCount"`
	SuccessCount int64   `json:"successCount"`
}

// UsageBudget 预算配额
type UsageBudget struct {
	commonmodel.Model
	TenantID           string  `json:"tenantID" gorm:"index;size:36"`
	PrincipalID        string  `json:"principalID" gorm:"index;size:36;comment:主体ID(空=租户级)"`
	ModelName          string  `json:"model" gorm:"size:100;comment:模型(空=全部)"`
	DailyTokenLimit    int64   `json:"dailyTokenLimit"`
	DailyCostLimit     float64 `json:"dailyCostLimit"`
	MonthlyTokenLimit  int64   `json:"monthlyTokenLimit"`
	MonthlyCostLimit   float64 `json:"monthlyCostLimit"`
	Enable             bool    `json:"enable" gorm:"index;default:true"`
}

func (UsageBudget) TableName() string { return "usage_budget" }

// RecordUsage 记录一次 LLM 调用的用量
func RecordUsage(rec *UsageRecord) {
	if store.DB() == nil {
		return
	}
	rec.TotalTokens = rec.PromptTokens + rec.CompTokens + rec.CacheTokens
	if err := store.DB().Create(rec).Error; err != nil {
		log.Errorf(context.Background(), "record usage failed: %v", err)
	}
}

// GetUsageSummary 按维度聚合用量
func GetUsageSummary(tenantID, principalID string, startTime, endTime int64) (*UsageSummary, error) {
	db := store.DB().Model(&UsageRecord{})
	if tenantID != "" {
		db = db.Where("tenant_id = ?", tenantID)
	}
	if principalID != "" {
		db = db.Where("principal_id = ?", principalID)
	}
	if startTime > 0 {
		db = db.Where("created_at >= ?", time.Unix(startTime, 0))
	}
	if endTime > 0 {
		db = db.Where("created_at <= ?", time.Unix(endTime, 0))
	}
	var s UsageSummary
	s.TenantID = tenantID
	err := db.Select(`
		COALESCE(SUM(total_tokens), 0) as total_tokens,
		COALESCE(SUM(cost), 0) as total_cost,
		COUNT(*) as request_count,
		COUNT(CASE WHEN success THEN 1 END) as success_count
	`).Scan(&s).Error
	return &s, err
}

// GetUsageByModel 按模型维度聚合
func GetUsageByModel(tenantID string, startTime, endTime int64) (map[string]*UsageSummary, error) {
	db := store.DB().Model(&UsageRecord{})
	if tenantID != "" {
		db = db.Where("tenant_id = ?", tenantID)
	}
	if startTime > 0 {
		db = db.Where("created_at >= ?", time.Unix(startTime, 0))
	}
	if endTime > 0 {
		db = db.Where("created_at <= ?", time.Unix(endTime, 0))
	}
	type modelAgg struct {
		ModelName     string  `json:"model"`
		TotalTokens   int64   `json:"total_tokens"`
		TotalCost     float64 `json:"total_cost"`
		RequestCount  int64   `json:"request_count"`
		SuccessCount  int64   `json:"success_count"`
	}
	var results []modelAgg
	err := db.Select("model_name as model, SUM(total_tokens) as total_tokens, SUM(cost) as total_cost, COUNT(*) as request_count, COUNT(CASE WHEN success THEN 1 END) as success_count").
		Group("model_name").Order("total_tokens desc").Find(&results).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string]*UsageSummary)
	for _, r := range results {
		out[r.ModelName] = &UsageSummary{
			TotalTokens: r.TotalTokens, TotalCost: r.TotalCost,
			RequestCount: r.RequestCount, SuccessCount: r.SuccessCount,
		}
	}
	return out, nil
}

// GetUsageByPrincipal 按主体维度聚合(区分人/Agent)
func GetUsageByPrincipal(tenantID string, startTime, endTime int64) ([]map[string]interface{}, error) {
	db := store.DB().Model(&UsageRecord{})
	if tenantID != "" {
		db = db.Where("tenant_id = ?", tenantID)
	}
	if startTime > 0 {
		db = db.Where("created_at >= ?", time.Unix(startTime, 0))
	}
	if endTime > 0 {
		db = db.Where("created_at <= ?", time.Unix(endTime, 0))
	}
	var results []map[string]interface{}
	err := db.Select("principal_id, principal_kind, SUM(total_tokens) as tokens, SUM(cost) as cost, COUNT(*) as count").
		Group("principal_id, principal_kind").Order("tokens desc").Find(&results).Error
	return results, err
}

// CheckBudget 检查是否超出预算(返回是否允许)
func CheckBudget(tenantID, principalID, modelName string) (allowed bool, dailyTokens, monthlyTokens int64, err error) {
	budget := &UsageBudget{}
	db := store.DB().Where("enable = ? AND tenant_id = ?", true, tenantID)
	if principalID != "" {
		db = db.Where("principal_id IN (?, '')", principalID)
	}
	if modelName != "" {
		db = db.Where("model IN (?, '')", modelName)
	}
	if e := db.First(budget).Error; e != nil {
		return true, 0, 0, nil // 无预算 = 不限制
	}

	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	usageDB := store.DB().Model(&UsageRecord{}).Where("tenant_id = ?", tenantID)
	if principalID != "" {
		usageDB = usageDB.Where("principal_id = ?", principalID)
	}
	usageDB.Where("created_at >= ?", dayStart).Count(&dailyTokens)
	usageDB.Where("created_at >= ?", monthStart).Count(&monthlyTokens)

	if budget.DailyTokenLimit > 0 && dailyTokens >= budget.DailyTokenLimit {
		return false, dailyTokens, monthlyTokens, nil
	}
	if budget.MonthlyTokenLimit > 0 && monthlyTokens >= budget.MonthlyTokenLimit {
		return false, dailyTokens, monthlyTokens, nil
	}
	return true, dailyTokens, monthlyTokens, nil
}

// CRUD for budgets
func CreateBudget(b *UsageBudget) (string, error) {
	err := store.DB().Create(b).Error
	return b.ID, err
}

func UpdateBudget(b *UsageBudget) error {
	return store.DB().Omit("created_at").Save(b).Error
}

func DeleteBudget(id string) error {
	return store.DB().Delete(&UsageBudget{}, "id=?", id).Error
}

var _ = gorm.ErrRecordNotFound // suppress unused
