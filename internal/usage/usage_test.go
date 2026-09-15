package usage

import (
	"testing"

	"github.com/CloudSilk/pkg/db"
	"github.com/CloudSilk/usercenter/internal/store"
	glebsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestMain(m *testing.M) {
	gdb, err := gorm.Open(glebsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	if err := gdb.AutoMigrate(&UsageRecord{}, &UsageBudget{}); err != nil {
		panic(err)
	}
	store.SetDB(db.NewDBClient(gdb, false))
	m.Run()
}

func TestRecordUsageBasic(t *testing.T) {
	rec := &UsageRecord{
		PrincipalID: "u1", TenantID: "t1", ProviderName: "openai",
		ModelName: "gpt-4", PromptTokens: 100, CompTokens: 50, CacheTokens: 0,
		Success: true,
	}
	RecordUsage(rec)
	if rec.TotalTokens != 150 {
		t.Fatalf("TotalTokens = %d, want 150", rec.TotalTokens)
	}
	var got UsageRecord
	if err := store.DB().First(&got, "principal_id = ?", "u1").Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.PromptTokens != 100 || got.CompTokens != 50 {
		t.Fatalf("unexpected record: %+v", got)
	}
}

func TestRecordUsageWithCache(t *testing.T) {
	rec := &UsageRecord{
		PrincipalID: "u2", TenantID: "t1", ModelName: "gpt-4",
		PromptTokens: 100, CompTokens: 50, CacheTokens: 200,
	}
	RecordUsage(rec)
	if rec.TotalTokens != 350 {
		t.Fatalf("TotalTokens should include cache, got %d", rec.TotalTokens)
	}
}

func TestGetUsageSummary(t *testing.T) {
	t.Cleanup(func() {
		_ = store.DB().Where("tenant_id = ?", "t-sum").Delete(&UsageRecord{}).Error
	})
	seed := []*UsageRecord{
		{TenantID: "t-sum", PrincipalID: "p1", ModelName: "gpt-4", PromptTokens: 100, CompTokens: 50, TotalTokens: 150, Cost: 0.01, Success: true},
		{TenantID: "t-sum", PrincipalID: "p1", ModelName: "gpt-4", PromptTokens: 200, CompTokens: 100, TotalTokens: 300, Cost: 0.02, Success: true},
		{TenantID: "t-sum", PrincipalID: "p2", ModelName: "gpt-3.5", PromptTokens: 50, CompTokens: 25, TotalTokens: 75, Cost: 0.001, Success: false, ErrorCode: "429"},
	}
	for _, r := range seed {
		RecordUsage(r)
	}

	summary, err := GetUsageSummary("t-sum", "", 0, 0)
	if err != nil {
		t.Fatalf("GetUsageSummary: %v", err)
	}
	if summary.TotalTokens != 525 {
		t.Fatalf("TotalTokens = %d, want 525", summary.TotalTokens)
	}
	if summary.RequestCount != 3 {
		t.Fatalf("RequestCount = %d, want 3", summary.RequestCount)
	}
	if summary.SuccessCount != 2 {
		t.Fatalf("SuccessCount = %d, want 2", summary.SuccessCount)
	}
}

func TestGetUsageByModel(t *testing.T) {
	t.Cleanup(func() {
		_ = store.DB().Where("tenant_id = ?", "t-by-model").Delete(&UsageRecord{}).Error
	})
	seed := []*UsageRecord{
		{TenantID: "t-by-model", ModelName: "gpt-4", PromptTokens: 100, CompTokens: 50, TotalTokens: 150, Cost: 0.01, Success: true},
		{TenantID: "t-by-model", ModelName: "gpt-4", PromptTokens: 200, CompTokens: 100, TotalTokens: 300, Cost: 0.02, Success: true},
		{TenantID: "t-by-model", ModelName: "gpt-3.5", PromptTokens: 50, CompTokens: 25, TotalTokens: 75, Cost: 0.001, Success: false},
	}
	for _, r := range seed {
		RecordUsage(r)
	}

	byModel, err := GetUsageByModel("t-by-model", 0, 0)
	if err != nil {
		t.Fatalf("GetUsageByModel: %v", err)
	}
	gpt4, ok := byModel["gpt-4"]
	if !ok {
		t.Fatal("gpt-4 not found")
	}
	if gpt4.RequestCount != 2 {
		t.Fatalf("gpt-4 RequestCount = %d, want 2", gpt4.RequestCount)
	}
}

func TestCheckBudgetByTokens(t *testing.T) {
	t.Cleanup(func() {
		_ = store.DB().Where("tenant_id LIKE ?", "budget-tok-%").Delete(&UsageRecord{}).Error
		_ = store.DB().Where("tenant_id LIKE ?", "budget-tok-%").Delete(&UsageBudget{}).Error
	})

	budget := &UsageBudget{
		TenantID: "budget-tok-t", PrincipalID: "budget-p",
		DailyTokenLimit: 500, Enable: true,
	}
	if err := store.DB().Create(budget).Error; err != nil {
		t.Fatalf("create budget: %v", err)
	}
	rec := &UsageRecord{TenantID: budget.TenantID, PrincipalID: budget.PrincipalID,
		ModelName: "m", PromptTokens: 400, CompTokens: 100, TotalTokens: 500, Success: true}
	RecordUsage(rec)

	allowed, daily, monthly, err := CheckBudget(budget.TenantID, budget.PrincipalID, "m")
	if err != nil {
		t.Fatalf("CheckBudget: %v", err)
	}
	if allowed {
		t.Fatalf("should be denied at daily limit, allowed=%v daily=%d monthly=%d", allowed, daily, monthly)
	}
	if daily < 500 {
		t.Fatalf("daily tokens should be >= 500, got %d", daily)
	}
}

func TestCheckBudgetByCost(t *testing.T) {
	t.Cleanup(func() {
		_ = store.DB().Where("tenant_id = ?", "budget-cost-%").Delete(&UsageRecord{}).Error
		_ = store.DB().Where("tenant_id LIKE ?", "budget-cost-%").Delete(&UsageBudget{}).Error
	})

	budget := &UsageBudget{
		TenantID: "budget-cost-t", PrincipalID: "cost-p",
		DailyCostLimit: 1.0, Enable: true,
	}
	if err := store.DB().Create(budget).Error; err != nil {
		t.Fatalf("create budget: %v", err)
	}
	rec := &UsageRecord{TenantID: budget.TenantID, PrincipalID: "cost-p",
		ModelName: "m", PromptTokens: 1000, CompTokens: 500, TotalTokens: 1500,
		Cost: 1.5, Success: true}
	RecordUsage(rec)

	allowed, _, _, err := CheckBudget(budget.TenantID, budget.PrincipalID, "m")
	if err != nil {
		t.Fatalf("CheckBudget: %v", err)
	}
	if allowed {
		t.Fatal("should be denied at daily cost limit")
	}
}

func TestCheckBudgetNoBudgetAllows(t *testing.T) {
	allowed, _, _, err := CheckBudget("no-budget-t", "no-budget-p", "m")
	if err != nil {
		t.Fatalf("CheckBudget: %v", err)
	}
	if !allowed {
		t.Fatal("no budget should allow")
	}
}
