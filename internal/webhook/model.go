package webhook

import (
	commonmodel "github.com/CloudSilk/pkg/model"
)

// Subscription 表示一条 Webhook 订阅规则。
// 一个订阅绑定一个事件类型集合、一个目标 URL、一个 HMAC-SHA256 签名密钥。
type Subscription struct {
	commonmodel.Model
	TenantID string `json:"tenantID" gorm:"index;size:36"`
	Name     string `json:"name" gorm:"size:100"`
	URL      string `json:"url" gorm:"size:500"`
	Events   string `json:"events" gorm:"size:500"`  // 逗号分隔："user.created,role.updated"
	Secret   string `json:"secret" gorm:"size:100"` // HMAC-SHA256 签名密钥
	Enable   bool   `json:"enable" gorm:"index;default:true"`
}
