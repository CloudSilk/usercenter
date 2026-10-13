package pay

import (
	"encoding/json"
	"testing"
)

// 官方文档「2.6 参考的 Python 脚本」中的示例值
func TestXPaySignaturesOfficialExample(t *testing.T) {
	body := `{"openid": "xxx", "user_ip": "127.0.0.1", "env": 0}`
	if got := XPayPaySig("12345", "/xpay/query_user_balance", body); got != "c37809f27c6d7fd1837ad2500a04512b66b34fd793a39a385fade56dca89a4b5" {
		t.Fatal("pay_sig", got)
	}
	if got := XPayUserSignature("9hAb/NEYUlkaMBEsmFgzig==", body); got != "089d9e8dc5d308977360c4b79ec600a93d736802802a807d634192328032f6c7" {
		t.Fatal("signature", got)
	}
}

func TestBuildXPayClientParams(t *testing.T) {
	p, err := BuildXPayClientParams("k", "sk", XPayModeGoods, XPaySignData{OfferID: "123", BuyQuantity: 1, CurrencyType: "CNY", ProductID: "p1", GoodsPrice: 10, OutTradeNo: "T12345678", Attach: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if p.PaySig != XPayPaySig("k", "requestVirtualPayment", p.SignData) || p.Signature != XPayUserSignature("sk", p.SignData) || p.Mode != "short_series_goods" {
		t.Fatal(p)
	}
	var m map[string]any
	json.Unmarshal([]byte(p.SignData), &m)
	// env=0 必须出现在 signData 中；未打折时不带 activitySellingPrice
	if m["env"] != float64(0) || m["offerId"] != "123" || m["goodsPrice"] != float64(10) || m["currencyType"] != "CNY" || m["activitySellingPrice"] != nil {
		t.Fatal(p.SignData)
	}
}
