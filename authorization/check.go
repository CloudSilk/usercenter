package authorization

import (
	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/user"
)

// Check 检查用户是否拥有对指定资源(obj/path)执行指定动作(act/method)的权限。
//
// 这是面向嵌入模块（如 ManuNexus）的运行时权限检查公开函数，
// 封装了 internal/permission.EnforceCached + 用户角色遍历逻辑。
//
// 鉴权顺序（与 HTTP 中间件一致）：
//  1. 超管(roleID="1") → Casbin matcher 硬编码放行
//  2. 登录即放行("0" 规则) → 任意已认证用户可访问
//  3. 遍历用户角色 → 任一角色匹配 Casbin 规则即放行
//
// 参数：
//   - userID: 用户ID
//   - obj: 资源标识（HTTP 路径，如 "/api/mom/v1/work-orders"）
//   - act: 动作（HTTP 方法，如 "GET"/"POST"/"PUT"/"DELETE"）
//
// 返回：true=有权限, false=无权限
func Check(userID, obj, act string) (bool, error) {
	// 1. 查用户角色
	u, err := user.GetUserById(userID)
	if err != nil {
		return false, err
	}
	roleIDs := u.GetRoleIDs()
	if len(roleIDs) == 0 {
		// 无角色用户：检查是否为登录即放行的资源
		return permission.EnforceCached("0", obj, act)
	}

	// 2. 遍历角色判定权限
	for _, roleID := range roleIDs {
		ok, err := permission.EnforceCached(roleID, obj, act)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

// CheckByRoleIDs 直接基于角色ID列表检查权限（不查用户表）。
// 适用于已从 JWT/Principal 获取角色列表的场景。
func CheckByRoleIDs(roleIDs []string, obj, act string) (bool, error) {
	// 超管通配
	for _, roleID := range roleIDs {
		if roleID == "1" {
			return true, nil
		}
	}

	// 登录即放行
	ok, err := permission.EnforceCached("0", obj, act)
	if err != nil {
		return false, err
	}
	if ok {
		return true, nil
	}

	// 遍历角色
	for _, roleID := range roleIDs {
		ok, err := permission.EnforceCached(roleID, obj, act)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}
