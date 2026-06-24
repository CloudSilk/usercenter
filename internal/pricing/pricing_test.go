package pricing

import "testing"

func TestCalculateCost(t *testing.T) {
	price := &ModelPrice{
		ModelName:   "gpt-4",
		InputPer1M:  30.0, // $30 / 1M input tokens
		OutputPer1M: 60.0, // $60 / 1M output tokens
	}

	// 1000 prompt tokens + 500 completion tokens
	// cost = 1000/1e6*30 + 500/1e6*60 = 0.03 + 0.03 = 0.06
	cost := CalculateCost(price, 1000, 500)
	if cost < 0.059 || cost > 0.061 {
		t.Fatalf("expected cost ~0.06, got %f", cost)
	}
}

func TestCalculateCost_ZeroTokens(t *testing.T) {
	price := &ModelPrice{InputPer1M: 10, OutputPer1M: 20}
	cost := CalculateCost(price, 0, 0)
	if cost != 0 {
		t.Fatalf("expected 0 cost for 0 tokens, got %f", cost)
	}
}

func TestCalculateCost_LargeUsage(t *testing.T) {
	price := &ModelPrice{InputPer1M: 5, OutputPer1M: 15}
	// 1M input + 1M output = 5 + 15 = 20
	cost := CalculateCost(price, 1000000, 1000000)
	if cost < 19.99 || cost > 20.01 {
		t.Fatalf("expected cost ~20.0, got %f", cost)
	}
}

func TestGetPrice_TenantOverridePrecedence(t *testing.T) {
	// 这个测试验证 GetPrice 的租户优先逻辑需要 DB，这里只验证函数存在且签名正确
	// 完整测试需要 store.DB()
	_ = GetPrice
}
