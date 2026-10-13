package pay

import (
	"bytes"
	"context"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

const apiV3Key = "0123456789abcdef0123456789abcdef"

func pemPriv(k *rsa.PrivateKey) string {
	b, _ := x509.MarshalPKCS8PrivateKey(k)
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: b}))
}
func pemPub(k *rsa.PublicKey) string {
	b, _ := x509.MarshalPKIXPublicKey(k)
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: b}))
}

// 模拟微信支付平台：用“平台私钥”签应答与回调
type fakeWx struct {
	plat     *rsa.PrivateKey
	merchant *rsa.PublicKey
	srv      *httptest.Server
	paths    []string
}

func (f *fakeWx) sign(w http.Header, body []byte) {
	ts, nonce := strconv.FormatInt(time.Now().Unix(), 10), "n"+strconv.FormatInt(time.Now().UnixNano(), 10)
	h := sha256.Sum256([]byte(ts + "\n" + nonce + "\n" + string(body) + "\n"))
	sig, _ := rsa.SignPKCS1v15(rand.Reader, f.plat, crypto.SHA256, h[:])
	w.Set("Wechatpay-Timestamp", ts)
	w.Set("Wechatpay-Nonce", nonce)
	w.Set("Wechatpay-Serial", "PUB_KEY_ID_TEST")
	w.Set("Wechatpay-Signature", base64.StdEncoding.EncodeToString(sig))
	w.Set("Request-Id", "req-1")
}

func newFake(t *testing.T) (*fakeWx, WechatConfig) {
	plat, _ := rsa.GenerateKey(rand.Reader, 2048)
	mer, _ := rsa.GenerateKey(rand.Reader, 2048)
	f := &fakeWx{plat: plat, merchant: &mer.PublicKey}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.paths = append(f.paths, r.Method+" "+r.URL.Path)
		if !strings.HasPrefix(r.Header.Get("Authorization"), "WECHATPAY2-SHA256-RSA2048 ") {
			w.WriteHeader(401)
			return
		}
		var body []byte
		switch {
		case r.URL.Path == "/v3/pay/transactions/jsapi":
			body = []byte(`{"prepay_id":"wx_prepay_1"}`)
		case strings.HasPrefix(r.URL.Path, "/v3/pay/transactions/out-trade-no/") && strings.HasSuffix(r.URL.Path, "/close"):
			f.sign(w.Header(), nil)
			w.WriteHeader(204)
			return
		case strings.HasPrefix(r.URL.Path, "/v3/pay/transactions/out-trade-no/"):
			id := strings.TrimPrefix(r.URL.Path, "/v3/pay/transactions/out-trade-no/")
			body = []byte(fmt.Sprintf(`{"out_trade_no":%q,"transaction_id":"4200001","trade_state":"SUCCESS","amount":{"total":1900},"success_time":"2026-10-13T10:00:00+08:00"}`, id))
		case r.URL.Path == "/v3/refund/domestic/refunds":
			body = []byte(`{"refund_id":"5030001","out_refund_no":"rf_1","status":"PROCESSING"}`)
		default:
			w.WriteHeader(404)
			return
		}
		f.sign(w.Header(), body)
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	u, _ := url.Parse(f.srv.URL)
	cl := &http.Client{Transport: rewrite{u}}
	return f, WechatConfig{AppID: "wxapp", MchID: "1900000109", SerialNo: "MERSERIAL", PrivateKeyPEM: pemPriv(mer), APIv3Key: apiV3Key,
		PubKeyID: "PUB_KEY_ID_TEST", PubKeyPEM: pemPub(&plat.PublicKey), NotifyURL: "https://x/pay/wechat/notify", RefundNotifyURL: "https://x/pay/wechat/refund-notify", HTTPClient: cl}
}

type rewrite struct{ u *url.URL }

func (r rewrite) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme, req.URL.Host = r.u.Scheme, r.u.Host
	return http.DefaultTransport.RoundTrip(req)
}

