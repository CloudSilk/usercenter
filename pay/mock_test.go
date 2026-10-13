package pay

import (
	"bytes"
	"context"
	"errors"
	"net/http/httptest"
	"testing"
)

func TestMockSignature(t *testing.T) {
	m := NewMock("s3cret")
	m.Prepay(context.Background(), PrepayReq{OrderID: "o1", AmountFen: 100})
	body, sig := m.MarkPaid("o1")
	req := httptest.NewRequest("POST", "/pay/mock/notify", bytes.NewReader(body))
	req.Header.Set("X-Mock-Signature", sig)
	n, err := m.ParseNotify(context.Background(), req)
	if err != nil || n.Txn.State != "SUCCESS" {
		t.Fatal(n, err)
	}
	req = httptest.NewRequest("POST", "/pay/mock/notify", bytes.NewReader(body))
	req.Header.Set("X-Mock-Signature", "bad")
	if _, err := m.ParseNotify(context.Background(), req); !errors.Is(err, ErrBadSignature) {
		t.Fatal(err)
	}
}
