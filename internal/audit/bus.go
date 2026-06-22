package audit

import (
	"context"
	"sync"

	"github.com/CloudSilk/pkg/utils/log"
)

// 进程内事件总线：RecordAuditWithKind 写库成功后广播给 SSE 订阅者（管理后台实时审计大屏）。
// 每个订阅者一个带缓冲通道；满则丢弃（不阻塞写库与发布）。

const subscriberBuffer = 64

var (
	busMu sync.RWMutex
	subs  = map[chan *AuditLog]struct{}{}
)

// Subscribe 订阅审计事件流，返回事件通道与取消订阅函数。
func Subscribe() (<-chan *AuditLog, func()) {
	ch := make(chan *AuditLog, subscriberBuffer)
	busMu.Lock()
	subs[ch] = struct{}{}
	busMu.Unlock()
	return ch, func() {
		busMu.Lock()
		if _, ok := subs[ch]; ok {
			delete(subs, ch)
			close(ch)
		}
		busMu.Unlock()
	}
}

// publish 非阻塞广播：订阅者通道满则丢弃该条（保护审计写入路径）。
func publish(al *AuditLog) {
	busMu.RLock()
	defer busMu.RUnlock()
	for ch := range subs {
		select {
		case ch <- al:
		default:
			// 订阅者落后，丢弃以免拖慢主路径
			log.Errorf(context.Background(), "audit subscriber buffer full, dropping event")
		}
	}
}
