package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TraceContextKey 是写入 gin.Context 的 trace_id 键名。
const TraceContextKey = "trace_id"

// TraceID 从 gin.Context 中读取已注入的 trace_id。
// 若不存在返回空字符串（调用方应按需兜底生成）。
func TraceID(c *gin.Context) string {
	if v, ok := c.Get(TraceContextKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// RequestIDMiddleware 生成/转发 X-Request-ID 并注入 trace_id 到 gin.Context。
//
// 行为：
//  1. 读取请求头 X-Request-ID，若存在则沿用（透传上游 trace）；
//  2. 若不存在则使用 uuid.New().String() 生成；
//  3. 注入 gin.Context key "trace_id"（可通过 TraceID() 取出）；
//  4. 设置响应头 X-Request-ID，使调用方能关联请求。
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader("X-Request-ID")
		if rid == "" {
			rid = c.GetHeader("X-Request-Id")
		}
		if rid == "" {
			rid = c.GetHeader("x-request-id")
		}
		if rid == "" {
			rid = uuid.New().String()
		}
		// 剪掉可能被截断的换行/空白
		rid = strings.TrimSpace(rid)

		c.Set(TraceContextKey, rid)
		c.Header("X-Request-ID", rid)
		c.Next()
	}
}
