package alert

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/CloudSilk/pkg/utils/log"
	"github.com/CloudSilk/usercenter/internal/webhook"
)

// Webhook 告警（REDESIGN Wave3 可观测性）。
//
// 配置一个 Webhook URL（SetWebhookURL 或 systemconfig alert.webhook.url）后，
// 关键事件（超额、暴力破解等）会以 JSON POST 推送到该 URL，供 Slack/钉钉/飞书/自建
// 告警平台消费。推送异步、失败仅记日志，绝不阻塞业务。

var (
	webhookMu   sync.RWMutex
	webhookURL  string
	httpClient  = &http.Client{Timeout: 5 * time.Second}
)

// SetWebhookURL 设置告警 Webhook URL（空则禁用推送）。
func SetWebhookURL(url string) {
	webhookMu.Lock()
	webhookURL = url
	webhookMu.Unlock()
}

// WebhookURL 返回当前配置的 Webhook URL。
func WebhookURL() string {
	webhookMu.RLock()
	defer webhookMu.RUnlock()
	return webhookURL
}

// FireWebhook 异步推送告警事件。payload 为任意可序列化结构。
func FireWebhook(eventType string, payload map[string]interface{}) {
	webhookMu.RLock()
	url := webhookURL
	webhookMu.RUnlock()
	if url == "" {
		return
	}
	body := map[string]interface{}{
		"event": eventType, "ts": time.Now().Unix(), "payload": payload,
	}
	go func() {
		b, err := json.Marshal(body)
		if err != nil {
			return
		}
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := httpClient.Do(req)
		if err != nil {
			log.Errorf(context.Background(), "alert webhook post failed: %v", err)
			return
		}
		resp.Body.Close()
	}()
}

// FireEvent 按订阅模型推送事件。查询匹配的已启用订阅，向每个目标的 URL
// 异步 POST JSON 负载，携带 HMAC-SHA256 签名（X-Signature-256 头）。
// 与 FireWebhook（单 URL 告警通道）并存：前者面向业务事件订阅，后者保留向后兼容。
func FireEvent(eventType string, payload map[string]interface{}) {
	body := map[string]interface{}{
		"event": eventType, "ts": time.Now().Unix(), "payload": payload,
	}
	subs := webhook.GetEnabledSubsForEvent(eventType)
	if len(subs) == 0 {
		return
	}
	go func() {
		b, err := json.Marshal(body)
		if err != nil {
			return
		}
		for _, sub := range subs {
			req, err := http.NewRequest(http.MethodPost, sub.URL, bytes.NewReader(b))
			if err != nil {
				log.Errorf(context.Background(), "webhook sub %s: new request failed: %v", sub.ID, err)
				continue
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Event-Type", eventType)

			// HMAC-SHA256 签名（仅当订阅配置了 Secret 时）
			if sub.Secret != "" {
				mac := hmac.New(sha256.New, []byte(sub.Secret))
				mac.Write(b)
				sig := hex.EncodeToString(mac.Sum(nil))
				req.Header.Set("X-Signature-256", sig)
			}

			resp, err := httpClient.Do(req)
			if err != nil {
				log.Errorf(context.Background(), "webhook sub %s (%s): post failed: %v", sub.ID, sub.URL, err)
				continue
			}
			resp.Body.Close()
		}
	}()
}
