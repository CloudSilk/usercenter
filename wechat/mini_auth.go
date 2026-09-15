package wechat

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// miniAPIBase 小程序 code2session API 基址。
// 默认微信官方地址；测试或代理场景可通过环境变量 WECHAT_API_BASE 覆盖，
// 或调用 SetMiniAPIBase 显式设置（每次调用实时读取，便于测试注入）。
var miniAPIBase = "https://api.weixin.qq.com"

// SetMiniAPIBase 设置小程序登录 API 基址。
func SetMiniAPIBase(base string) {
	trimmed := strings.TrimRight(strings.TrimSpace(base), "/")
	if trimmed != "" {
		miniAPIBase = trimmed
	}
}

func getMiniAPIBase() string {
	if base := strings.TrimRight(strings.TrimSpace(os.Getenv("WECHAT_API_BASE")), "/"); base != "" {
		return base
	}
	return miniAPIBase
}

// code2SessionResult jscode2session 返回结构。
type code2SessionResult struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	UnionID    string `json:"unionid"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

// Code2SessionDirect 以 appid/secret 直接调用 jscode2session（可配置 API 基址）。
// 与 silenceper SDK 的 Code2Session 等价，但支持基址覆盖与自定义 HTTP 客户端。
func Code2SessionDirect(appID, secret, jsCode string) (*code2SessionResult, error) {
	target := fmt.Sprintf("%s/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
		getMiniAPIBase(), url.QueryEscape(appID), url.QueryEscape(secret), url.QueryEscape(jsCode))
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(target)
	if err != nil {
		return nil, fmt.Errorf("请求微信 code2session 失败: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var result code2SessionResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("code2session 响应解析失败: %w", err)
	}
	if result.ErrCode != 0 {
		return nil, fmt.Errorf("Code2Session error : errcode=%v , errmsg=%v", result.ErrCode, result.ErrMsg)
	}
	return &result, nil
}
