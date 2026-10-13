package pay

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Mock 模拟渠道：内存记账；“支付”由 /pay/mock/notify 带 HMAC 签名的回调完成（Sign 生成）。
// 仅用于本地开发、测试与演示，线上必须配置 jsapi 或 virtual。
type Mock struct {
	Secret string
	mu     sync.Mutex
	orders map[string]*Txn
}

func NewMock(secret string) *Mock { return &Mock{Secret: secret, orders: map[string]*Txn{}} }

func (m *Mock) Name() string { return "mock" }

func (m *Mock) Prepay(_ context.Context, r PrepayReq) (string, map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.orders[r.OrderID] = &Txn{OrderID: r.OrderID, State: "NOTPAY", AmountFen: r.AmountFen}
	pid := "mock_prepay_" + r.OrderID
	return pid, map[string]string{"provider": "mock", "package": "prepay_id=" + pid, "orderId": r.OrderID}, nil
}

// MarkPaid 模拟用户完成支付（返回回调体与签名，便于调用方投递回调）
func (m *Mock) MarkPaid(orderID string) (body []byte, sig string) {
	m.mu.Lock()
	t := m.orders[orderID]
	if t == nil {
		t = &Txn{OrderID: orderID}
		m.orders[orderID] = t
	}
	now := time.Now()
	t.State, t.TransactionID, t.PaidAt = "SUCCESS", "mock_tx_"+orderID, &now
	cp := *t
	m.mu.Unlock()
	body, _ = json.Marshal(map[string]any{"id": "mock_evt_pay_" + orderID, "kind": "pay", "txn": cp})
	return body, m.Sign(body)
}

func (m *Mock) Sign(body []byte) string {
	h := hmac.New(sha256.New, []byte(m.Secret))
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

func (m *Mock) Query(_ context.Context, id string) (*Txn, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t := m.orders[id]; t != nil {
		cp := *t
		return &cp, nil
	}
	return &Txn{OrderID: id, State: "NOTPAY"}, nil
}

func (m *Mock) Close(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t := m.orders[id]; t != nil {
		if t.State == "SUCCESS" {
			return fmt.Errorf("mock: 订单已支付，不能关单")
		}
		t.State = "CLOSED"
	}
	return nil
}

// Refund 模拟退款：受理后立即成功（同时生成退款回调，由调用方选择投递）
func (m *Mock) Refund(_ context.Context, r RefundReq) (*RefundResult, error) {
	if r.RefundFen <= 0 || r.RefundFen > r.TotalFen {
		return nil, fmt.Errorf("mock: 退款金额无效")
	}
	return &RefundResult{OrderID: r.OrderID, RefundID: r.RefundID, ProviderRefundID: "mock_rf_" + r.RefundID, State: "SUCCESS"}, nil
}

func (m *Mock) ParseNotify(_ context.Context, req *http.Request) (*Notify, error) {
	body, _ := io.ReadAll(req.Body)
	if !hmac.Equal([]byte(m.Sign(body)), []byte(req.Header.Get("X-Mock-Signature"))) {
		return nil, ErrBadSignature
	}
	var v struct {
		ID     string        `json:"id"`
		Kind   string        `json:"kind"`
		Txn    *Txn          `json:"txn"`
		Refund *RefundResult `json:"refund"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, err
	}
	return &Notify{EventID: v.ID, Kind: v.Kind, Txn: v.Txn, Refund: v.Refund}, nil
}
