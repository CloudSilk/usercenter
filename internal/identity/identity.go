// Package identity 管理外部社交身份（GitHub/Google 等）与本地用户的绑定。
//
// 社交登录回调拿到 provider 侧用户标识后，按 (provider, providerUserID) 查本地绑定：
// 命中则直接签发 token；未命中则按 email 再匹配，仍无则按需创建用户并绑定。
package identity

import (
	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/internal/store"
)

// UserExternalIdentity 第三方社交身份绑定记录。
type UserExternalIdentity struct {
	commonmodel.Model
	UserID         string `json:"userID" gorm:"index;size:36;comment:本地用户ID"`
	Provider       string `json:"provider" gorm:"index;size:30;comment:github/google等"`
	ProviderUserID string `json:"providerUserID" gorm:"size:200;comment:provider侧用户唯一ID"`
	ProviderLogin  string `json:"providerLogin" gorm:"size:200;comment:provider侧登录名/邮箱"`
}

func (UserExternalIdentity) TableName() string { return "user_external_identity" }

// FindByProvider 按 (provider, providerUserID) 查绑定。
func FindByProvider(provider, providerUserID string) (*UserExternalIdentity, error) {
	var id UserExternalIdentity
	err := store.DB().Where("provider = ? AND provider_user_id = ?", provider, providerUserID).First(&id).Error
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// Bind 创建绑定（同一 provider+providerUserID 唯一）。
func Bind(userID, provider, providerUserID, providerLogin string) (*UserExternalIdentity, error) {
	existing, err := FindByProvider(provider, providerUserID)
	if err == nil && existing != nil {
		// 已存在：更新登录名/用户映射后返回
		existing.UserID = userID
		existing.ProviderLogin = providerLogin
		_ = store.DB().Model(&UserExternalIdentity{}).Where("id = ?", existing.ID).
			Updates(map[string]interface{}{"user_id": userID, "provider_login": providerLogin}).Error
		return existing, nil
	}
	rec := &UserExternalIdentity{
		UserID: userID, Provider: provider,
		ProviderUserID: providerUserID, ProviderLogin: providerLogin,
	}
	if err := store.DB().Create(rec).Error; err != nil {
		return nil, err
	}
	return rec, nil
}

// ListByUser 列出某用户绑定的全部外部身份。
func ListByUser(userID string) ([]*UserExternalIdentity, error) {
	var list []*UserExternalIdentity
	err := store.DB().Where("user_id = ?", userID).Find(&list).Error
	return list, err
}

// Unbind 解绑。
func Unbind(id, userID string) error {
	return store.DB().Where("id = ? AND user_id = ?", id, userID).Delete(&UserExternalIdentity{}).Error
}
