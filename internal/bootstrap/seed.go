package bootstrap

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/CloudSilk/usercenter/internal/auth"
	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/tenant"
	"github.com/CloudSilk/usercenter/internal/user"
)

const (
	bootstrapAdminName  = "admin"
	bootstrapSuperAdmin = "super_admin" // 同时写入角色名，匹配 isSuperAdmin 的名称判定
)

// SeedBootstrapAdmin 首次部署播种：仅当 users 表为空时，创建平台租户、超级管理员
// 角色与初始管理员账号，使全新部署开箱即可登录。
//
// 安全约束：
//   - 幂等：users 表已有任意用户即 no-op（避免每次启动重复播种或覆盖现网数据）。
//   - pwd 为空时随机生成 16 位强密码并返回明文，调用方须立即记录/展示并提醒改密。
//   - 初始管理员 force_change_pwd=true，首次登录强制改密，避免明文初始口令长期留存。
//
// tenantID / superAdminRoleID 由调用方从配置注入（生产为 PlatformTenantID /
// SuperAdminRoleID），函数本身不依赖全局常量，便于在常量设置前的启动阶段调用。
// 返回 seeded=true 表示本次执行了播种；generatedPwd 非空表示本次新生成了随机口令
// （仅 pwd 为空且执行播种时）。已有用户时 seeded=false（no-op）。
func SeedBootstrapAdmin(tenantID, superAdminRoleID, pwd string) (seeded bool, generatedPwd string, err error) {
	var count int64
	if err = store.DB().Model(&user.User{}).Count(&count).Error; err != nil {
		return false, "", fmt.Errorf("统计用户失败: %w", err)
	}
	if count > 0 {
		return false, "", nil // 已有用户，跳过
	}

	// 平台租户
	t := &tenant.Tenant{}
	t.ID = tenantID
	t.Name = "平台"
	t.Enable = true
	t.IsMust = true
	t.Expired = time.Now().AddDate(10, 0, 0) // 显式赋值，规避 MySQL 严格模式拒绝 '0000-00-00'
	if e := store.DB().Create(t).Error; e != nil {
		return false, "", fmt.Errorf("创建平台租户失败: %w", e)
	}

	// 超级管理员角色（ID 注入，name=super_admin，双匹配 isSuperAdmin 判定）
	r := &permission.Role{}
	r.ID = superAdminRoleID
	r.Name = bootstrapSuperAdmin
	r.TenantID = tenantID
	r.IsMust = true
	r.Description = "超级管理员（首次部署自动播种）"
	if e := store.DB().Create(r).Error; e != nil {
		return false, "", fmt.Errorf("创建超级管理员角色失败: %w", e)
	}

	// 初始口令：配置优先，否则随机生成
	plain := pwd
	if plain == "" {
		plain = generateBootstrapPwd()
		generatedPwd = plain
	}
	hashed, e := auth.EncryptedPassword(plain)
	if e != nil {
		return false, "", fmt.Errorf("初始口令哈希失败: %w", e)
	}
	u := &user.User{}
	u.UserName = bootstrapAdminName
	u.Password = hashed
	u.Nickname = "管理员"
	u.Enable = true
	u.TenantID = tenantID
	u.ForceChangePwd = true
	if e := store.DB().Create(u).Error; e != nil {
		return false, "", fmt.Errorf("创建初始管理员失败: %w", e)
	}
	if e := store.DB().Create(&user.UserRole{UserID: u.ID, RoleID: superAdminRoleID}).Error; e != nil {
		return false, "", fmt.Errorf("关联管理员角色失败: %w", e)
	}
	return true, generatedPwd, nil
}

// generateBootstrapPwd 用 crypto/rand 生成 16 位强随机口令（大小写+数字+符号）。
func generateBootstrapPwd() string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789!@#$%^&*"
	b := make([]byte, 16)
	for i := range b {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			// 极端情况（无熵源）退化为时间戳派生，避免启动卡死；口令仍强制首登改密
			b[i] = charset[int(time.Now().UnixNano())%len(charset)]
			continue
		}
		b[i] = charset[idx.Int64()]
	}
	return string(b)
}
