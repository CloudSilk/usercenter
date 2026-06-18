package model

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils/log"
	"github.com/CloudSilk/usercenter/model/token"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/golang-jwt/jwt/v5"
	"github.com/patrickmn/go-cache"
)

// authResultCache 缓存 Casbin Enforce 鉴权结果，key 为 sub|obj|act。
// 用于减少高频请求下的策略遍历开销；权限规则变更时通过 invalidateAuthCache 清空。
var authResultCache = cache.New(2*time.Minute, 5*time.Minute)

// enforceCached 带内存缓存+panic 恢复的鉴权判定。
// Casbin 内置的 ParamsMatchFunc 在参数类型异常时可能 panic（类型断言），
// 此处 recover 转为 err 返回，避免单个请求导致整个进程崩溃。
func enforceCached(sub, obj, act string) (ok bool, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("casbin enforce panic (sub=%s obj=%s act=%s): %v\n%s", sub, obj, act, r, debug.Stack())
			ok = false
		}
	}()
	key := sub + "|" + obj + "|" + act
	if v, ok := authResultCache.Get(key); ok {
		return v.(bool), nil
	}
	ok, err = enforcer.Enforce(sub, obj, act)
	if err != nil {
		return false, err
	}
	authResultCache.SetDefault(key, ok)
	return ok, nil
}

// invalidateAuthCache 清空全部鉴权缓存（权限规则变更后调用）
func invalidateAuthCache() {
	authResultCache.Flush()
}

func Authenticate(t, method, url string, checkAuth bool) (*apipb.CurrentUser, int, error) {
	currentUser, decodeTokenErr := token.DecodeToken(t)
	// 判断是否不需要登录
	ok, err := enforceCached("-1", url, method)
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
	ok, err = enforceCached("0", url, method)
	if err != nil {
		return nil, model.InternalServerError, err
	}
	if ok {
		return currentUser, model.Success, nil
	}

	for _, roleID := range currentUser.RoleIDs {
		ok, err := enforceCached(roleID, url, method)
		if err != nil {
			return nil, model.InternalServerError, err
		}

		if ok {
			return currentUser, model.Success, nil
		}
	}

	return currentUser, model.Unauthorized, errors.New("Unauthorized")
}
