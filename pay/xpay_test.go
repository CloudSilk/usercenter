package pay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type fakeXPay struct {
	srv       *httptest.Server
	tokenHits int32
	orders    map[string]map[string]any // order_id → order
	refunds   []map[string]any
}

func newFakeXPay(t *testing.T, appKey string) *fakeXPay {
	f := &fakeXPay{orders: map[string]map[string]any{}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		switch r.URL.Path {
		case "/cgi-bin/stable_token":
			atomic.AddInt32(&f.tokenHits, 1)
			fmt.Fprint(w, `{"access_token":"AT","expires_in":7200}`)
			return
		case "/sns/jscode2session":
			if r.URL.Query().Get("js_code") != "code1" || r.URL.Query().Get("secret") != "sec" {
				fmt.Fprint(w, `{"errcode":40029,"errmsg":"invalid code"}`)
				return
			}
			fmt.Fprint(w, `{"openid":"o-user","session_key":"9hAb/NEYUlkaMBEsmFgzig=="}`)
			return
		}
		if r.URL.Query().Get("access_token") != "AT" || r.URL.Query().Get("pay_sig") != XPayPaySig(appKey, r.URL.Path, string(body)) {
			fmt.Fprint(w, `{"errcode":268490003,"errmsg":"sig"}`)
			return
		}
		var m map[string]any
		json.Unmarshal(body, &m)
		switch r.URL.Path {
		case "/xpay/query_order":
			o := f.orders[fmt.Sprint(m["order_id"])]
			if o == nil || m["openid"] != "o-user" {
				fmt.Fprint(w, `{"errcode":268490002,"errmsg":"not found"}`)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"errcode": 0, "order": o})
		case "/xpay/refund_order":
			f.refunds = append(f.refunds, m)
			fmt.Fprintf(w, `{"errcode":0,"refund_order_id":%q,"refund_wx_order_id":"wxrf1","pay_order_id":%q}`, m["refund_order_id"], m["order_id"])
		default:
			w.WriteHeader(404)
		}
	}))
	return f
}

