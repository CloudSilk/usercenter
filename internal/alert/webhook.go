package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/CloudSilk/pkg/utils/log"
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
