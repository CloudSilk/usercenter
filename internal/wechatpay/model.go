// Package wechatpay 微信支付（APIv3）域：多租户商户配置、支付订单记录，
// 以及 JSAPI 下单、订单查询、关单、支付结果回调的业务逻辑。
package wechatpay

import (
	"errors"
	"time"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/wechatconfig"
	"gorm.io/gorm"
)

// 支付订单状态机:INIT(已创建待预支付) -> CREATED(已拿到 prepay_id)
// -> PAID(回调/查单确认支付成功) / CLOSED(已关单或上游关闭)。
const (
	PayOrderInit    = "INIT"
	PayOrderCreated = "CREATED"
	PayOrderPaid    = "PAID"
	PayOrderClosed  = "CLOSED"
)

// PayConfig 多租户微信支付商户配置,关联一个微信应用(小程序)。
type PayConfig struct {
	commonmodel.Model
	TenantID        string `json:"tenantID" gorm:"size:36;index"`
	WechatConfigID  string `json:"wechatConfigID" gorm:"size:36;index;comment:关联微信应用配置ID"`
	AppID           string `json:"appID" gorm:"size:36;comment:小程序AppID,需与商户号完成绑定"`
	MchID           string `json:"mchID" gorm:"size:32;index;comment:微信支付商户号"`
	MchSerialNo     string `json:"mchSerialNo" gorm:"size:64;comment:商户API证书序列号"`
	APIV3Key        string `json:"apiV3Key" gorm:"size:64;comment:商户APIv3密钥"`
	PrivateKey      string `json:"privateKey" gorm:"type:text;comment:商户私钥PEM内容(apiclient_key.pem)"`
	NotifyURL       string `json:"notifyURL" gorm:"size:255;comment:支付结果回调完整URL(HTTPS)"`
	RefundNotifyURL string `json:"refundNotifyURL" gorm:"size:255;comment:退款结果回调完整URL(HTTPS),留空复用notifyURL"`
	// RefundApprovalRequired 开启后退款申请先进入 PENDING 待审核,审核通过才调用微信侧退款。
	RefundApprovalRequired bool   `json:"refundApprovalRequired" gorm:"comment:退款是否需要人工审核"`
	Enable                 bool   `json:"enable" gorm:"index;comment:是否启用"`
	Description            string `json:"description" gorm:"size:255"`
}

// PayOrder 支付订单记录,OutTradeNo 对微信侧全局唯一。
type PayOrder struct {
	commonmodel.Model
	TenantID       string     `json:"tenantID" gorm:"size:36;index"`
	UserID         string     `json:"userID" gorm:"size:36;index"`
	WechatConfigID string     `json:"wechatConfigID" gorm:"size:36;index"`
	AppID          string     `json:"appID" gorm:"size:36"`
	MchID          string     `json:"mchID" gorm:"size:32;index"`
	OutTradeNo     string     `json:"outTradeNo" gorm:"size:32;uniqueIndex"`
	TransactionID  string     `json:"transactionID" gorm:"size:64;index"`
	OpenID         string     `json:"openID" gorm:"size:64"`
	Description    string     `json:"description" gorm:"size:128"`
	Attach         string     `json:"attach" gorm:"size:128"`
	Amount         int64      `json:"amount" gorm:"comment:订单金额,单位:分"`
	Currency       string     `json:"currency" gorm:"size:8;default:CNY"`
	Status         string     `json:"status" gorm:"size:16;index"`
	PrepayID       string     `json:"prepayID" gorm:"size:64"`
	ExpireAt       *time.Time `json:"expireAt" gorm:"comment:订单失效时间"`
	TradeState     string     `json:"tradeState" gorm:"size:32;comment:微信侧最近一次交易状态"`
	TradeStateDesc string     `json:"tradeStateDesc" gorm:"size:128"`
	LastEvent      string     `json:"lastEvent" gorm:"size:64;comment:最近一次回调事件类型"`
	PaidAt         *time.Time `json:"paidAt"`
	ClosedAt       *time.Time `json:"closedAt"`
}

func CreatePayConfig(m *PayConfig) (string, error) {
	err := store.DB().Create(m).Error
	return m.ID, err
}

func UpdatePayConfig(m *PayConfig) error {
	return store.DB().Omit("created_at").Save(m).Error
}

func DeletePayConfig(id string) error {
	return store.DB().Delete(&PayConfig{}, "id=?", id).Error
}

