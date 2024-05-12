package model

import (
	"errors"
	"fmt"

	"github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/model/token"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/golang-jwt/jwt/v4"
)

func Authenticate(t, method, url string, checkAuth bool) (*apipb.CurrentUser, int, error) {
	currentUser, decodeTokenErr := token.DecodeToken(t)
	// 判断是否不需要登录
	ok, err := enforcer.Enforce("-1", url, method)
	if err != nil {
		return nil, model.InternalServerError, err
	}
	if ok {

		return currentUser, model.Success, nil
	}

	if decodeTokenErr != nil {
		if errors.As(decodeTokenErr, &jwt.ErrTokenExpired) {
			return nil, model.TokenExpired, errors.New("token is expired")
		} else {
			return nil, model.TokenInvalid, decodeTokenErr
		}
	}

	if ok, err := token.DefaultTokenCache.Exists("", t); err != nil {
		fmt.Println(err.Error())
		return nil, model.InternalServerError, err
	} else if !ok {
		return nil, model.TokenInvalid, errors.New("token invalid")
	}

	if !checkAuth {
		return currentUser, model.Success, nil
	}

	// 判断是否不需要校验权限
	ok, err = enforcer.Enforce("0", url, method)
	if err != nil {
		return nil, model.InternalServerError, err
	}
	if ok {
		return currentUser, model.Success, nil
	}

	for _, roleID := range currentUser.RoleIDs {
		ok, err := enforcer.Enforce(roleID, url, method)
		if err != nil {
			return nil, model.InternalServerError, err
		}

		if ok {
			return currentUser, model.Success, nil
		}
	}

	return currentUser, model.Unauthorized, errors.New("Unauthorized")
}
