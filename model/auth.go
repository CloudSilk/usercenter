package model

import (
	"context"
	"errors"

	"github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils/log"
	"github.com/CloudSilk/usercenter/internal/auth/token"
	"github.com/CloudSilk/usercenter/internal/permission"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/golang-jwt/jwt/v5"
)

// Authenticate 鉴权主体逻辑。权限判定(enforceCached)已迁至 internal/permission。
func Authenticate(t, method, url string, checkAuth bool) (*apipb.CurrentUser, int, error) {
	currentUser, decodeTokenErr := token.DecodeToken(t)
	// 判断是否不需要登录
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
		} else {
			return nil, model.TokenInvalid, decodeTokenErr
		}
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

	// 判断是否不需要校验权限
	ok, err = permission.EnforceCached("0", url, method)
	if err != nil {
		return nil, model.InternalServerError, err
	}
	if ok {
		return currentUser, model.Success, nil
	}

	for _, roleID := range currentUser.RoleIDs {
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
