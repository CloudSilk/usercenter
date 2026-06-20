package permission

// IP 级限流(#13 联动拦截) + Webhook/Event(#20)

import (
	"context"
	"sync"
	"time"

	"github.com/CloudSilk/pkg/utils/log"
)

// IPRateLimiter IP 级限流器(滑动窗口)
type IPRateLimiter struct {
	mu     sync.Mutex
	window time.Duration
	limit  int
	hits   map[string][]time.Time
}

// NewIPRateLimiter 创建限流器
func NewIPRateLimiter(limit int, window time.Duration) *IPRateLimiter {
	return &IPRateLimiter{
		window: window,
		limit:  limit,
		hits:   make(map[string][]time.Time),
	}
}

// Allow 检查 IP 是否允许请求(滑动窗口)
func (r *IPRateLimiter) Allow(ip string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-r.window)

	// 清理过期记录
	var valid []time.Time
	for _, t := range r.hits[ip] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= r.limit {
		r.hits[ip] = valid
		return false
	}

	r.hits[ip] = append(valid, now)
	return true
}

// GlobalRateLimiter 全局实例(由 middleware 使用)
var GlobalRateLimiter = NewIPRateLimiter(100, time.Minute) // 默认 100 req/min

// --- Webhook/Event(#20) ---

// EventType 事件类型
type EventType string

const (
	EventUserCreated      EventType = "user.created"
	EventUserDisabled     EventType = "user.disabled"
	EventRoleChanged      EventType = "user.role_changed"
	EventPermissionChanged EventType = "user.permissions_changed"
	EventTenantCreated    EventType = "tenant.created"
)

// EventHandler 事件处理函数签名
type EventHandler func(eventType EventType, data map[string]interface{})

// EventBus 进程内事件总线(简易实现,不引 Kafka/NATS)
type EventBus struct {
	mu       sync.RWMutex
	handlers map[EventType][]EventHandler
}

var globalBus = &EventBus{handlers: make(map[EventType][]EventHandler)}

// Subscribe 订阅事件
func Subscribe(eventType EventType, handler EventHandler) {
	globalBus.mu.Lock()
	defer globalBus.mu.Unlock()
	globalBus.handlers[eventType] = append(globalBus.handlers[eventType], handler)
}

// Publish 发布事件(同步调用所有 handler,handler 内部可异步)
func Publish(eventType EventType, data map[string]interface{}) {
	globalBus.mu.RLock()
	handlers := globalBus.handlers[eventType]
	globalBus.mu.RUnlock()

	for _, h := range handlers {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Errorf(context.Background(), "event handler panic: %v (event=%s)", r, eventType)
				}
			}()
			h(eventType, data)
		}()
	}
}