func GetPayConfigByID(id string) (*PayConfig, error) {
	m := &PayConfig{}
	err := store.DB().Where("id = ?", id).First(m).Error
	return m, err
}

// GetEnabledPayConfigByWechatConfigID 取应用下唯一启用中的商户配置。
func GetEnabledPayConfigByWechatConfigID(wechatConfigID string) (*PayConfig, error) {
	m := &PayConfig{}
	err := store.DB().Where("wechat_config_id = ? AND enable = ?", wechatConfigID, true).First(m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("该微信应用未启用支付配置")
	}
	return m, err
}

// GetPayConfigByWechatConfigID 按微信应用ID取商户配置(不要求启用中),
// 供存量订单的查单/关单回溯使用。
func GetPayConfigByWechatConfigID(wechatConfigID string) (*PayConfig, error) {
	m := &PayConfig{}
	err := store.DB().Where("wechat_config_id = ?", wechatConfigID).First(m).Error
	return m, err
}

// GetEnabledPayConfigByApp 按微信应用 appName 解析商户配置,小程序支付入口使用。
func GetEnabledPayConfigByApp(app string) (*PayConfig, *wechatconfig.WechatConfig, error) {
	wc, err := wechatconfig.GetWechatConfigByAppName(app)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, errors.New("微信应用不存在: " + app)
		}
		return nil, nil, err
	}
	cfg, err := GetEnabledPayConfigByWechatConfigID(wc.ID)
	if err != nil {
		return nil, nil, err
	}
	return cfg, wc, nil
}

// PayConfigQuery 管理端分页查询条件。
type PayConfigQuery struct {
	PageIndex      int
	PageSize       int
	TenantID       string
	MchID          string
	WechatConfigID string
}

type PayConfigListResult struct {
	Records []*PayConfig
	Total   int64
	Pages   int64
}

func QueryPayConfigs(q *PayConfigQuery) (*PayConfigListResult, error) {
	db := store.DB().Model(&PayConfig{})
	if q.TenantID != "" {
		db = db.Where("tenant_id = ?", q.TenantID)
	}
	if q.MchID != "" {
		db = db.Where("mch_id LIKE ?", "%"+q.MchID+"%")
	}
	if q.WechatConfigID != "" {
		db = db.Where("wechat_config_id = ?", q.WechatConfigID)
	}
	if q.PageSize <= 0 {
		q.PageSize = 10
	}
	if q.PageIndex <= 0 {
		q.PageIndex = 1
	}
	result := &PayConfigListResult{}
	if err := db.Count(&result.Total).Error; err != nil {
		return nil, err
	}
	result.Pages = (result.Total + int64(q.PageSize) - 1) / int64(q.PageSize)
	err := db.Order("created_at DESC").Offset((q.PageIndex - 1) * q.PageSize).Limit(q.PageSize).Find(&result.Records).Error
	return result, err
}

func CreatePayOrder(m *PayOrder) (string, error) {
	err := store.DB().Create(m).Error
	return m.ID, err
}

