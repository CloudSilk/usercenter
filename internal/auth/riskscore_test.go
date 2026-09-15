package auth

import (
	"strings"
	"testing"
	"time"
)

func riskSig() *RiskSignals {
	return &RiskSignals{
		Principal:     nil, // 跳过异地检测(依赖 session 存储)
		TokenIssuedIP: "",
		CurrentIP:     "",
		TokenAge:      time.Hour,
	}
}

func TestCalculateRiskScore_NoSignalsAllows(t *testing.T) {
	score := CalculateRiskScore(riskSig())
	if score.Action != "allow" || score.Level != RiskNone || len(score.Reasons) != 0 {
		t.Fatalf("unexpected clean score: %+v", score)
	}
}

func TestCalculateRiskScore_ImpossibleTravelDenies(t *testing.T) {
	sig := riskSig()
	sig.TokenIssuedIP = "1.1.1.1"
	sig.CurrentIP = "2.2.2.2"
	sig.TokenAge = 10 * time.Minute // 短时间内 IP 变化

	score := CalculateRiskScore(sig)
	if score.Action != "deny" {
		t.Fatalf("impossible travel should deny, got %+v", score)
	}
	if len(score.Reasons) == 0 || !strings.Contains(score.Reasons[0], "IP 漂移") {
		t.Fatalf("expected IP drift reason: %+v", score.Reasons)
	}
}

func TestCalculateRiskScore_IpChangeAfterLongTimeIsLow(t *testing.T) {
	sig := riskSig()
	sig.TokenIssuedIP = "1.1.1.1"
	sig.CurrentIP = "2.2.2.2"
	sig.TokenAge = 48 * time.Hour // 漫游场景

	score := CalculateRiskScore(sig)
	// 漫游(Low)+token 过老(Low)累加为 20,未达 Medium(30),放行
	if score.Action != "allow" || score.Level != RiskLow*2 {
		t.Fatalf("long-lived token IP change should be low/allow: %+v", score)
	}
}

func TestCalculateRiskScore_NewDeviceStepsUp(t *testing.T) {
	sig := riskSig()
	sig.NewDevice = true
	score := CalculateRiskScore(sig)
	if score.Action != "step_up" || score.Level != RiskMedium {
		t.Fatalf("new device should step_up: %+v", score)
	}
}

func TestCalculateRiskScore_FailedAuthsStepsUp(t *testing.T) {
	sig := riskSig()
	sig.FailedAuths = 5
	score := CalculateRiskScore(sig)
	if score.Action != "step_up" {
		t.Fatalf("failed auths should step_up: %+v", score)
	}
}

func TestCalculateRiskScore_OldTokenIsLow(t *testing.T) {
	sig := riskSig()
	sig.TokenAge = 48 * time.Hour
	score := CalculateRiskScore(sig)
	// 单一 token 过老信号 = 一个 Low,放行
	if score.Action != "allow" || score.Level != RiskLow {
		t.Fatalf("old token should be low/allow: %+v", score)
	}
}

func TestCalculateRiskScore_CombinedSignalsDeny(t *testing.T) {
	sig := riskSig()
	sig.TokenIssuedIP = "1.1.1.1"
	sig.CurrentIP = "2.2.2.2"
	sig.TokenAge = 10 * time.Minute
	sig.NewDevice = true // 漂移(High)+新设备(Medium)
	score := CalculateRiskScore(sig)
	if score.Action != "deny" {
		t.Fatalf("combined high signals should deny: %+v", score)
	}
	if len(score.Reasons) < 2 {
		t.Fatalf("expected both reasons recorded: %+v", score.Reasons)
	}
}

func TestIsSensitiveAction(t *testing.T) {
	sensitive := []string{
		"/api/core/auth/user/resetpwd",
		"/api/core/auth/user/changepwd",
		"/api/core/user/delete",
		"/api/core/user/export",
		"/oauth/authorize",
		"/oauth/token",
	}
	for _, p := range sensitive {
		if !IsSensitiveAction(p) {
			t.Fatalf("%q should be sensitive", p)
		}
	}
	normal := []string{"/api/core/user/detail", "/api/wechat/pay/order", "/"}
	for _, p := range normal {
		if IsSensitiveAction(p) {
			t.Fatalf("%q should not be sensitive", p)
		}
	}
}