func testXPay(t *testing.T) (*XPay, *fakeXPay) {
	f := newFakeXPay(t, "live-key")
	x, err := NewXPay(XPayConfig{AppID: "wxapp", AppSecret: "sec", OfferID: "1450000001", AppKey: "live-key", SandboxAppKey: "sb-key", PushToken: "AAAAA",
		PushAESKey: docAESKey, BaseURL: f.srv.URL, OpenIDOf: func(context.Context, string) (string, error) { return "o-user", nil }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(f.srv.Close)
	return x, f
}

func TestXPayConfigValidation(t *testing.T) {
	if _, err := NewXPay(XPayConfig{AppID: "a"}); err == nil {
		t.Fatal("incomplete")
	}
	if _, err := NewXPay(XPayConfig{AppID: "a", OfferID: "o", PushToken: "t", AppKey: "k", AppSecret: "s", Env: 1}); err == nil {
		t.Fatal("env=1 需要沙箱 AppKey")
	}
}

func TestXPayPrepay(t *testing.T) {
	x, _ := testXPay(t)
	ctx := context.Background()
	// 通过 code2Session 换 session_key；优惠价 → activitySellingPrice
	id, p, err := x.Prepay(ctx, PrepayReq{OrderID: "WC20261013000001", AmountFen: 1500, ListFen: 1900, ProductID: "vip_month", LoginCode: "code1", OpenID: "o-user", Platform: "android"})
	if err != nil || id != "WC20261013000001" || p["mode"] != "short_series_goods" || p["provider"] != "xpay" {
		t.Fatal(id, p, err)
	}
	if p["paySig"] != XPayPaySig("live-key", "requestVirtualPayment", p["signData"]) || p["signature"] != XPayUserSignature("9hAb/NEYUlkaMBEsmFgzig==", p["signData"]) {
		t.Fatal("签名不符", p)
	}
	var sd map[string]any
	json.Unmarshal([]byte(p["signData"]), &sd)
	if sd["offerId"] != "1450000001" || sd["productId"] != "vip_month" || sd["goodsPrice"] != float64(1900) || sd["activitySellingPrice"] != float64(1500) || sd["outTradeNo"] != "WC20261013000001" || sd["attach"] != "WC20261013000001" || sd["buyQuantity"] != float64(1) {
		t.Fatal(p["signData"])
	}
	// iOS 最低 1 元
	if _, _, err := x.Prepay(ctx, PrepayReq{OrderID: "WC20261013000002", AmountFen: 99, ListFen: 99, ProductID: "p", SessionKey: "sk", Platform: "ios"}); !errors.Is(err, ErrIOSMinAmount) {
		t.Fatal(err)
	}
	if _, _, err := x.Prepay(ctx, PrepayReq{OrderID: "WC20261013000003", AmountFen: 100, ListFen: 100, ProductID: "p", SessionKey: "sk", Platform: "ios"}); err != nil {
		t.Fatal(err)
	}
	// code 属于别的用户
	if _, _, err := x.Prepay(ctx, PrepayReq{OrderID: "WC20261013000004", AmountFen: 100, ProductID: "p", LoginCode: "code1", OpenID: "o-other"}); err == nil {
		t.Fatal("openid mismatch")
	}
	for _, bad := range []PrepayReq{{OrderID: "_bad00000", AmountFen: 1, ProductID: "p", SessionKey: "s"}, {OrderID: "short", AmountFen: 1, ProductID: "p", SessionKey: "s"}, {OrderID: "WC20261013000005", AmountFen: 1, SessionKey: "s"}, {OrderID: "WC20261013000006", AmountFen: 1, ProductID: "p"}} {
		if _, _, err := x.Prepay(ctx, bad); err == nil {
			t.Fatal("应拒绝", bad)
		}
	}
}

func TestXPayQueryRefund(t *testing.T) {
	x, f := testXPay(t)
	ctx := context.Background()
	f.orders["WC1000000001"] = map[string]any{"order_id": "WC1000000001", "status": 4, "paid_fee": 1900, "left_fee": 1900, "order_type": 0, "wx_order_id": "wx_o_1", "paid_time": 1760000000}
	txn, err := x.Query(ctx, "WC1000000001")
	if err != nil || txn.State != "SUCCESS" || txn.TransactionID != "wx_o_1" || txn.AmountFen != 1900 || txn.PaidAt == nil {
		t.Fatal(txn, err)
	}
	if f.tokenHits != 1 {
		t.Fatal("token 应缓存", f.tokenHits)
	}
	rf, err := x.Refund(ctx, RefundReq{OrderID: "WC1000000001", RefundID: "RF1000000001", RefundFen: 1900, TotalFen: 1900, Reason: "误购"})
	if err != nil || rf.State != "PROCESSING" || rf.ProviderRefundID != "wxrf1" {
		t.Fatal(rf, err)
	}
	r := f.refunds[0]
	if r["left_fee"] != float64(1900) || r["refund_fee"] != float64(1900) || r["openid"] != "o-user" || r["refund_reason"] != "0" || r["req_from"] != "1" || r["env"] != float64(0) || r["biz_meta"] != "误购" {
		t.Fatal(r)
	}
	if _, err := x.Refund(ctx, RefundReq{OrderID: "WC1000000001", RefundID: "RF1000000002", RefundFen: 2000}); err == nil {
		t.Fatal("超过 left_fee")
	}
	f.orders["RF1000000001"] = map[string]any{"order_id": "RF1000000001", "status": 8, "order_type": 1, "wx_order_id": "wxrf1"}
	if q, err := x.QueryRefund(ctx, "WC1000000001", "RF1000000001"); err != nil || q.State != "SUCCESS" {
		t.Fatal(q, err)
	}
	f.orders["WC1000000002"] = map[string]any{"order_id": "WC1000000002", "status": 4, "paid_fee": 600, "left_fee": 600, "order_type": 7}
	if _, err := x.Refund(ctx, RefundReq{OrderID: "WC1000000002", RefundID: "RF1000000003", RefundFen: 600}); !errors.Is(err, ErrNotSupported) {
		t.Fatal("iOS 订单不能主动退款", err)
	}
	if err := x.Close(ctx, "WC1000000003"); err != nil {
		t.Fatal(err)
	}
}

// 构造安全模式推送请求
func pushReq(t *testing.T, x *XPay, msg string, asXML bool) *http.Request {
	enc, err := x.crypto.Encrypt([]byte(msg), nil)
	if err != nil {
		t.Fatal(err)
	}
	ts, nonce := "1714112445", "415670741"
	q := url.Values{"timestamp": {ts}, "nonce": {nonce}, "encrypt_type": {"aes"}, "msg_signature": {PushSignature("AAAAA", ts, nonce, enc)}, "signature": {PushSignature("AAAAA", ts, nonce)}}
	body := `{"ToUserName":"gh_x","Encrypt":"` + enc + `"}`
	if asXML {
		body = "<xml><ToUserName><![CDATA[gh_x]]></ToUserName><Encrypt><![CDATA[" + enc + "]]></Encrypt></xml>"
	}
	return httptest.NewRequest("POST", "/pay/xpay/push?"+q.Encode(), strings.NewReader(body))
}

const deliverXML = `<xml><ToUserName><![CDATA[gh_x]]></ToUserName><FromUserName><![CDATA[o-official]]></FromUserName><CreateTime>1714112445</CreateTime><MsgType><![CDATA[event]]></MsgType><Event><![CDATA[xpay_goods_deliver_notify]]></Event><OpenId><![CDATA[o-user]]></OpenId><OutTradeNo><![CDATA[WC1000000001]]></OutTradeNo><Env>0</Env><WeChatPayInfo><MchOrderNo><![CDATA[wx_o_1]]></MchOrderNo><TransactionId><![CDATA[4200001]]></TransactionId><PaidTime>1714112440</PaidTime></WeChatPayInfo><GoodsInfo><ProductId><![CDATA[vip_month]]></ProductId><Quantity>1</Quantity><OrigPrice>1900</OrigPrice><ActualPrice>1500</ActualPrice><Attach><![CDATA[WC1000000001]]></Attach></GoodsInfo></xml>`

func TestXPayPushParse(t *testing.T) {
	x, _ := testXPay(t)
	ctx := context.Background()
	n, err := x.ParseNotify(ctx, pushReq(t, x, deliverXML, true))
	if err != nil || n.Kind != "pay" || n.Txn.OrderID != "WC1000000001" || n.Txn.TransactionID != "wx_o_1" || n.Txn.AmountFen != 1500 || n.EventID != "deliver:wx_o_1" || n.OpenID != "o-user" || !n.AckXML {
		t.Fatal(n, err)
	}
	if st, ct, b := x.Ack(n, nil); st != 200 || ct != "application/xml" || !strings.Contains(string(b), "<ErrCode>0</ErrCode>") {
		t.Fatal(st, string(b))
	}
	// JSON 格式退款推送
	n, err = x.ParseNotify(ctx, pushReq(t, x, `{"Event":"xpay_refund_notify","OpenId":"o-user","WxRefundId":"wxrf1","MchRefundId":"RF1000000001","WxOrderId":"wx_o_1","MchOrderId":"WC1000000001","RefundFee":1500,"RetCode":0}`, false))
	if err != nil || n.Kind != "refund" || n.Refund.RefundID != "RF1000000001" || n.Refund.State != "SUCCESS" || n.Refund.OrderID != "WC1000000001" || n.AckXML {
		t.Fatal(n, err)
	}
	// 沙箱推送打到现网配置：拒绝发货（ErrCode≠0）
	n, err = x.ParseNotify(ctx, pushReq(t, x, strings.Replace(deliverXML, "<Env>0</Env>", "<Env>1</Env>", 1), true))
	if err == nil {
		t.Fatal("env mismatch")
	}
	if _, _, b := x.Ack(n, err); !strings.Contains(string(b), "<ErrCode>1</ErrCode>") {
		t.Fatal(string(b))
	}
	// 篡改签名
	req := pushReq(t, x, deliverXML, true)
	q := req.URL.Query()
	q.Set("msg_signature", "deadbeef")
	req.URL.RawQuery = q.Encode()
	n, err = x.ParseNotify(ctx, req)
	if st, _, _ := x.Ack(n, err); !errors.Is(err, ErrBadSignature) || st != 401 {
		t.Fatal(err)
	}
	// URL 校验
	v := url.Values{"signature": {"f464b24fc39322e44b38aa78f5edd27bd1441696"}, "echostr": {"4375120948345356249"}, "timestamp": {"1714036504"}, "nonce": {"1514711492"}}
	n, err = x.ParseNotify(ctx, httptest.NewRequest("GET", "/pay/xpay/push?"+v.Encode(), nil))
	if st, _, b := x.Ack(n, err); err != nil || st != 200 || string(b) != "4375120948345356249" {
		t.Fatal(string(b), err)
	}
}

func testDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestDeliveryLedgerOnce(t *testing.T) {
	l := DeliveryLedger{DB: testDB(t)}
	ctx := context.Background()
	n := 0
	tx := &Txn{OrderID: "WC1", TransactionID: "wx_o_1", AmountFen: 100}
	if ok, err := l.Once(ctx, "xpay", tx, "o", func(context.Context) error { return errors.New("boom") }); ok || err == nil {
		t.Fatal("失败应释放")
	}
	for i := 0; i < 3; i++ {
		if _, err := l.Once(ctx, "xpay", tx, "o", func(context.Context) error { n++; return nil }); err != nil {
			t.Fatal(err)
		}
	}
	if n != 1 || !l.Delivered(ctx, "xpay", "wx_o_1") {
		t.Fatal("只能发货一次", n)
	}
	if _, err := l.Once(ctx, "xpay", &Txn{}, "", func(context.Context) error { return nil }); err == nil {
		t.Fatal("缺渠道单号")
	}
}

func TestMountNotifyWithLedger(t *testing.T) {
	gin.SetMode(gin.TestMode)
	x, _ := testXPay(t)
	l := DeliveryLedger{DB: testDB(t)}
	delivered := 0
	r := gin.New()
	MountNotify(r, "/pay/xpay/push", func() (Provider, error) { return x, nil }, func(ctx context.Context, p Provider, n *Notify) error {
		if n.Kind != "pay" {
			return nil
		}
		_, err := l.Once(ctx, p.Name(), n.Txn, n.OpenID, func(context.Context) error { delivered++; return nil })
		return err
	})
	for i := 0; i < 3; i++ { // 平台重推
		w := httptest.NewRecorder()
		r.ServeHTTP(w, pushReq(t, x, deliverXML, true))
		if w.Code != 200 || !strings.Contains(w.Body.String(), "<ErrCode>0</ErrCode>") {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if delivered != 1 {
		t.Fatal(delivered)
	}
	w := httptest.NewRecorder()
	v := url.Values{"signature": {PushSignature("AAAAA", "1", "2")}, "echostr": {"hi"}, "timestamp": {"1"}, "nonce": {"2"}}
	r.ServeHTTP(w, httptest.NewRequest("GET", "/pay/xpay/push?"+v.Encode(), nil))
	if w.Body.String() != "hi" {
		t.Fatal(w.Body.String())
	}
}

func TestNewByConfig(t *testing.T) {
	ctx := context.Background()
	if p, err := New(ctx, Config{Provider: "mock"}); err != nil || p.Name() != "mock" {
		t.Fatal(p, err)
	}
	if p, err := New(ctx, Config{Provider: "virtual", XPay: XPayConfig{AppID: "a", OfferID: "o", PushToken: "t", AppKey: "k", AppSecret: "s"}}); err != nil || p.Name() != "xpay" {
		t.Fatal(p, err)
	}
	if _, err := New(ctx, Config{Provider: "jsapi"}); err == nil {
		t.Fatal("jsapi 配置不完整")
	}
	if _, err := New(ctx, Config{Provider: "alipay"}); err == nil {
		t.Fatal("unknown")
	}
	if Normalize("wechat") != "jsapi" || Normalize("xpay") != "virtual" {
		t.Fatal("normalize")
	}
}
