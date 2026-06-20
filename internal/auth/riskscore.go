package auth

// Zero Trust 连续验证 — 风险评分引擎(REDESIGN #15)

import (
	"strings"
	"time"

	"github.com/CloudSilk/usercenter/internal/principal"
	"github.com/CloudSilk/usercenter/internal/session"
)

// RiskLevel 风险等级
type RiskLevel int32

const (
	RiskNone   RiskLevel = 0  // 无风险
	RiskLow    RiskLevel = 10 // 低风险(允许,记录)
	RiskMedium RiskLevel = 30 // 中风险(要求 step-up)
	RiskHigh   RiskLevel = 60 // 高风险(拒绝或强制 MFA)
)

// RiskScore 风险评分结果
type RiskScore struct {
	Level    RiskLevel
	Reasons  []string
	Action   string // "allow" / "step_up" / "deny"
}

// RiskSignals 风险信号(从请求上下文收集)
type RiskSignals struct {
	Principal    principal.Principal
	CurrentIP    string
	TokenIssuedIP string
	UserAgent    string
	TokenAge     time.Duration
	NewDevice    bool
	FailedAuths  int // 近期失败次数
}

// CalculateRiskScore 基于实时信号计算风险分
func CalculateRiskScore(sig *RiskSignals) *RiskScore {
	score := &RiskScore{Level: RiskNone, Action: "allow"}

	// 信号1:IP 漂移(Impossible Travel)
	if sig.TokenIssuedIP != "" && sig.CurrentIP != "" && sig.TokenIssuedIP != sig.CurrentIP {
		if sig.TokenAge < 2*time.Hour {
			score.Level += RiskHigh
			score.Reasons = append(score.Reasons, "IP 漂移(token 签发后短时间内 IP 变化)")
		} else {
			score.Level += RiskLow
			score.Reasons = append(score.Reasons, "IP 变化(可能正常漫游)")
		}
	}

	// 信号2:新设备
	if sig.NewDevice {
		score.Level += RiskMedium
		score.Reasons = append(score.Reasons, "新设备首次访问")
	}

	// 信号3:近期失败次数
	if sig.FailedAuths >= 3 {
		score.Level += RiskMedium
		score.Reasons = append(score.Reasons, "近期多次认证失败")
	}

	// 信号4:token 年龄过大(超 24h 未刷新)
	if sig.TokenAge > 24*time.Hour {
		score.Level += RiskLow
		score.Reasons = append(score.Reasons, "token 过老(>24h)")
	}

	// 信号5:异地登录检测
	if sig.Principal != nil && sig.CurrentIP != "" {
		if anomaly, _ := session.DetectAnomaly(sig.Principal.Subject(), sig.CurrentIP, 60); anomaly {
			score.Level += RiskHigh
			score.Reasons = append(score.Reasons, "异地登录检测触发")
		}
	}

	// 决策
	switch {
	case score.Level >= RiskHigh:
		score.Action = "deny"
	case score.Level >= RiskMedium:
		score.Action = "step_up"
	default:
		score.Action = "allow"
	}

	return score
}

// IsSensitiveAction 判断是否为需要 step-up 的敏感操作
func IsSensitiveAction(path string) bool {
	sensitive := []string{
		"/resetpwd", "/changepwd", "/delete",
		"/enable", "/export", "/copy",
		"/oauth/authorize", "/oauth/token",
	}
	for _, s := range sensitive {
		if strings.Contains(path, s) {
			return true
		}
	}
	return false
}
