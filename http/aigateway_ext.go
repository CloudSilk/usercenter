package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/CloudSilk/usercenter/internal/alert"
	"github.com/CloudSilk/usercenter/internal/aicache"
	"github.com/CloudSilk/usercenter/internal/apikey"
	"github.com/CloudSilk/usercenter/internal/conversation"
	"github.com/CloudSilk/usercenter/internal/gatewaylog"
	"github.com/CloudSilk/usercenter/internal/prompt"
	"github.com/CloudSilk/usercenter/internal/ratelimit"
	"github.com/CloudSilk/usercenter/internal/usage"
	ucm "github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/gin-gonic/gin"
)

// AI 网关增强能力（Wave 2-8）：
//   - 对话会话管理（session_id 自动加载历史）
//   - Prompt 模板自动注入（prompt_template_id + prompt_vars）
//   - 语义缓存（相似 prompt 命中缓存，跳过 LLM 调用）
//   - 音频端点（Whisper 转写 + TTS 合成）
//   - 内容审核（/v1/moderations + 可选输入/输出审核）
//   - 请求日志全量记录（gatewaylog）
//   - 每用户限流（令牌桶）

// chatEnhancedFields 从请求体中提取增强字段（不影响原 body 透传）。
type chatEnhancedFields struct {
	SessionID         string                 `json:"session_id"`
	PromptTemplateID  string                 `json:"prompt_template_id"`
	PromptVars        map[string]string      `json:"prompt_vars"`
	Moderate          bool                   `json:"moderate"`
	ModerateOutput    bool                   `json:"moderate_output"`
}

// extractChatEnhancements 解析增强字段，返回增强后的 body 和元数据。
// 注意：增强字段会从 body 中移除后再透传给上游（上游不认识这些字段）。
func extractChatEnhancements(body []byte) (cleanBody []byte, enh chatEnhancedFields, model string, stream bool, err error) {
	var m map[string]any
	if err = json.Unmarshal(body, &m); err != nil {
		return body, enh, "", false, fmt.Errorf("非法 JSON 请求体")
	}
	if v, ok := m["model"].(string); ok {
		model = v
	}
	if v, ok := m["stream"].(bool); ok {
		stream = v
	}
	if v, ok := m["session_id"].(string); ok {
		enh.SessionID = v
	}
	if v, ok := m["prompt_template_id"].(string); ok {
		enh.PromptTemplateID = v
	}
	if v, ok := m["prompt_vars"].(map[string]any); ok {
		enh.PromptVars = make(map[string]string, len(v))
		for k, val := range v {
			enh.PromptVars[k] = fmt.Sprintf("%v", val)
		}
	}
	if v, ok := m["moderate"].(bool); ok {
		enh.Moderate = v
	}
	if v, ok := m["moderate_output"].(bool); ok {
		enh.ModerateOutput = v
	}
	// 移除增强字段后重新序列化
	delete(m, "session_id")
	delete(m, "prompt_template_id")
	delete(m, "prompt_vars")
	delete(m, "moderate")
	delete(m, "moderate_output")
	cleanBody, err = json.Marshal(m)
	if err != nil {
		return body, enh, "", false, fmt.Errorf("请求体序列化失败: %w", err)
	}
	return cleanBody, enh, model, stream, nil
}

// applyPromptTemplate 加载并渲染 Prompt 模板，注入为 system message。
func applyPromptTemplate(body []byte, templateID string, vars map[string]string) ([]byte, error) {
	if templateID == "" {
		return body, nil
	}
	tpl, err := prompt.GetByID(templateID)
	if err != nil || tpl == nil {
		return body, fmt.Errorf("prompt 模板不存在: %s", templateID)
	}
	rendered := prompt.Render(tpl.Content, vars)

	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return body, err
	}
	messages, _ := m["messages"].([]any)
	// 如果首条是 system，追加；否则插入到首位
	systemMsg := map[string]any{"role": "system", "content": rendered}
	if len(messages) > 0 {
		if first, ok := messages[0].(map[string]any); ok {
			if role, _ := first["role"].(string); role == "system" {
				// 追加到现有 system content
				existing, _ := first["content"].(string)
				first["content"] = existing + "\n\n" + rendered
				m["messages"] = messages
				out, _ := json.Marshal(m)
				return out, nil
			}
		}
	}
	// 插入到首位
	m["messages"] = append([]any{systemMsg}, messages...)
	out, _ := json.Marshal(m)
	return out, nil
}