func GetPayOrderByOutTradeNo(outTradeNo string) (*PayOrder, error) {
	m := &PayOrder{}
	err := store.DB().Where("out_trade_no = ?", outTradeNo).First(m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return m, err
}

// MarkOrderPaid 将订单置为已支付,带状态守卫保证回调幂等:已支付的订单不会被重复更新。
// 返回本次是否发生了状态变更。
func MarkOrderPaid(id, transactionID string, paidAt time.Time) (bool, error) {
	res := store.DB().Model(&PayOrder{}).
		Where("id = ? AND status <> ?", id, PayOrderPaid).
		Updates(map[string]any{
			"status":           PayOrderPaid,
			"trade_state":      "SUCCESS",
			"trade_state_desc": "支付成功",
			"transaction_id":   transactionID,
			"paid_at":          paidAt,
		})
	return res.RowsAffected > 0, res.Error
}

func MarkOrderClosed(id string, desc string) error {
	now := time.Now()
	return store.DB().Model(&PayOrder{}).
		Where("id = ? AND status <> ?", id, PayOrderPaid).
		Updates(map[string]any{
			"status":           PayOrderClosed,
			"trade_state_desc": desc,
			"closed_at":        now,
		}).Error
}

// UpdatePayOrderEvent 记录最近一次回调事件(不改变支付状态),用于退款等事件的留痕。
func UpdatePayOrderEvent(id, tradeState, tradeStateDesc, lastEvent string) error {
	return store.DB().Model(&PayOrder{}).Where("id = ?", id).
		Updates(map[string]any{
			"trade_state":      tradeState,
			"trade_state_desc": tradeStateDesc,
			"last_event":       lastEvent,
		}).Error
}

func UpdatePayOrderPrepay(id, prepayID string) error {
	return store.DB().Model(&PayOrder{}).Where("id = ?", id).
		Update("prepay_id", prepayID).Error
}

// PayOrderQuery 管理端支付订单分页查询条件。
type PayOrderQuery struct {
	PageIndex  int
	PageSize   int
	TenantID   string
	UserID     string
	MchID      string
	Status     string
	OutTradeNo string
}

type PayOrderListResult struct {
	Records []*PayOrder
	Total   int64
	Pages   int64
}

// maxExportOrders 导出单次最大行数,防止全量导出拖垮服务。
const maxExportOrders = 50000

// filterPayOrders 构建订单查询过滤条件(分页查询与导出共用)。
func filterPayOrders(q *PayOrderQuery) *gorm.DB {
	db := store.DB().Model(&PayOrder{})
	if q.TenantID != "" {
		db = db.Where("tenant_id = ?", q.TenantID)
	}
	if q.UserID != "" {
		db = db.Where("user_id = ?", q.UserID)
	}
	if q.MchID != "" {
		db = db.Where("mch_id = ?", q.MchID)
	}
	if q.Status != "" {
		db = db.Where("status = ?", q.Status)
	}
	if q.OutTradeNo != "" {
		db = db.Where("out_trade_no = ?", q.OutTradeNo)
	}
	return db
}

func QueryPayOrders(q *PayOrderQuery) (*PayOrderListResult, error) {
	db := filterPayOrders(q)
	if q.PageSize <= 0 {
		q.PageSize = 10
	}
	if q.PageIndex <= 0 {
		q.PageIndex = 1
	}
	result := &PayOrderListResult{}
	if err := db.Count(&result.Total).Error; err != nil {
		return nil, err
	}
	result.Pages = (result.Total + int64(q.PageSize) - 1) / int64(q.PageSize)
	err := db.Order("created_at DESC").Offset((q.PageIndex - 1) * q.PageSize).Limit(q.PageSize).Find(&result.Records).Error
	return result, err
}

// ListPayOrdersForExport 导出用全量查询(复用过滤条件,无分页,行数封顶)。
func ListPayOrdersForExport(q *PayOrderQuery) ([]*PayOrder, error) {
	var list []*PayOrder
	err := filterPayOrders(q).Order("created_at DESC").Limit(maxExportOrders).Find(&list).Error
	return list, err
}

// 退款单状态:PENDING/REJECTED 为本地审核流状态(未到达微信侧),
// 其余与微信侧 RefundStatus 枚举对齐。
const (
	RefundPending    = "PENDING"  // 待审核
	RefundRejected   = "REJECTED" // 审核拒绝
	RefundProcessing = "PROCESSING"
	RefundSuccess    = "SUCCESS"
	RefundClosed     = "CLOSED"
	RefundAbnormal   = "ABNORMAL"
)

// PayRefund 退款单记录,OutRefundNo 商户侧唯一。
type PayRefund struct {
	commonmodel.Model
	TenantID       string     `json:"tenantID" gorm:"size:36;index"`
	UserID         string     `json:"userID" gorm:"size:36;index;comment:原订单归属用户"`
	PayOrderID     string     `json:"payOrderID" gorm:"size:36;index"`
	OutTradeNo     string     `json:"outTradeNo" gorm:"size:32;index"`
	OutRefundNo    string     `json:"outRefundNo" gorm:"size:64;uniqueIndex"`
	RefundID       string     `json:"refundID" gorm:"size:64;index"`
	Amount         int64      `json:"amount" gorm:"comment:退款金额,单位:分"`
	Total          int64      `json:"total" gorm:"comment:原订单金额,单位:分"`
	Reason         string     `json:"reason" gorm:"size:128"`
	Status         string     `json:"status" gorm:"size:16;index"`
	SuccessTime    *time.Time `json:"successTime"`
	ApproverID     string     `json:"approverID" gorm:"size:36;comment:审核人用户ID"`
	ApproveComment string     `json:"approveComment" gorm:"size:255;comment:审核意见"`
	ApprovedAt     *time.Time `json:"approvedAt"`
}

func CreatePayRefund(m *PayRefund) (string, error) {
	err := store.DB().Create(m).Error
	return m.ID, err
}

func UpdatePayRefund(m *PayRefund) error {
	return store.DB().Omit("created_at").Save(m).Error
}

// GetPayRefundByOutRefundNo 不存在时返回 (nil, nil)。
func GetPayRefundByOutRefundNo(outRefundNo string) (*PayRefund, error) {
	m := &PayRefund{}
	err := store.DB().Where("out_refund_no = ?", outRefundNo).First(m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return m, err
}

// SumActiveRefundAmount 统计订单占用退款额度的总额(待审核+受理中+已成功),用于可退余额校验。
func SumActiveRefundAmount(payOrderID string) (int64, error) {
	var total int64
	err := store.DB().Model(&PayRefund{}).
		Where("pay_order_id = ? AND status IN ?", payOrderID,
			[]string{RefundPending, RefundProcessing, RefundSuccess}).
		Select("COALESCE(SUM(amount),0)").
		Scan(&total).Error
	return total, err
}

// MarkRefundApproved 审核落库:通过时置为微信返回状态,拒绝时置 REJECTED。
// 带状态守卫(WHERE status=PENDING)防止并发重复审核;返回是否发生变更。
func MarkRefundApproved(id, approverID, comment string, approved bool, wxStatus, refundID string, successTime time.Time) (bool, error) {
	now := time.Now()
	updates := map[string]any{
		"approver_id":     approverID,
		"approve_comment": comment,
		"approved_at":     now,
	}
	if approved {
		status := wxStatus
		if status == "" {
			status = RefundProcessing
		}
		updates["status"] = status
		updates["refund_id"] = refundID
		if status == RefundSuccess && !successTime.IsZero() {
			updates["success_time"] = successTime
		}
	} else {
		updates["status"] = RefundRejected
	}
	res := store.DB().Model(&PayRefund{}).
		Where("id = ? AND status = ?", id, RefundPending).
		Updates(updates)
	return res.RowsAffected > 0, res.Error
}

// MarkRefundStatus 幂等推进退款单状态;已终态(SUCCESS)不再变更。返回是否发生变更。
// refundID 非空时同步回写微信退款号。
func MarkRefundStatus(id, refundID, status string, successTime time.Time) (bool, error) {
	updates := map[string]any{"status": status}
	if refundID != "" {
		updates["refund_id"] = refundID
	}
	if status == RefundSuccess && !successTime.IsZero() {
		updates["success_time"] = successTime
	}
	res := store.DB().Model(&PayRefund{}).
		Where("id = ? AND status <> ?", id, RefundSuccess).
		Updates(updates)
	return res.RowsAffected > 0, res.Error
}

// PayRefundQuery 管理端退款单分页查询条件。
type PayRefundQuery struct {
	PageIndex   int
	PageSize    int
	TenantID    string
	Status      string
	OutTradeNo  string
	OutRefundNo string
}

type PayRefundListResult struct {
	Records []*PayRefund
	Total   int64
	Pages   int64
}

func QueryPayRefunds(q *PayRefundQuery) (*PayRefundListResult, error) {
	db := store.DB().Model(&PayRefund{})
	if q.TenantID != "" {
		db = db.Where("tenant_id = ?", q.TenantID)
	}
	if q.Status != "" {
		db = db.Where("status = ?", q.Status)
	}
	if q.OutTradeNo != "" {
		db = db.Where("out_trade_no = ?", q.OutTradeNo)
	}
	if q.OutRefundNo != "" {
		db = db.Where("out_refund_no = ?", q.OutRefundNo)
	}
	if q.PageSize <= 0 {
		q.PageSize = 10
	}
	if q.PageIndex <= 0 {
		q.PageIndex = 1
	}
	result := &PayRefundListResult{}
	if err := db.Count(&result.Total).Error; err != nil {
		return nil, err
	}
	result.Pages = (result.Total + int64(q.PageSize) - 1) / int64(q.PageSize)
	err := db.Order("created_at DESC").Offset((q.PageIndex - 1) * q.PageSize).Limit(q.PageSize).Find(&result.Records).Error
	return result, err
}

// ListRefundsByOutTradeNo 用户查询自己订单的退款记录。
func ListRefundsByOutTradeNo(outTradeNo string) ([]*PayRefund, error) {
	var list []*PayRefund
	err := store.DB().Where("out_trade_no = ?", outTradeNo).Order("created_at DESC").Find(&list).Error
	return list, err
}