// notifyReq 构造平台回调：资源 AES-256-GCM 加密 + 平台私钥签名
func (f *fakeWx) notifyReq(t *testing.T, event string, plain any) *http.Request {
	pt, _ := json.Marshal(plain)
	blk, _ := aes.NewCipher([]byte(apiV3Key))
	gcm, _ := cipher.NewGCM(blk)
	nonce := "abcdefghijkl"
	ct := gcm.Seal(nil, []byte(nonce), pt, []byte("transaction"))
	body, _ := json.Marshal(map[string]any{"id": "EV-" + event, "create_time": time.Now().Format(time.RFC3339), "event_type": event, "resource_type": "encrypt-resource", "summary": "s",
		"resource": map[string]any{"algorithm": "AEAD_AES_256_GCM", "ciphertext": base64.StdEncoding.EncodeToString(ct), "nonce": nonce, "associated_data": "transaction", "original_type": "transaction"}})
	req := httptest.NewRequest("POST", "/pay/wechat/notify", bytes.NewReader(body))
	f.sign(req.Header, body)
	return req
}

func TestWechatJSAPIFlow(t *testing.T) {
	f, cfg := newFake(t)
	defer f.srv.Close()
	ctx := context.Background()
	w, err := NewWechat(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	pid, params, err := w.Prepay(ctx, PrepayReq{OrderID: "od_1", Description: "文澈 月卡", AmountFen: 1900, OpenID: "o-abc", ExpireAt: time.Now().Add(15 * time.Minute)})
	if err != nil || pid != "wx_prepay_1" || params["package"] != "prepay_id=wx_prepay_1" || params["signType"] != "RSA" || params["paySign"] == "" {
		t.Fatal(pid, params, err)
	}
	// 小程序调起支付签名：商户私钥对 appId\ntimeStamp\nnonceStr\npackage\n 签名
	msg := fmt.Sprintf("%s\n%s\n%s\n%s\n", params["appId"], params["timeStamp"], params["nonceStr"], params["package"])
	h := sha256.Sum256([]byte(msg))
	sig, _ := base64.StdEncoding.DecodeString(params["paySign"])
	if rsa.VerifyPKCS1v15(f.merchant, crypto.SHA256, h[:], sig) != nil {
		t.Fatal("paySign 验证失败")
	}
	txn, err := w.Query(ctx, "od_1")
	if err != nil || txn.State != "SUCCESS" || txn.TransactionID != "4200001" || txn.AmountFen != 1900 || txn.PaidAt == nil {
		t.Fatal(txn, err)
	}
	if err := w.Close(ctx, "od_2"); err != nil {
		t.Fatal(err)
	}
	rf, err := w.Refund(ctx, RefundReq{OrderID: "od_1", RefundID: "rf_1", RefundFen: 1900, TotalFen: 1900, Reason: "误购"})
	if err != nil || rf.State != "PROCESSING" || rf.ProviderRefundID != "5030001" {
		t.Fatal(rf, err)
	}
	// 支付回调：验签 + 解密
	n, err := w.ParseNotify(ctx, f.notifyReq(t, "TRANSACTION.SUCCESS", map[string]any{"out_trade_no": "od_1", "transaction_id": "4200001", "trade_state": "SUCCESS", "amount": map[string]any{"total": 1900}}))
	if err != nil || n.Kind != "pay" || n.Txn.OrderID != "od_1" || n.Txn.State != "SUCCESS" || n.EventID != "EV-TRANSACTION.SUCCESS" {
		t.Fatal(n, err)
	}
	// 退款回调
	n, err = w.ParseNotify(ctx, f.notifyReq(t, "REFUND.SUCCESS", map[string]any{"out_trade_no": "od_1", "out_refund_no": "rf_1", "refund_id": "5030001", "refund_status": "SUCCESS"}))
	if err != nil || n.Kind != "refund" || n.Refund.RefundID != "rf_1" || n.Refund.State != "SUCCESS" {
		t.Fatal(n, err)
	}
	// 篡改回调体：验签失败
	req := f.notifyReq(t, "TRANSACTION.SUCCESS", map[string]any{"out_trade_no": "od_1"})
	b, _ := io.ReadAll(req.Body)
	req.Body = io.NopCloser(bytes.NewReader(bytes.Replace(b, []byte(`"summary":"s"`), []byte(`"summary":"x"`), 1)))
	if _, err := w.ParseNotify(ctx, req); !errors.Is(err, ErrBadSignature) {
		t.Fatal("篡改的回调必须拒绝", err)
	}
	if len(f.paths) != 4 {
		t.Fatal(f.paths)
	}
}

func TestWechatConfigIncomplete(t *testing.T) {
	if _, err := NewWechat(context.Background(), WechatConfig{MchID: "1"}); err == nil {
		t.Fatal("配置不完整应报错")
	}
}
