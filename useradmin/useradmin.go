// Package useradmin 导出给宿主应用（如 ClassPaw）的服务端用户管理能力。
// HTTP 层的用户管理接口在平台租户下收敛为超管专属；宿主应用在完成自身
// 业务授权（如班级负责人重置本班家长密码）后，经本包在服务端侧执行。
package useradmin

import "github.com/CloudSilk/usercenter/internal/user"

// ResetUserPassword 重置用户登录密码。
func ResetUserPassword(userID, password string) error {
	return user.ResetPwd(userID, password)
}

// SetUserEnable 启用/禁用用户登录。
func SetUserEnable(userID string, enable bool) error {
	return user.EnableUser(userID, enable)
}
