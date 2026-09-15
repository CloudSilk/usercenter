package alert

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestFireWebhookNoURL(t *testing.T) {
	SetWebhookURL("")
	defer SetWebhookURL("")

	// 空 URL:无操作,不应 panic
	FireWebhook("test_event", map[string]interface{}{"key": "val"})
}

func TestFireWebhookDelivers(t *testing.T) {
	received := make(chan map[string]interface{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		received <- body
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	SetWebhookURL(server.URL)
	defer SetWebhookURL("")

	FireWebhook("test_event", map[string]interface{}{"key": "val"})
	select {
	case body := <-received:
		if body["event"] != "test_event" {
			t.Fatalf("expected event test_event, got %v", body["event"])
		}
		payload, ok := body["payload"].(map[string]interface{})
		if !ok || payload["key"] != "val" {
			t.Fatalf("unexpected payload: %v", payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("webhook delivery timeout")
	}
}

func TestFireWebhookContentType(t *testing.T) {
	contentType := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType <- r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	SetWebhookURL(server.URL)
	defer SetWebhookURL("")

	FireWebhook("ct_test", map[string]interface{}{})
	select {
	case ct := <-contentType:
		if ct != "application/json" {
			t.Fatalf("unexpected content type: %q", ct)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("webhook delivery timeout")
	}
}

func TestConcurrentFireWebhook(t *testing.T) {
	var mu sync.Mutex
	var count int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		count++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	SetWebhookURL(server.URL)
	defer SetWebhookURL("")

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			FireWebhook("concurrent_test", map[string]interface{}{"n": n})
		}(i)
	}
	wg.Wait()
	time.Sleep(500 * time.Millisecond) // 等待异步推送完成
	mu.Lock()
	c := count
	mu.Unlock()
	if c < 5 {
		t.Fatalf("expected at least 5 webhook calls, got %d", c)
	}
}
