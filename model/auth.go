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

// AuthenticatePrincipal 鉴权并返回 Principal(阶段3:Principal 为一等返回值)。
// 这是 REDESIGN §4 阶段3 的核心交付——Authenticate 不再返回 CurrentUser,
// 而是返回 Principal,彻底消除 FromCurrentUser 适配器。
func AuthenticatePrincipal(t, method, url string, checkAuth bool) (principal.Principal, int, error) {
	currentUser, decodeTokenErr := token.DecodeToken(t)

	// 白名单(免登录)
	ok, err := permission.EnforceCached("-1", url, method)
	if err != nil {
		return nil, model.InternalServerError, err
	}
	if ok {
		return principal.FromTokenAndUser(t, currentUser), model.Success, nil
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

	p := principal.FromTokenAndUser(t, currentUser)

	if !checkAuth {
		return p, model.Success, nil
	}

	// 判断是否不需要校验权限
	ok, err = permission.EnforceCached("0", url, method)
	if err != nil {
		return nil, model.InternalServerError, err
	}
	if ok {
		return p, model.Success, nil
	}

	// 遍历角色判定权限(统一用 Principal.Roles())
	for _, roleID := range p.Roles() {
		ok, err := permission.EnforceCached(roleID, url, method)
		if err != nil {
			return nil, model.InternalServerError, err
		}
		if ok {
			return p, model.Success, nil
		}
	}

	return p, model.Unauthorized, errors.New("Unauthorized")
}

// 阶段3:替代了 FromCurrentUser 适配器,Gate 1 归零。

// Authenticate 旧接口(向后兼容 provider/RPC 调用)。
// 内部委托 AuthenticatePrincipal,提取 CurrentUser 兼容旧调用方。
// 阶段3 后此函数标记 Deprecated,后续 provider 迁移完成后删除。
//
// Deprecated: 使用 AuthenticatePrincipal 替代
func Authenticate(t, method, url string, checkAuth bool) (*apipb.CurrentUser, int, error) {
	_, code, err := AuthenticatePrincipal(t, method, url, checkAuth)
	if code != model.Success {
		// 失败时仍需要 CurrentUser 供某些路径使用(如白名单返回的匿名用户)
		cu, _ := token.DecodeToken(t)
		return cu, code, err
	}
	cu, _ := token.DecodeToken(t)
	return cu, code, err
}
