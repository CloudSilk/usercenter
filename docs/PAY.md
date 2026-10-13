# 可复用支付包 `pay`：配置与接入指南

`github.com/CloudSilk/usercenter/pay` 给嵌入 usercenter 的业务服务提供三种支付渠道，接口统一（`pay.Provider`）：

| 渠道 | `Config.Provider` | `Name()` | 适用 |
| --- | --- | --- | --- |
| 微信支付 APIv3 JSAPI | `jsapi`（别名 `wechat`） | `wechat` | 实物/服务类，普通商户号，`wx.requestPayment` |
| 小程序虚拟支付 | `virtual`（别名 `xpay`） | `xpay` | 虚拟商品（会员、解锁内容等），`wx.requestVirtualPayment`；企业/事业单位/个体工商户与**个人主体**小程序都适用 |
| 模拟渠道 | `mock` | `mock` | 本地开发、测试与演示，不扣款 |

> 小程序内销售虚拟商品（会员、付费内容、解锁功能等）按平台规则必须接入虚拟支付。

`pay` 包不改变 `internal/wechatpay`（`/api/wechat/pay/*`）既有接口与表结构，向后兼容。

## 1. 用法

```go
p, err := pay.New(ctx, pay.Config{Provider: "virtual", XPay: pay.XPayConfig{
    AppID: "wx…", AppSecret: "…", OfferID: "…", AppKey: "…", SandboxAppKey: "…", Env: 0,
    PushToken: "…", PushAESKey: "…",
    OpenIDOf: func(ctx context.Context, orderID string) (string, error) { /* 按业务单号查下单用户 openid */ },
}})
// 下单：返回给小程序的调起参数
_, params, err := p.Prepay(ctx, pay.PrepayReq{OrderID: "WC2026…", AmountFen: 1500, ListFen: 1900, ProductID: "vip_month",
    OpenID: openid, LoginCode: code /* 或 SessionKey */, Platform: "android"})
// 回调 / 消息推送：GET（URL 校验）+ POST
ledger := pay.DeliveryLedger{DB: db}
pay.MountNotify(r, "/pay/xpay/push", func() (pay.Provider, error) { return p, nil }, func(ctx context.Context, p pay.Provider, n *pay.Notify) error {
    if n.Kind == "pay" {
        _, err := ledger.Once(ctx, p.Name(), n.Txn, n.OpenID, func(ctx context.Context) error { return fulfil(n.Txn) })
        return err
    }
    return nil
})
```

- `Prepay` 返回的 `params` 可直接展开传入 `wx.requestPayment`（JSAPI）或 `wx.requestVirtualPayment`（虚拟支付：`signData`、`mode`、`paySig`、`signature`）。**前端必须原样传 `signData` 字符串**，不得重新序列化。
- `Query` 是推送丢失时的查单兜底（虚拟支付为 `/xpay/query_order`，建议每 5 分钟轮询未完成订单）；`Refund` 发起退款，结果以退款回调/推送或 `RefundQuerier.QueryRefund` 为准。
- 只发货一次：`DeliveryLedger.Once` 以 `(provider, 渠道单号)` 唯一约束（表 `pay_deliveries`，已含在 usercenter 迁移中；独立使用时调 `pay.AutoMigrate(db)`）。虚拟支付的渠道单号为推送中的 `WeChatPayInfo.MchOrderNo`（即 `wx_order_id`）。
- `pay.MiniAppCredentials(appName)` 可直接复用 usercenter「微信配置」中的小程序 AppID/AppSecret。

## 2. 虚拟支付实现要点（依据官方文档）

