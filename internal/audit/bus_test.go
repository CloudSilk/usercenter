package audit

import (
	"testing"
	"time"
)

// TestEventBus_PublishDelivers 验证订阅者收到发布的事件。
func TestEventBus_PublishDelivers(t *testing.T) {
	ch, unsub := Subscribe()
	defer unsub()

	al := &AuditLog{Action: "test_action", UserID: "u1"}
	go func() {
		time.Sleep(10 * time.Millisecond)
		publish(al)
	}()

	select {
	case got := <-ch:
		if got.Action != "test_action" || got.UserID != "u1" {
			t.Fatalf("收到的事件不正确: %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("订阅者未收到事件")
	}
}

// TestEventBus_UnsubscribeStops 验证取消订阅后不再收到事件。
func TestEventBus_UnsubscribeStops(t *testing.T) {
	ch, unsub := Subscribe()
	unsub()

	// 取消订阅后通道已关闭，publish 不应 panic
	publish(&AuditLog{Action: "x"})

	// 读已关闭通道应得到零值 + ok=false（或阻塞前关闭）
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("取消订阅后不应再收到事件")
		}
	default:
		// 通道已关闭且无数据，符合预期
	}
}

// TestEventBus_MultipleSubscribers 验证多订阅者各自收到。
func TestEventBus_MultipleSubscribers(t *testing.T) {
	ch1, un1 := Subscribe()
	ch2, un2 := Subscribe()
	defer un1()
	defer un2()

	publish(&AuditLog{Action: "broadcast"})
	for i, ch := range []<-chan *AuditLog{ch1, ch2} {
		select {
		case got := <-ch:
			if got.Action != "broadcast" {
				t.Fatalf("订阅者 %d 收到错误事件: %+v", i, got)
			}
		case <-time.After(time.Second):
			t.Fatalf("订阅者 %d 未收到事件", i)
		}
	}
}
