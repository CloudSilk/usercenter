package http

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/CloudSilk/usercenter/internal/apikey"
	"github.com/CloudSilk/usercenter/internal/usage"
	ucm "github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/gin-gonic/gin"
)

// AI 网关：OpenAI 兼容的流式代理。
//
// POST /v1/chat/completions
//
// 把客户端请求按 model 路由到已配置的服务商/Key（apikey.SelectKey，含主从池与故障转移），
// 透传上游响应（流式 SSE 或整包），按真实用量记录 usage、执行租户/Agent 配额，并在上游
// 429 时把 Key 打入冷却后自动重试下一个路由。这让 usercenter 成为 AI 应用的统一网关：
// 应用只认 usercenter 一个端点 + 一张 usercenter token，Key/路由/计量/配额都收口在此。
//
// 鉴权：由调用方中间件负责（已写入 Principal）；本 handler 仅消费 tenantID/principal。

const (
	gatewayCooldownOn429 = 5 * time.Minute
	gatewayUpstreamTO    = 120 * time.Second
)

// RegisterAIGatewayRouter 挂载 OpenAI 兼容网关端点。鉴权由调用方中间件负责。
func RegisterAIGatewayRouter(r *gin.Engine) {
	r.POST("/v1/chat/completions", ChatCompletions)
	r.GET("/v1/models", ListModels)
}

// ListModels 返回当前租户可用的模型别名（来自已配置的 ModelRoute）。
func ListModels(c *gin.Context) {
	tenantID := ucm.GetTenantID(c)
	routes, err := apikey.GetRoutes(tenantID)
	if err != nil {
		writeErr(c, err)
		return
	}
	seen := map[string]bool{}
	data := []gin.H{}
	for _, r := range routes {
		if !r.Enable || seen[r.ModelAlias] {
			continue
		}
		seen[r.ModelAlias] = true
		data = append(data, gin.H{"id": r.ModelAlias, "object": "model", "owned_by": "usercenter"})
	}
	c.JSON(http.StatusOK, gin.H{"object": "list", "data": data})
}

