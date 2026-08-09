// Package authn 提供 AuthenticatePrincipal —— 鉴权并返回 Principal 的单一入口。
//
// 本文件将 model/auth.go 中的 AuthenticatePrincipal 逻辑迁入 internal/authn，
// 切断 http/middleware 对 model 包的依赖，使 middleware 可直接调用 authn.AuthenticatePrincipal。
//
// 选择 internal/authn 而非 internal/principal 是因为 internal/auth/token 已 import principal,
// 将 AuthenticatePrincipal 放入 principal 会形成 principal→token→principal 循环。
//
// REDESIGN §4 阶段3：以后所有鉴权入口均通过此函数，返回 principal.Principal 而非 *apipb.CurrentUser。
package authn

import (
	"context"
	"errors"
	"time"

	"github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils/log"
	"github.com/CloudSilk/usercenter/internal/auth/token"
	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/principal"
	"github.com/CloudSilk/usercenter/internal/session"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/golang-jwt/jwt/v5"
)

// AuthenticatePrincipal 鉴权并返回 Principal(阶段3:Principal 为一等返回值)。
//
// 这是 REDESIGN §4 阶段3 的核心交付——Authenticate 不再返回 CurrentUser,
// 而是返回 Principal,彻底消除 FromCurrentUser 适配器。
func AuthenticatePrincipal(t, method, url string, checkAuth bool) (principal.Principal, *apipb.CurrentUser, int, error) {
	currentUser, decodeTokenErr := token.DecodeToken(t)

	// 白名单(免登录)
	ok, err := permission.EnforceCached("-1", url, method)
	if err != nil {
		return nil, nil, model.InternalServerError, err
	}
	if ok {
		p := principal.FromTokenAndUser(t, currentUser)
		return p, currentUser, model.Success, nil
	}

	if decodeTokenErr != nil {
		if errors.Is(decodeTokenErr, jwt.ErrTokenExpired) {
			return nil, nil, model.TokenExpired, errors.New("token is expired")
		}
		return nil, nil, model.TokenInvalid, decodeTokenErr
	}

	if ok, err := token.DefaultTokenCache.Exists("", t); err != nil {
		log.Error(context.Background(), err)
		return nil, nil, model.InternalServerError, err
	} else if !ok {
		return nil, nil, model.TokenInvalid, errors.New("token invalid")
	}
	tokenSig := token.GetTokenSignature(t)
	if tokenSig != "" {
		if session.IsRevoked(tokenSig) {
			return nil, nil, model.TokenInvalid, errors.New("session revoked")
		}
	}

	p := principal.FromTokenAndUser(t, currentUser)
	if code, err := validatePrincipalStatus(p, time.Now()); code != model.Success {
		return nil, nil, code, err
	}
	if tokenSig != "" {
		session.UpdateActivity(tokenSig)
	}

	if !checkAuth {
		return p, currentUser, model.Success, nil
	}

	// 判断是否不需要校验权限
	ok, err = permission.EnforceCached("0", url, method)
	if err != nil {
		return nil, nil, model.InternalServerError, err
	}
	if ok {
		return p, currentUser, model.Success, nil
	}

	// 遍历角色判定权限(统一用 Principal.Roles())
	for _, roleID := range p.Roles() {
		ok, err := permission.EnforceCached(roleID, url, method)
		if err != nil {
			return nil, nil, model.InternalServerError, err
		}
		if ok {
			return p, currentUser, model.Success, nil
		}
	}

	return p, currentUser, model.Unauthorized, errors.New("Unauthorized")
}
