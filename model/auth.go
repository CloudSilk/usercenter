package model

import (
	"context"
	"errors"

	"github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils/log"
	"github.com/CloudSilk/usercenter/internal/auth/token"
	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/principal"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/golang-jwt/jwt/v5"
)

// Authenticate 鉴权主体逻辑(REDESIGN §4 阶段1:Principal 分流 + 影子双跑)。
//
// 变更点(ADR-002):
//   - Agent token(type=1)走独立分流:从 AgentClaims 提取身份,不依赖人类字段
//   - 影子双跑:旧路径(currentUser.RoleIDs)与新路径(Principal.Roles())并行,
//     差异打 warn log(不阻断)。阶段3 删旧路径后双跑自动消失。
func Authenticate(t, method, url string, checkAuth bool) (*apipb.CurrentUser, int, error) {
	currentUser, decodeTokenErr := token.DecodeToken(t)

	// 判断是否不需要登录(白名单)
	ok, err := permission.EnforceCached("-1", url, method)
	if err != nil {
		return nil, model.InternalServerError, err
	}
	if ok {
		return currentUser, model.Success, nil
	}

	if decodeTokenErr != nil {
		if errors.Is(decodeTokenErr, jwt.ErrTokenExpired) {
			return nil, model.TokenExpired, errors.New("token is expired")
		}
		return nil, model.TokenInvalid, decodeTokenErr
	}

	if ok, err := token.DefaultTokenCache.Exists("", t); err != nil {
		log.Error(context.Background(), err)
		return nil, model.InternalServerError, err
	} else if !ok {
		return nil, model.TokenInvalid, errors.New("token invalid")
	}

	if !checkAuth {
		return currentUser, model.Success, nil
	}

	// --- ADR-002 Principal 分流 ---
	// Agent token 走独立路径:从 AgentClaims 提取身份
	var roles []string
	if token.IsAgentToken(currentUser) {
		// Agent 身份:解码 AgentClaims,用其 RoleIDs
		ac, acErr := token.DecodeAgentPrincipal(t)
		if acErr != nil {
			log.Errorf(context.Background(), "agent token decode failed: %v", acErr)
			return currentUser, model.TokenInvalid, errors.New("invalid agent token")
		}
		roles = ac.RoleIDs
		// 影子检查:Agent 的 principal.Kind() 应为 Agent
		_ = principal.NewAgent(ac.AgentID, ac.OwnerUserID, ac.TenantID, ac.RoleIDs)
	} else {
		// 人类身份:用 CurrentUser.RoleIDs + 影子双跑 Principal
		roles = currentUser.RoleIDs

		// 影子双跑:Principal.Roles() 应与 currentUser.RoleIDs 一致
		p := principal.FromCurrentUser(currentUser)
		principalRoles := p.Roles()
		if !roleIDsMatch(roles, principalRoles) {
			log.Warnf(context.Background(),
				"[SHADOW] Principal role mismatch: currentUser=%v principal=%v (url=%s)",
				roles, principalRoles, url)
		}
	}

	// 判断是否不需要校验权限(登录即可访问)
	ok, err = permission.EnforceCached("0", url, method)
	if err != nil {
		return nil, model.InternalServerError, err
	}
	if ok {
		return currentUser, model.Success, nil
	}

	// 遍历角色判定权限
	for _, roleID := range roles {
		ok, err := permission.EnforceCached(roleID, url, method)
		if err != nil {
			return nil, model.InternalServerError, err
		}
		if ok {
			return currentUser, model.Success, nil
		}
	}

	return currentUser, model.Unauthorized, errors.New("Unauthorized")
}

// roleIDsMatch 比较两个角色 ID 列表是否一致(影子双跑用,顺序不敏感)
func roleIDsMatch(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	set := make(map[string]bool, len(a))
	for _, v := range a {
		set[v] = true
	}
	for _, v := range b {
		if !set[v] {
			return false
		}
	}
	return true
}