// ChatCompletions OpenAI 兼容聊天补全代理（流式 + 整包）。
func ChatCompletions(c *gin.Context) {
	start := time.Now()
	tenantID := ucm.GetTenantID(c)
	principalID := ucm.GetUserID(c)
	principalKind := int32(ucm.GetPrincipalKind(c))

	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, errResp("读取请求体失败", http.StatusBadRequest))
		return
	}
	_ = c.Request.Body.Close()

	// 解析 model 与 stream 标志（不破坏原始 body 透传）
	var peek struct {
		Model    string `json:"model"`
		Stream   bool   `json:"stream"`
		Messages []struct {
			Role    string `json:"role"`
			Content any    `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(bodyBytes, &peek); err != nil {
		c.JSON(http.StatusBadRequest, errResp("非法 JSON 请求体", http.StatusBadRequest))
		return
	}
	if peek.Model == "" {
		c.JSON(http.StatusBadRequest, errResp("缺少 model 字段", http.StatusBadRequest))
		return
	}

	// 配额：调用前检查租户/Agent 预算
	if allowed, _, _, _ := usage.CheckBudget(tenantID, principalID, peek.Model); !allowed {
		recordGatewayUsage(nil, tenantID, principalID, principalKind, peek.Model, 0, 0, 0, time.Since(start), false, "quota_exceeded")
		c.JSON(http.StatusTooManyRequests, errResp("超出用量配额", http.StatusTooManyRequests))
		return
	}

	// 路由选 Key + 转发（上游 429 → 冷却 + 重试一次）
	resp, sel, upstreamErr := forwardWithRetry(c, tenantID, peek.Model, bodyBytes, peek.Stream)
	if upstreamErr != nil {
		recordGatewayUsage(nil, tenantID, principalID, principalKind, peek.Model, 0, 0, 0, time.Since(start), false, "upstream_error")
		c.JSON(http.StatusBadGateway, errResp(upstreamErr.Error(), http.StatusBadGateway))
		return
	}
	defer resp.Body.Close()

	// 用量计量
	pt, ct := int64(0), int64(0)
	if peek.Stream {
		pt, ct = streamProxy(c, resp)
	} else {
		pt, ct = bufferedProxy(c, resp)
	}
	providerName := ""
	if sel != nil && sel.Provider != nil {
		providerName = sel.Provider.Name
	}
	recordGatewayUsage(sel, tenantID, principalID, principalKind, peek.Model, pt, ct, 0, time.Since(start), true, "")
	_ = providerName
}

// forwardWithRetry 选 Key 转发；上游 429 时把该 Key 打入冷却并重试下一个路由（至多 1 次）。
func forwardWithRetry(c *gin.Context, tenantID, modelAlias string, body []byte, stream bool) (*http.Response, *apikey.KeySelection, error) {
	const maxAttempts = 3
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		sel, err := apikey.SelectKey(tenantID, modelAlias)
		if err != nil {
			return nil, nil, fmt.Errorf("无可用 Key: %w", err)
		}
		req, err := buildUpstreamRequest(sel, body)
		if err != nil {
			return nil, nil, err
		}
		client := &http.Client{Timeout: gatewayUpstreamTO}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			apikey.MarkCooldown(sel.Key.ID, gatewayCooldownOn429)
			lastErr = fmt.Errorf("上游 429（Key 已冷却，重试下一个）")
			continue
		}
		if resp.StatusCode >= 500 {
			resp.Body.Close()
			lastErr = fmt.Errorf("上游 %d", resp.StatusCode)
			continue
		}
		return resp, sel, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("转发失败")
	}
	return nil, nil, lastErr
}

// buildUpstreamRequest 构造转发到服务商的请求，按 AuthType 注入鉴权。
func buildUpstreamRequest(sel *apikey.KeySelection, body []byte) (*http.Request, error) {
	base := strings.TrimRight(sel.Provider.BaseURL, "/")
	// 兼容 baseURL 是否已含 /v1：OpenAI 兼容路径统一补 /chat/completions
	target := base + "/chat/completions"
	if strings.HasSuffix(base, "/v1") {
		target = base + "/chat/completions"
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	switch strings.ToLower(sel.Provider.AuthType) {
	case "bearer", "":
		req.Header.Set("Authorization", "Bearer "+sel.APIKey)
	case "header", "apikey":
		req.Header.Set("Authorization", sel.APIKey)
	case "query":
		// 已带 query 的场景少见，这里用 X-API-Key 兜底
		req.Header.Set("X-API-Key", sel.APIKey)
	default:
		req.Header.Set("Authorization", "Bearer "+sel.APIKey)
	}
	return req, nil
}

// bufferedProxy 透传整包响应，并解析 usage。
func bufferedProxy(c *gin.Context, resp *http.Response) (promptTokens, compTokens int64) {
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		copyHeaders(c, resp)
		c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), nil)
		return 0, 0
	}
	// 尝试解析 OpenAI usage
	var ou struct {
		Usage struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
			TotalTokens      int64 `json:"total_tokens"`
		} `json:"usage"`
	}
	_ = json.Unmarshal(buf, &ou)
	copyHeaders(c, resp)
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), buf)
	return ou.Usage.PromptTokens, ou.Usage.CompletionTokens
}

// streamProxy 透传 SSE 流，逐行扫描捕获最后一块的 usage（OpenAI stream_options.include_usage）。
func streamProxy(c *gin.Context, resp *http.Response) (promptTokens, compTokens int64) {
	flusher, ok := c.Writer.(http.Flusher)
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		// 写到客户端（含末尾换行）
		_, _ = c.Writer.Write(append(line, '\n'))
		// 捕获 usage 行：data: {... "usage": {...}}
		if bytes.HasPrefix(bytes.TrimSpace(line), []byte("data:")) {
			payload := bytes.TrimSpace(bytes.TrimPrefix(bytes.TrimSpace(line), []byte("data:")))
			if len(payload) > 0 && !bytes.Equal(payload, []byte("[DONE]")) {
				var chunk struct {
					Usage *struct {
						PromptTokens     int64 `json:"prompt_tokens"`
						CompletionTokens int64 `json:"completion_tokens"`
					} `json:"usage"`
				}
				if json.Unmarshal(payload, &chunk) == nil && chunk.Usage != nil {
					promptTokens = chunk.Usage.PromptTokens
					compTokens = chunk.Usage.CompletionTokens
				}
			}
		}
		if ok {
			flusher.Flush()
		}
	}
	return promptTokens, compTokens
}

func copyHeaders(c *gin.Context, resp *http.Response) {
	for k, vs := range resp.Header {
		for _, v := range vs {
			c.Writer.Header().Add(k, v)
		}
	}
}

// recordGatewayUsage 记录一次网关调用的用量（成功/失败均记，便于分析）。
func recordGatewayUsage(sel *apikey.KeySelection, tenantID, principalID string, principalKind int32, model string, prompt, comp, cache int64, latency time.Duration, success bool, errCode string) {
	rec := &usage.UsageRecord{
		PrincipalID:   principalID,
		PrincipalKind: principalKind,
		TenantID:      tenantID,
		ModelName:     model,
		PromptTokens:  prompt,
		CompTokens:    comp,
		CacheTokens:   cache,
		Success:       success,
		ErrorCode:     errCode,
		LatencyMs:     latency.Milliseconds(),
	}
	if sel != nil {
		if sel.Provider != nil {
			rec.ProviderID = sel.Provider.ID
			rec.ProviderName = sel.Provider.Name
		}
		if sel.Key != nil {
			rec.RequestID = sel.Key.ID // 复用字段记录所用 Key，便于审计
		}
	}
	usage.RecordUsage(rec)
}

func errResp(msg string, code int) gin.H {
	return gin.H{"error": gin.H{"message": msg, "code": code}}
}