// applyConversationHistory 加载会话历史消息，prepend 到 messages 数组。
func applyConversationHistory(body []byte, sessionID string, limit int) ([]byte, error) {
	if sessionID == "" {
		return body, nil
	}
	if limit <= 0 {
		limit = 20
	}
	msgs, err := conversation.GetMessages(sessionID, limit)
	if err != nil || len(msgs) == 0 {
		return body, nil
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return body, err
	}
	messages, _ := m["messages"].([]any)
	// 构建历史消息数组
	history := make([]any, 0, len(msgs))
	for _, msg := range msgs {
		history = append(history, map[string]any{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}
	m["messages"] = append(history, messages...)
	out, _ := json.Marshal(m)
	return out, nil
}

// buildCachePrompt 从 messages 数组构建用于缓存键的完整提示文本（含 system/历史/user 全部上下文）。
// 相比仅取最后一条 user 消息，含完整上下文可避免"相同最后一句但上下文不同"时误命中缓存。
// 返回原始文本（不 hash）：aicache 内部对它做 SHA-256 精确匹配 + embedding 语义匹配，两者都依赖原始文本。
func buildCachePrompt(body []byte) string {
	var m struct {
		Messages []struct {
			Role    string `json:"role"`
			Content any    `json:"content"`
		} `json:"messages"`
	}
	if json.Unmarshal(body, &m) != nil {
		return ""
	}
	var sb strings.Builder
	for _, msg := range m.Messages {
		sb.WriteString(msg.Role)
		sb.WriteString(":")
		sb.WriteString(fmt.Sprintf("%v", msg.Content))
		sb.WriteString("\n")
	}
	return sb.String()
}

// extractLastUserMessage 从 messages 数组中提取最后一条 user 消息。
func extractLastUserMessage(body []byte) string {
	var m struct {
		Messages []struct {
			Role    string `json:"role"`
			Content any    `json:"content"`
		} `json:"messages"`
	}
	if json.Unmarshal(body, &m) != nil {
		return ""
	}
	for i := len(m.Messages) - 1; i >= 0; i-- {
		if m.Messages[i].Role == "user" {
			return fmt.Sprintf("%v", m.Messages[i].Content)
		}
	}
	return ""
}

// extractAssistantContent 从响应 body 中提取 assistant 回复内容。
func extractAssistantContent(body []byte) string {
	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage *struct {
			CompletionTokens int64 `json:"completion_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(body, &resp) != nil {
		return ""
	}
	if len(resp.Choices) > 0 {
		return resp.Choices[0].Message.Content
	}
	return ""
}

// recordGatewayLog 记录网关请求日志（非流式）。
func recordGatewayLog(c *gin.Context, tenantID, principalID, model, sessionID string,
	promptTokens, compTokens int64, cost float64, latency time.Duration,
	statusCode int, success bool, errMsg, requestBody string, cached bool) {
	entry := &gatewaylog.GatewayLog{
		TenantID:        tenantID,
		PrincipalID:     principalID,
		RequestID:       ucm.TraceID(c),
		Method:          c.Request.URL.Path,
		ModelAlias:      model,
		Stream:          false,
		PromptTokens:    promptTokens,
		CompTokens:      compTokens,
		Cost:            cost,
		LatencyMs:       latency.Milliseconds(),
		StatusCode:      statusCode,
		Success:         success,
		ErrorMessage:    errMsg,
		Cached:          cached,
		SessionID:       sessionID,
		RequestBody:     truncateStr(requestBody, 2000),
	}
	gatewaylog.Record(entry)
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

// --- 音频端点 ---

// AudioTranscriptions Whisper 兼容语音转写端点。
// 支持 multipart/form-data 上传音频文件，透传到上游 /audio/transcriptions。
func AudioTranscriptions(c *gin.Context) {
	start := time.Now()
	tenantID := ucm.GetTenantID(c)
	principalID := ucm.GetUserID(c)
	pk := int32(ucm.GetPrincipalKind(c))
	principalKind := pk - 1
	if principalKind < 0 {
		principalKind = 0
	}

	// 限流检查
	if !ratelimit.CheckRateLimit(principalID) {
		c.Header("Retry-After", "1")
		c.JSON(http.StatusTooManyRequests, errResp("请求过于频繁", http.StatusTooManyRequests))
		return
	}

	// 解析 multipart body 透传
	contentType := c.ContentType()
	if contentType != "multipart/form-data" {
		c.JSON(http.StatusBadRequest, errResp("需要 multipart/form-data 请求", http.StatusBadRequest))
		return
	}

	// 读取 model 字段
	model := c.PostForm("model")
	if model == "" {
		c.JSON(http.StatusBadRequest, errResp("缺少 model 字段", http.StatusBadRequest))
		return
	}

	if allowed, _, _, _ := usage.CheckBudget(tenantID, principalID, model); !allowed {
		observeAIQuotaExceeded()
		alert.FireWebhook("ai_quota_exceeded", map[string]any{"tenantID": tenantID, "principalID": principalID, "model": model})
		c.JSON(http.StatusTooManyRequests, errResp("超出用量配额", http.StatusTooManyRequests))
		return
	}

	// 重新构造 multipart 请求体转发
	resp, sel, upstreamErr := forwardMultipart(c, tenantID, model, "/audio/transcriptions")
	if upstreamErr != nil {
		recordGatewayUsage(nil, tenantID, principalID, principalKind, model, 0, 0, 0, time.Since(start), false, "upstream_error")
		c.JSON(http.StatusBadGateway, errResp(upstreamErr.Error(), http.StatusBadGateway))
		return
	}
	defer resp.Body.Close()

	buf, _ := io.ReadAll(resp.Body)
	copyHeaders(c, resp)
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), buf)
	cost := recordGatewayUsage(sel, tenantID, principalID, principalKind, model, 0, 1, 0, time.Since(start), true, "")
	recordGatewayLog(c, tenantID, principalID, model, "", 0, 1, cost, time.Since(start), resp.StatusCode, resp.StatusCode < 400, "", "", false)
}

// AudioSpeech TTS 兼容语音合成端点。
// JSON body: {model, input, voice, response_format, speed}
func AudioSpeech(c *gin.Context) {
	start := time.Now()
	tenantID := ucm.GetTenantID(c)
	principalID := ucm.GetUserID(c)
	pk := int32(ucm.GetPrincipalKind(c))
	principalKind := pk - 1
	if principalKind < 0 {
		principalKind = 0
	}

	if !ratelimit.CheckRateLimit(principalID) {
		c.Header("Retry-After", "1")
		c.JSON(http.StatusTooManyRequests, errResp("请求过于频繁", http.StatusTooManyRequests))
		return
	}

	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, errResp("读取请求体失败", http.StatusBadRequest))
		return
	}
	_ = c.Request.Body.Close()

	var peek struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(bodyBytes, &peek); err != nil {
		c.JSON(http.StatusBadRequest, errResp("非法 JSON 请求体", http.StatusBadRequest))
		return
	}
	if peek.Model == "" {
		c.JSON(http.StatusBadRequest, errResp("缺少 model 字段", http.StatusBadRequest))
		return
	}

	if allowed, _, _, _ := usage.CheckBudget(tenantID, principalID, peek.Model); !allowed {
		observeAIQuotaExceeded()
		alert.FireWebhook("ai_quota_exceeded", map[string]any{"tenantID": tenantID, "principalID": principalID, "model": peek.Model})
		c.JSON(http.StatusTooManyRequests, errResp("超出用量配额", http.StatusTooManyRequests))
		return
	}

	resp, sel, upstreamErr := forwardWithRetry(c, tenantID, peek.Model, bodyBytes, false, "/audio/speech")
	if upstreamErr != nil {
		recordGatewayUsage(nil, tenantID, principalID, principalKind, peek.Model, 0, 0, 0, time.Since(start), false, "upstream_error")
		c.JSON(http.StatusBadGateway, errResp(upstreamErr.Error(), http.StatusBadGateway))
		return
	}
	defer resp.Body.Close()

	// 音频流直接透传
	buf, _ := io.ReadAll(resp.Body)
	copyHeaders(c, resp)
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), buf)
	cost := recordGatewayUsage(sel, tenantID, principalID, principalKind, peek.Model, 0, 1, 0, time.Since(start), true, "")
	recordGatewayLog(c, tenantID, principalID, peek.Model, "", 0, 1, cost, time.Since(start), resp.StatusCode, resp.StatusCode < 400, "", "", false)
}

// forwardMultipart 转发 multipart/form-data 请求到上游。
func forwardMultipart(c *gin.Context, tenantID, modelAlias, pathSuffix string) (*http.Response, *apikey.KeySelection, error) {
	const maxAttempts = 3
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		sel, err := apikey.SelectKey(tenantID, modelAlias)
		if err != nil {
			return nil, nil, fmt.Errorf("无可用 Key: %w", err)
		}
		// 重建上游 multipart 请求
		base := trimBaseURL(sel.Provider.BaseURL)
		target := base + pathSuffix
		req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, target, c.Request.Body)
		if err != nil {
			return nil, nil, err
		}
		req.Header.Set("Content-Type", c.ContentType())
		injectAuth(req, sel)
		client := &http.Client{Timeout: gatewayUpstreamTO}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			apikey.MarkCooldown(sel.Key.ID, gatewayCooldownOn429)
			lastErr = fmt.Errorf("上游 429")
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

func trimBaseURL(base string) string {
	for len(base) > 0 && base[len(base)-1] == '/' {
		base = base[:len(base)-1]
	}
	return base
}

func injectAuth(req *http.Request, sel *apikey.KeySelection) {
	switch authType(sel.Provider.AuthType) {
	case "bearer", "":
		req.Header.Set("Authorization", "Bearer "+sel.APIKey)
	case "header", "apikey":
		req.Header.Set("Authorization", sel.APIKey)
	case "query":
		req.Header.Set("X-API-Key", sel.APIKey)
	default:
		req.Header.Set("Authorization", "Bearer "+sel.APIKey)
	}
}

type authType string

// --- 内容审核端点 ---

// Moderations OpenAI 兼容内容审核端点。
func Moderations(c *gin.Context) {
	start := time.Now()
	tenantID := ucm.GetTenantID(c)
	principalID := ucm.GetUserID(c)
	pk := int32(ucm.GetPrincipalKind(c))
	principalKind := pk - 1
	if principalKind < 0 {
		principalKind = 0
	}

	// --- 限流检查（每用户令牌桶）---
	if !ratelimit.CheckRateLimit(principalID) {
		c.Header("Retry-After", "1")
		c.JSON(http.StatusTooManyRequests, errResp("请求过于频繁", http.StatusTooManyRequests))
		return
	}

	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, errResp("读取请求体失败", http.StatusBadRequest))
		return
	}
	_ = c.Request.Body.Close()

	var peek struct {
		Model string `json:"model"`
	}
	_ = json.Unmarshal(bodyBytes, &peek)
	model := peek.Model
	if model == "" {
		model = "text-moderation-latest"
	}

	resp, sel, upstreamErr := forwardWithRetry(c, tenantID, model, bodyBytes, false, "/moderations")
	if upstreamErr != nil {
		c.JSON(http.StatusBadGateway, errResp(upstreamErr.Error(), http.StatusBadGateway))
		return
	}
	defer resp.Body.Close()

	buf, _ := io.ReadAll(resp.Body)
	copyHeaders(c, resp)
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), buf)
	cost := recordGatewayUsage(sel, tenantID, principalID, principalKind, model, 0, 1, 0, time.Since(start), true, "")
	recordGatewayLog(c, tenantID, principalID, model, "", 0, 1, cost, time.Since(start), resp.StatusCode, resp.StatusCode < 400, "", "", false)
}

// moderationFailOpen 控制审核服务不可用 / 结果无法解析时的策略：
// true（默认）= fail-open 放行（保证可用性）；false = fail-close 拒绝（保证安全性，合规场景使用）。
var moderationFailOpen = true

// SetModerationFailOpen 配置审核 fail-open/close 策略。
func SetModerationFailOpen(failOpen bool) {
	moderationFailOpen = failOpen
}

// moderateInput 在 chat completion 前对输入做内容审核。
// 返回 true 表示安全可放行，false 表示不安全需拒绝。
func moderateInput(c *gin.Context, body []byte, tenantID, model string) (safe bool) {
	return moderateText(c, tenantID, extractLastUserMessage(body))
}

// moderateText 对任意文本做内容审核，返回 true 表示安全。
func moderateText(c *gin.Context, tenantID, text string) (safe bool) {
	if strings.TrimSpace(text) == "" {
		return true
	}
	moderationBody := map[string]any{
		"model": aiAuxModerationModel,
		"input": text,
	}
	mb, _ := json.Marshal(moderationBody)
	resp, _, err := forwardWithRetry(c, tenantID, aiAuxModerationModel, mb, false, "/moderations")
	if err != nil || resp == nil {
		return moderationFailOpen // 审核服务不可用：fail-open 放行 / fail-close 拒绝
	}
	defer resp.Body.Close()
	buf, _ := io.ReadAll(resp.Body)
	var result struct {
		Results []struct {
			Flagged bool `json:"flagged"`
		} `json:"results"`
	}
	if json.Unmarshal(buf, &result) == nil && len(result.Results) > 0 {
		return !result.Results[0].Flagged
	}
	return moderationFailOpen // 无法解析审核结果：按 fail-open/close 策略
}

// AI 网关辅助任务（标题生成 / 嵌入 / 内容审核）使用的模型别名。
// 默认值为 OpenAI 公共别名，可通过 SetAIAuxModels 覆盖以适配不同部署的可用模型。
var (
	aiAuxTitleModel      = "gpt-3.5-turbo"
	aiAuxEmbeddingModel  = "text-embedding-3-small"
	aiAuxModerationModel = "text-moderation-latest"
)

// SetAIAuxModels 配置网关辅助任务使用的模型别名（传空串则保留原值）。
// 用于适配非 OpenAI 部署（如 DeepSeek、自建模型），避免硬编码 gpt-3.5-turbo 等导致路由选 Key 失败。
func SetAIAuxModels(titleModel, embeddingModel, moderationModel string) {
	if titleModel != "" {
		aiAuxTitleModel = titleModel
	}
	if embeddingModel != "" {
		aiAuxEmbeddingModel = embeddingModel
	}
	if moderationModel != "" {
		aiAuxModerationModel = moderationModel
	}
}

// cacheConfigOverride 允许在路由注册前覆盖默认缓存配置（由 main.go 从 config 注入）。
// 为 nil 时 initAIEnhancements 使用内置默认值。
var cacheConfigOverride *aicache.CacheConfig

// SetCacheConfig 覆盖语义缓存默认配置（必须在 RegisterAIGatewayRouter 之前调用）。
// 各字段为 0 值时由 aicache.New 回填默认（threshold 0.95 / TTL 24h / maxEntries 10000）。
func SetCacheConfig(enabled bool, threshold float64, ttl time.Duration, maxEntries int) {
	cacheConfigOverride = &aicache.CacheConfig{
		Enabled:             enabled,
		SimilarityThreshold: threshold,
		TTL:                 ttl,
		MaxEntries:          maxEntries,
	}
}

// initAIEnhancements 初始化增强能力（缓存 + 嵌入函数）。
func initAIEnhancements() {
	cfg := aicache.CacheConfig{
		Enabled:             true,
		SimilarityThreshold: 0.95,
		TTL:                 24 * time.Hour,
		MaxEntries:          10000,
	}
	if cacheConfigOverride != nil {
		cfg = *cacheConfigOverride
	}
	aicache.Init(cfg)

	// 注入嵌入函数：调用 /v1/embeddings 生成 prompt 向量，使缓存支持语义相似度匹配。
	// 平台租户（tenantID=""）查询全局路由；失败时返回 nil，缓存回退到精确哈希。
	aicache.SetEmbeddingFunc(generateEmbedding)
}

// summarizeSession 用 LLM 为会话生成简短标题。
// 取最近若干条消息，请求 LLM 生成 ≤20 字的标题，更新到会话。
// 失败时静默（标题保持原状）。
func summarizeSession(sessionID string) {
	msgs, err := conversation.GetMessages(sessionID, 6)
	if err != nil || len(msgs) == 0 {
		return
	}
	// 拼接对话片段
	var sb strings.Builder
	for _, m := range msgs {
		sb.WriteString(m.Role)
		sb.WriteString(": ")
		content := m.Content
		if len(content) > 200 {
			content = content[:200]
		}
		sb.WriteString(content)
		sb.WriteString("\n")
	}
	promptText := "请用不超过20个中文字符为以下对话生成一个简短标题，只返回标题文字，不要标点：\n" + sb.String()

	body, _ := json.Marshal(map[string]any{
		"model":       aiAuxTitleModel,
		"max_tokens":  30,
		"temperature": 0,
		"messages": []map[string]string{
			{"role": "user", "content": promptText},
		},
	})

	title := callLLMForText(aiAuxTitleModel, body)
	if title != "" {
		// 截断到合理长度
		if len([]rune(title)) > 30 {
			title = string([]rune(title)[:30])
		}
		_ = conversation.UpdateSessionTitle(sessionID, title)
	}
}

// callLLMForText 调用 chat completions 提取纯文本回复（用于标题生成等辅助任务）。
func callLLMForText(model string, body []byte) string {
	sel, err := apikey.SelectKey("", model)
	if err != nil || sel == nil {
		return ""
	}
	req, err := buildUpstreamRequest(sel, body, "/chat/completions")
	if err != nil {
		return ""
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil || resp == nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return ""
	}
	buf, _ := io.ReadAll(resp.Body)
	return extractAssistantContent(buf)
}

// generateEmbedding 调用 embeddings 端点生成文本嵌入向量。
// 失败返回 nil（缓存回退到精确哈希匹配，不阻断主流程）。
func generateEmbedding(prompt string) []float32 {
	// 截断超长 prompt，避免嵌入成本过高
	text := prompt
	if len(text) > 8000 {
		text = text[:8000]
	}
	body, _ := json.Marshal(map[string]any{
		"model": aiAuxEmbeddingModel,
		"input": text,
	})

	// 用空 tenantID 查询全局路由，直接选 Key 转发（无需 gin.Context）
	sel, err := apikey.SelectKey("", aiAuxEmbeddingModel)
	if err != nil || sel == nil {
		return nil
	}
	req, err := buildUpstreamRequest(sel, body, "/embeddings")
	if err != nil {
		return nil
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil || resp == nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil
	}
	buf, _ := io.ReadAll(resp.Body)
	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if json.Unmarshal(buf, &result) != nil || len(result.Data) == 0 {
		return nil
	}
	return result.Data[0].Embedding
}
