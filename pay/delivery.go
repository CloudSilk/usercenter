package pay

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PayDelivery 发货台账：每个渠道单号（虚拟支付为 wx_order_id）只发货一次
type PayDelivery struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	Provider    string     `json:"provider" gorm:"size:16;uniqueIndex:uk_pay_delivery"`
	ProviderTxn string     `json:"providerTxn" gorm:"size:64;uniqueIndex:uk_pay_delivery"` // 渠道单号（wx_order_id / transaction_id）
	OrderID     string     `json:"orderId" gorm:"size:64;index"`                           // 业务单号 outTradeNo
	OpenID      string     `json:"openId" gorm:"size:64"`
	AmountFen   int64      `json:"amountFen"`
	Status      string     `json:"status" gorm:"size:16"` // delivering / delivered
	CreatedAt   time.Time  `json:"createdAt"`
	DeliveredAt *time.Time `json:"deliveredAt"`
}

// TableName 表名
func (PayDelivery) TableName() string { return "pay_deliveries" }

// AutoMigrate 创建 pay 包的表（usercenter 自身迁移已包含；独立使用 pay 包时调用）
func AutoMigrate(db *gorm.DB) error { return db.AutoMigrate(&PayDelivery{}) }

// ErrDelivering 同一渠道单号正在另一请求中发货
var ErrDelivering = errors.New("pay: 订单正在发货中")

// DeliveryLedger 以 (provider, 渠道单号) 唯一约束实现“只发货一次”
type DeliveryLedger struct{ DB *gorm.DB }

// Once 首次见到该渠道单号时执行 deliver；已发货返回 (false, nil)；deliver 失败会释放占位以便推送重试或查单补发。
func (l DeliveryLedger) Once(ctx context.Context, provider string, t *Txn, openID string, deliver func(ctx context.Context) error) (bool, error) {
	if t == nil || t.TransactionID == "" {
		return false, errors.New("pay: 缺少渠道单号，无法幂等发货")
	}
	rec := PayDelivery{Provider: provider, ProviderTxn: t.TransactionID, OrderID: t.OrderID, OpenID: openID, AmountFen: t.AmountFen, Status: "delivering", CreatedAt: time.Now()}
	res := l.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&rec)
	if res.Error != nil {
		return false, res.Error
	}
	if res.RowsAffected == 0 {
		var cur PayDelivery
		l.DB.WithContext(ctx).Where("provider = ? AND provider_txn = ?", provider, t.TransactionID).First(&cur)
		if cur.Status == "delivering" {
			return false, ErrDelivering
		}
		return false, nil
	}
	if err := deliver(ctx); err != nil {
		l.DB.WithContext(ctx).Delete(&PayDelivery{}, rec.ID)
		return false, err
	}
	now := time.Now()
	l.DB.WithContext(ctx).Model(&PayDelivery{}).Where("id = ?", rec.ID).Updates(map[string]any{"status": "delivered", "delivered_at": &now})
	return true, nil
}

// Delivered 渠道单号是否已发货
func (l DeliveryLedger) Delivered(ctx context.Context, provider, providerTxn string) bool {
	var n int64
	l.DB.WithContext(ctx).Model(&PayDelivery{}).Where("provider = ? AND provider_txn = ? AND status = 'delivered'", provider, providerTxn).Count(&n)
	return n > 0
}
