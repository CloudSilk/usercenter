package pricing

import (
	"context"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils/log"
	"github.com/CloudSilk/usercenter/internal/store"
)

// ModelPrice 模型计价表(租户级或全局)
type ModelPrice struct {
	commonmodel.Model
	TenantID     string  `json:"tenantID" gorm:"index;size:36"`
	ModelName    string  `json:"modelName" gorm:"size:100;index;comment:模型别名"`
	InputPer1M   float64 `json:"inputPer1M" gorm:"comment:输入 token 每百万美元"`
	OutputPer1M  float64 `json:"outputPer1M" gorm:"comment:输出 token 每百万美元"`
	Currency     string  `json:"currency" gorm:"size:10;default:USD"`
	Enable       bool    `json:"enable" gorm:"default:true"`
}

func (ModelPrice) TableName() string { return "model_price" }

// GetPrice 查询租户级或全局计价(租户级优先)。
func GetPrice(tenantID, modelName string) (*ModelPrice, error) {
	db := store.DB()
	if db == nil {
		return nil, nil
	}
	var p ModelPrice
	// 先查租户级
	if tenantID != "" {
		if err := db.Where("enable = ? AND tenant_id = ? AND model_name = ?", true, tenantID, modelName).First(&p).Error; err == nil {
			return &p, nil
		}
	}
	// 再查全局(tenant_id 为空)
	if err := db.Where("enable = ? AND tenant_id = '' AND model_name = ?", true, modelName).First(&p).Error; err != nil {
		return nil, nil // 无计价 = 不计费
	}
	return &p, nil
}

// CalculateCost 根据计价表和 token 数计算费用(USD)。
func CalculateCost(p *ModelPrice, prompt, comp int64) float64 {
	if p == nil {
		return 0
	}
	return float64(prompt)/1e6*p.InputPer1M + float64(comp)/1e6*p.OutputPer1M
}

// CreatePrice 创建计价记录
func CreatePrice(p *ModelPrice) (string, error) {
	err := store.DB().Create(p).Error
	return p.ID, err
}

// UpdatePrice 更新计价记录
func UpdatePrice(p *ModelPrice) error {
	return store.DB().Omit("created_at").Save(p).Error
}

// DeletePrice 删除计价记录
func DeletePrice(id string) error {
	return store.DB().Delete(&ModelPrice{}, "id=?", id).Error
}

// ListPrices 列出某租户的计价(含全局)
func ListPrices(tenantID string) ([]*ModelPrice, error) {
	var list []*ModelPrice
	db := store.DB()
	if tenantID != "" {
		db = db.Where("tenant_id IN (?, '')", tenantID)
	} else {
		db = db.Where("tenant_id = ''")
	}
	if err := db.Order("model_name").Find(&list).Error; err != nil {
		log.Errorf(context.Background(), "list model prices failed: %v", err)
		return nil, err
	}
	return list, nil
}