- 文档：[企业/个体户](https://developers.weixin.qq.com/miniprogram/dev/platform-capabilities/business-capabilities/virtual-payment.html)、[个人主体](https://developers.weixin.qq.com/miniprogram/dev/platform-capabilities/business-capabilities/virtual-payment/person.html)、[wx.requestVirtualPayment](https://developers.weixin.qq.com/miniprogram/dev/api/payment/wx.requestVirtualPayment.html)、[query_order](https://developers.weixin.qq.com/miniprogram/dev/server/API/VirtualPayment/api_query_order.html)、[refund_order](https://developers.weixin.qq.com/miniprogram/dev/server/API/VirtualPayment/api_refund_order.html)、[消息推送](https://developers.weixin.qq.com/miniprogram/dev/framework/server-ability/message-push.html)。
- `paySig = hex(hmac_sha256(AppKey, "requestVirtualPayment&" + signData))`；服务端 `/xpay/*` 接口 uri 为接口路径、signData 为 POST body。`signature = hex(hmac_sha256(session_key, signData))`。AppKey 随 `env` 选择（0 现网 / 1 沙箱）。单测覆盖官方示例值。
- 模式为道具直购 `short_series_goods`：`productId` 为 MP 后台已发布道具，`goodsPrice` 须等于后台道具价格；优惠后价格以 `activitySellingPrice` 传入。
- `session_key` 由前端在下单前 `wx.login` 拿到 code，服务端 `code2Session` 换取（并校验 openid 一致）。
- iOS 走 Apple 支付：需配置小程序简称并在「虚拟支付 → 基本配置」开通苹果 IAP；最低 1 元（`ErrIOSMinAmount`）；仅现网、不支持沙箱；微信 ≥ 8.0.68、iOS ≥ 15；iOS 订单无法由开发者主动退款（`Refund` 返回 `ErrNotSupported`）。
- 消息推送：GET URL 校验（`sha1(sort(token,timestamp,nonce))`，回 echostr）；安全模式用 `msg_signature` 验签并 AES-256-CBC 解密、校验 AppID；支持 XML 与 JSON；回包 `ErrCode` 格式，处理失败回 `ErrCode=1` 由平台重推（最多 15 次）。推送 `Env` 与配置不符时拒绝发货。
- 虚拟支付没有关单接口：`Close` 为空操作，业务单号单次有效。

## 3. 需要从后台获取的信息

### JSAPI（微信支付商户平台 pay.weixin.qq.com + 小程序 MP 后台）

| 项 | 来源 |
| --- | --- |
| 小程序 AppID | MP 后台 → 开发管理 → 开发设置 |
| 商户号 mchid | 商户平台 → 账户中心 → 商户信息 |
| AppID 与商户号绑定 | 商户平台 → 产品中心 → AppID 账号管理（在 MP 后台确认关联） |
| JSAPI 支付产品已开通 | 商户平台 → 产品中心 |
| 商户 API 证书序列号 + 商户 API 私钥 `apiclient_key.pem` | 商户平台 → 账户中心 → API 安全 → 申请 API 证书 |
| APIv3 密钥（32 字节） | 商户平台 → API 安全 → 设置 APIv3 密钥 |
| 微信支付公钥 + 公钥 ID（`PUB_KEY_ID_…`） | 商户平台 → API 安全 → 微信支付公钥 |
| 支付/退款回调地址（HTTPS 公网） | 自备域名 |

### 虚拟支付（小程序 MP 后台）

| 项 | 来源 |
| --- | --- |
| 小程序 AppID、AppSecret | MP 后台 → 开发管理 → 开发设置（AppSecret 用于 access_token 与 code2Session） |
| 开通虚拟支付 | MP 后台左侧「虚拟支付」（个人主体在「支付与交易 → 虚拟支付」）；企业/个体户需营业执照、提现账户与支付管理员；个人主体需身份证、类目含「工具」、已认证备案，月限额 10 万元 |
| OfferID、现网 AppKey、沙箱 AppKey | 虚拟支付 → 基本配置 → 基础配置 |
| 道具 ID 与价格（每个商品一个，已发布） | 虚拟支付 → 道具管理 |
| 消息推送 URL、Token、EncodingAESKey、数据格式 | MP 后台 → 开发管理 → 消息推送（建议安全模式）；并在虚拟支付基础配置中确认发货推送 |
| iOS：小程序简称 + 开通苹果 IAP | 账号设置 → 基本信息 → 小程序简称；虚拟支付 → 基本配置 |
| 服务器出口 IP 加入白名单（如启用） | MP 后台 → 开发管理 → 开发设置 → IP 白名单 |
