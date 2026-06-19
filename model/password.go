package model

import (
	"github.com/CloudSilk/usercenter/internal/auth"
)

// 密码逻辑已迁入 internal/auth(REDESIGN §4 阶段0)。
// 此处保留 PwdStrength 别名 + 函数委托,向后兼容 model 内部调用
// (user.go 的 generatePasswd/EncryptedPassword/ValidPasswdStrength)与 password_test。

// PwdStrength 密码强度等级(定义已迁至 internal/auth)
type PwdStrength = auth.PwdStrength

const (
	PwdStrengthOnliyNumber PwdStrength = auth.PwdStrengthOnliyNumber
	PwdStrengthOnliyChar   PwdStrength = auth.PwdStrengthOnliyChar
	PwdStrengthMix         PwdStrength = auth.PwdStrengthMix
	PwdStrengthAdvance     PwdStrength = auth.PwdStrengthAdvance
)

// ValidPasswdStrength 委托 internal/auth
func ValidPasswdStrength(str string) bool { return auth.ValidPasswdStrength(str) }

// EncryptedPassword 委托 internal/auth
func EncryptedPassword(password string) (string, error) { return auth.EncryptedPassword(password) }

// generatePasswd 委托 internal/auth.GeneratePasswd(model 内部小写包装,不破坏 user.go 调用)
func generatePasswd(length int, pwdStrength PwdStrength) string {
	return auth.GeneratePasswd(length, pwdStrength)
}
