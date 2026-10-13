package pay

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// 官方文档「消息推送」中的示例值
const docAESKey = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

func TestPushURLVerifyOfficialExample(t *testing.T) {
	c, _ := NewPushCrypto("AAAAA", "", "")
	echo, err := c.VerifyURL(PushQuery{Signature: "f464b24fc39322e44b38aa78f5edd27bd1441696", Timestamp: "1714036504", Nonce: "1514711492", Echostr: "4375120948345356249"})
	if err != nil || echo != "4375120948345356249" {
		t.Fatal(echo, err)
	}
	if _, err := c.VerifyURL(PushQuery{Signature: "bad", Timestamp: "1714036504", Nonce: "1514711492"}); !errors.Is(err, ErrBadSignature) {
		t.Fatal(err)
	}
	// 明文推送签名示例
	if PushSignature("AAAAA", "1714037059", "486452656") != "899cf89e464efb63f54ddac96b0a0a235f53aa78" {
		t.Fatal("plain signature")
	}
}

func TestPushDecryptOfficialExample(t *testing.T) {
	c, err := NewPushCrypto("AAAAA", docAESKey, "wxba5fad812f8e6fb9")
	if err != nil {
		t.Fatal(err)
	}
	enc := "+qdx1OKCy+5JPCBFWw70tm0fJGb2Jmeia4FCB7kao+/Q5c/ohsOzQHi8khUOb05JCpj0JB4RvQMkUyus8TPxLKJGQqcvZqzDpVzazhZv6JsXUnnR8XGT740XgXZUXQ7vJVnAG+tE8NUd4yFyjPy7GgiaviNrlCTj+l5kdfMuFUPpRSrfMZuMcp3Fn2Pede2IuQrKEYwKSqFIZoNqJ4M8EajAsjLY2km32IIjdf8YL/P50F7mStwntrA2cPDrM1kb6mOcfBgRtWygb3VIYnSeOBrebufAlr7F9mFUPAJGj04="
	body := []byte(`{"ToUserName":"gh_97417a04a28d","Encrypt":"` + enc + `"}`)
	q := PushQuery{Signature: "6c5c811b55cc85e0e1b54100749188c20beb3f5d", Timestamp: "1714112445", Nonce: "415670741", EncryptType: "aes", MsgSignature: "046e02f8204d34f8ba5fa3b1db94908f3df2e9b3"}
	msg, isX, err := c.Open(q, body)
	want := `{"ToUserName":"gh_97417a04a28d","FromUserName":"o9AgO5Kd5ggOC-bXrbNODIiE3bGY","CreateTime":1714112445,"MsgType":"event","Event":"debug_demo","debug_str":"hello world"}`
	if err != nil || isX || string(msg) != want {
		t.Fatal(string(msg), err)
	}
	q.MsgSignature = "0000"
	if _, _, err := c.Open(q, body); !errors.Is(err, ErrBadSignature) {
		t.Fatal(err)
	}
	// appid 不符必须拒绝
	c2, _ := NewPushCrypto("AAAAA", docAESKey, "wxother")
	if _, err := c2.Decrypt(enc); err == nil {
		t.Fatal("appid mismatch")
	}
}

func TestPushEncryptOfficialExample(t *testing.T) {
	c, _ := NewPushCrypto("AAAAA", docAESKey, "wxba5fad812f8e6fb9")
	enc, err := c.Encrypt([]byte(`{"demo_resp":"good luck"}`), []byte("707722b803182950"))
	if err != nil || enc != "ELGduP2YcVatjqIS+eZbp80MNLoAUWvzzyJxgGzxZO/5sAvd070Bs6qrLARC9nVHm48Y4hyRbtzve1L32tmxSQ==" {
		t.Fatal(enc, err)
	}
	if PushSignature("AAAAA", "1713424427", "415670741", enc) != "1b9339964ed2e271e7c7b6ff2b0ef902fc94dea1" {
		t.Fatal("reply signature")
	}
}

func TestPushPlainRequiresSignature(t *testing.T) {
	c, _ := NewPushCrypto("AAAAA", "", "")
	body := []byte(`<xml><Event><![CDATA[xpay_goods_deliver_notify]]></Event><OpenId>o1</OpenId><OutTradeNo>WC1</OutTradeNo><Env>0</Env><WeChatPayInfo><MchOrderNo>wx1</MchOrderNo><TransactionId>42</TransactionId><PaidTime>1714000000</PaidTime></WeChatPayInfo><GoodsInfo><ProductId>p1</ProductId><Quantity>1</Quantity><OrigPrice>1900</OrigPrice><ActualPrice>1500</ActualPrice><Attach>a</Attach></GoodsInfo></xml>`)
	q := PushQuery{Timestamp: "1", Nonce: "2", Signature: PushSignature("AAAAA", "1", "2")}
	msg, isX, err := c.Open(q, body)
	if err != nil || !isX {
		t.Fatal(err)
	}
	e, err := ParseXPayEvent(msg, isX)
	if err != nil || e.Event != XPayEventGoodsDeliver || e.WeChatPayInfo.MchOrderNo != "wx1" || e.GoodsInfo.ActualPrice != 1500 || e.OpenID != "o1" {
		t.Fatal(e, err)
	}
	q.Signature = "x"
	if _, _, err := c.Open(q, body); !errors.Is(err, ErrBadSignature) {
		t.Fatal(err)
	}
	ct, b := PushAck(true, 0, "success")
	if ct != "application/xml" || !bytes.Equal(b, []byte("<xml><ErrCode>0</ErrCode><ErrMsg><![CDATA[success]]></ErrMsg></xml>")) {
		t.Fatal(string(b))
	}
	if _, b := PushAck(false, 0, "success"); !strings.Contains(string(b), `"ErrCode":0`) {
		t.Fatal(string(b))
	}
}
